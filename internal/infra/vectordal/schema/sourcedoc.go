package schema

import (
	"github.com/mitchellh/mapstructure"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
)

type Id = uuid.UUID

const (
	FieldID            = "id"
	FieldNotebookID    = "notebook_id"
	FieldSourceID      = "source_id"
	FieldContent       = "content"
	FieldSparseContent = "sparse_content"
	FieldOwner         = "owner"
	FieldEmbedding     = "embedding"
)

// 放在向量数据库中的source信息定义
type SourceDoc struct {
	Id         string    `mapstructure:"id"`
	NotebookId string    `mapstructure:"notebook_id"`
	SourceId   string    `mapstructure:"source_id"`
	Content    string    `mapstructure:"content"`
	Owner      string    `mapstructure:"owner"`
	Embedding  []float32 `mapstructure:"embedding"`
}

func (s *SourceDoc) AsMap() map[string]any {
	out := make(map[string]any)
	mapstructure.Decode(s, &out)
	return out
}

type SourceDocBatchDeleteParams struct {
	NotebookId string
	SourceId   []string
}

type SourceDocQueryParams struct {
	// Target notebook id
	NotebookId string

	// Target source ids
	SourceId []string

	// Target queried text
	Target string

	// Target embedding of queried text
	Embedding []float32

	// top-k returned docs
	Limit int
}
