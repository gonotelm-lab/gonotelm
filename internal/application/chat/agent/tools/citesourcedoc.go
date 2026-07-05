package tools

import (
	"context"
	"fmt"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	pkstring "github.com/gonotelm-lab/gonotelm/pkg/string"

	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/cloudwego/eino/schema"
)

var citeSourceDocToolParams *schema.ParamsOneOf

const CiteSourceDocToolName = "CiteSourceDoc"

func init() {
	var err error
	citeSourceDocToolParams, err = utils.GoStruct2ParamsOneOf[CiteSourceDocToolInput]()
	if err != nil {
		panic(err)
	}
}

type CiteSourceDocTool struct {
	checker   SourceDocPermissionChecker
	citations CitationCollector
}

func NewCiteSourceDocTool(
	checker SourceDocPermissionChecker,
	citations CitationCollector,
) *CiteSourceDocTool {
	return &CiteSourceDocTool{
		checker:   checker,
		citations: citations,
	}
}

var _ tool.InvokableTool = &CiteSourceDocTool{}

type CiteSourceDocToolInput struct {
	SourceDocIds []string `json:"source_doc_ids" jsonschema:"title=source document unique identifiers,description=Ordered list of source document ids for the final answer. Index 1 in the answer maps to the first id, index 2 to the second, and so on. Each id must be a valid 32-character UUID obtained from source tools such as QuerySource."`
}

func (i *CiteSourceDocToolInput) Normalize() ([]valobj.Id, error) {
	if len(i.SourceDocIds) == 0 {
		return nil, fmt.Errorf("source_doc_ids is required")
	}

	sourceDocIDs := make([]valobj.Id, 0, len(i.SourceDocIds))
	for _, sourceDocId := range i.SourceDocIds {
		sourceDocID, err := valobj.NewIdFromString(sourceDocId)
		if err != nil {
			return nil, fmt.Errorf("source doc id is not valid uuid: %w", err)
		}

		sourceDocIDs = append(sourceDocIDs, sourceDocID)
	}

	return sourceDocIDs, nil
}

func (t *CiteSourceDocTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: CiteSourceDocToolName,
		Desc: "Record the ordered citation list for the final answer. " +
			"Call this tool exactly once per conversation turn, immediately before outputting the final answer. " +
			"Pass source document ids returned by QuerySource or other source tools. " +
			"The order of source_doc_ids determines inline citation indices: the first id is <sup>1</sup>, the second is <sup>2</sup>, and so on (1-based). " +
			"Each source document id must be a valid UUID that the user has access to. " +
			"Returns 'OK' on success.",
		ParamsOneOf: citeSourceDocToolParams,
	}, nil
}

func (t *CiteSourceDocTool) InvokableRun(
	ctx context.Context,
	args string,
	opts ...tool.Option,
) (string, error) {
	var input CiteSourceDocToolInput
	err := sonic.Unmarshal(pkstring.AsBytes(args), &input)
	if err != nil {
		return "", fmt.Errorf("args input is not valid json: %w", err)
	}

	sourceDocIds, err := input.Normalize()
	if err != nil {
		return "", err
	}

	if err := t.checker.Check(ctx, sourceDocIds); err != nil {
		return "", err
	}

	if t.citations != nil {
		t.citations.Set(sourceDocIds)
	}

	return OkToolCallResult, nil
}
