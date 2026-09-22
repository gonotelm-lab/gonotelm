package slides

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"path"
	"strings"

	"github.com/gabriel-vasile/mimetype"

	"github.com/gonotelm-lab/gonotelm/internal/application/shared/agent/tools"
	"github.com/gonotelm-lab/gonotelm/internal/application/worker/artifact/types"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	artifactentity "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"
	sandboxent "github.com/gonotelm-lab/gonotelm/internal/domain/sandbox/entity"
	sourceentitiy "github.com/gonotelm-lab/gonotelm/internal/domain/source/entity"
	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"

	einotool "github.com/cloudwego/eino/components/tool"
	einotoolutils "github.com/cloudwego/eino/components/tool/utils"
)

const checkPPTXValidToolName = "CheckPPTX"

type checkPPTXToolInput struct {
	Filename string `json:"filename" jsonschema_description:"title=targe file path,description=The target file path"`
}

type pptxGenerator struct {
	deps *types.WorkerDeps
}

func newPPTXGenerator(deps *types.WorkerDeps) *pptxGenerator {
	return &pptxGenerator{deps: deps}
}

func (g *pptxGenerator) generate(
	ctx context.Context,
	req *types.Request,
	outlineExp *slidesOutlineExpectation,
	sandbox sandboxent.Sandbox,
	sources []OutlineSource,
) (*SlidesStorageResult, error) {
	ctx = pkgcontext.WithSceneType(ctx, pkgcontext.StudioSlidesPPTXScene)

	// use thinking in pptx generation, it will take much longer
	agent, err := newSlidesPPTXAgent(g.deps, req)
	if err != nil {
		return nil, err
	}

	checkPPTXTool, err := g.getCheckPPTXValidTool(sandbox)
	if err != nil {
		return nil, errors.WithMessage(err, "infer check pptx tool failed")
	}

	sandboxDesc := sandbox.Description()
	workspaceDir := path.Join(sandboxDesc.Key.WorkspaceDir(), types.StudioSlidesDir, req.ArtifactId.String())

	err = agent.AppendTools(map[string]einotool.InvokableTool{
		tools.BashToolName:      tools.NewBashTool(sandbox, workspaceDir),
		tools.ReadFileToolName:  tools.NewReadFileTool(sandbox),
		tools.WriteFileToolName: tools.NewWriteFileTool(sandbox),
		tools.EditFileToolName:  tools.NewEditFileTool(sandbox),
		tools.ListDirToolName:   tools.NewListDirTool(sandbox),
		checkPPTXValidToolName:  checkPPTXTool,
	})
	if err != nil {
		return nil, errors.Wrap(err, "gen pptx agent append tools failed")
	}

	if err := ensureSlidesWorkspace(ctx, sandbox, workspaceDir); err != nil {
		return nil, err
	}
	outputLocation := path.Join(workspaceDir, "slides", "output", "presentation.pptx")
	payload := artifactentity.PayloadAs[*artifactentity.SlidesPayload](req.Payload)
	msgs, err := RenderSlides(ctx,
		outlineExp.Title, outlineExp.Outline,
		sources,
		sandboxDesc.Runtime, workspaceDir,
		outputLocation,
		payload.GetVisualStyle(),
		payload.GetLanguage(),
		payload.GetTip(),
	)
	if err != nil {
		return nil, errors.Wrap(err, "gen pptx render prompts failed")
	}

	_, err = agent.React(ctx, msgs)
	if err != nil {
		return nil, errors.Wrap(err, "generate pptx output failed")
	}

	slog.InfoContext(ctx, fmt.Sprintf("generate pptx output agent usage: %+v", agent.TokenUsage()))

	pptxReader, err := sandbox.ReadFile2(ctx, outputLocation)
	if err != nil {
		return nil, err
	}
	defer func() {
		if cerr := pptxReader.Close(); cerr != nil {
			slog.WarnContext(ctx, "slides generator pptx reader close failed", slog.Any("err", cerr))
		}
	}()

	storeKey := formatSlidesStoreKey(req.NotebookId, req.ArtifactId)
	if err := types.UploadReader(ctx, g.deps.ObjectStorage, storeKey, sourceentitiy.MimeTypePPTX, pptxReader); err != nil {
		return nil, errors.Wrapf(err, "upload slides object failed, artifact_id=%s", req.ArtifactId)
	}

	return &SlidesStorageResult{
		StoreKey:    storeKey,
		ContentType: sourceentitiy.MimeTypePPTX,
	}, nil
}

func formatSlidesStoreKey(notebookId, artifactId valobj.Id) string {
	return fmt.Sprintf("artifact/%s/%s.pptx", notebookId.String(), artifactId.String())
}

func (g *pptxGenerator) getCheckPPTXValidTool(sandbox sandboxent.Sandbox) (einotool.InvokableTool, error) {
	tool, err := einotoolutils.InferTool(
		checkPPTXValidToolName,
		"Validate a PPTX file. Input is the filename of the PPTX you just generated. "+
			"Always call this on your output file before finishing the task. "+
			"Returns 'OK' if the file is a valid PPTX; otherwise it returns an error message describing what is wrong, "+
			"and you must fix the file and validate again.",
		func(ctx context.Context, input *checkPPTXToolInput) (output string, err error) {
			if err := g.checkPPTXArtifactValid(ctx, sandbox, input.Filename); err != nil {
				return "", err
			}
			return "OK", nil
		},
	)
	if err != nil {
		return nil, errors.Wrapf(errors.ErrInner, "infer check pptx tool failed, err=%v", err)
	}

	return tool, nil
}

func (g *pptxGenerator) checkPPTXArtifactValid(ctx context.Context, sandbox sandboxent.Sandbox, filePath string) error {
	fileContent, err := sandbox.ReadFile(ctx, filePath, sandboxent.WithReadMaxBytes(4096))
	if err != nil {
		return fmt.Errorf("can not read pptx file: %s, err: %w", filePath, err)
	}

	mimeType, err := mimetype.DetectReader(bytes.NewBuffer(fileContent))
	if err != nil {
		return fmt.Errorf("can not detect mime type of file: %s, err: %w", filePath, err)
	}

	splits := strings.Split(mimeType.String(), ";")
	if len(splits) < 1 {
		return fmt.Errorf("can not detect mime type of file: %s, len(splits) less than 1", filePath)
	}

	detected := strings.TrimSpace(splits[0])
	if detected != sourceentitiy.MimeTypePPTX {
		return fmt.Errorf("file %s is not valid pptx, but %s", filePath, detected)
	}

	return nil
}
