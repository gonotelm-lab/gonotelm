package slides

import (
	"bytes"
	"context"
	"fmt"

	"github.com/gonotelm-lab/gonotelm/internal/domain/sandbox/entity"
	"github.com/gonotelm-lab/gonotelm/pkg/eino-ext/parser/pptx"
)

// 检查agent生成的pptx的产物是否和格式正确的pptx文件
func (g *Generator) checkPPTXArtifactValid(ctx context.Context, sandbox entity.Sandbox, path string) (string, error) {
	fileContent, err := sandbox.ReadFile(ctx, path)
	if err != nil {
		return "", fmt.Errorf("can not read pptx file: %s, err: %w", path, err)
	}

	docs, err := pptx.NewParser(nil).Parse(ctx, bytes.NewBuffer(fileContent))
	if err != nil {
		return "", fmt.Errorf("can not parse pptx file: %s, err: %w", path, err)
	}

	if len(docs[0].Content) == 0 {
		return "", fmt.Errorf("pptx file %s content is empty", path)
	}

	return docs[0].Content, nil
}
