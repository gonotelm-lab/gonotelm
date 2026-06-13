package schema

import (
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
	"github.com/mitchellh/mapstructure"
	spfcast "github.com/spf13/cast"
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
	FieldChunkPos      = "chunk_pos"
	FieldMeta          = "$meta"
)

var OutputFields = []string{
	FieldID,
	FieldNotebookID,
	FieldSourceID,
	FieldContent,
	FieldOwner,
	FieldChunkPos,
	FieldMeta,
}

// 放在向量数据库中的source信息定义
type SourceDoc struct {
	Id         string    `mapstructure:"id"`
	NotebookId string    `mapstructure:"notebook_id"`
	SourceId   string    `mapstructure:"source_id"`
	Content    string    `mapstructure:"content"`
	Owner      string    `mapstructure:"owner"`
	Embedding  []float32 `mapstructure:"embedding"`
	ChunkPos   int32     `mapstructure:"chunk_pos"`

	// Meta 用于写入动态字段，只允许可 JSON 序列化的值：
	// string/bool/number、[]any、map[string]any（可嵌套前述类型）。
	Meta map[string]any `mapstructure:"-"`

	// 搜索返回的分数
	Score float32 `mapstructure:"-"`
}

func (s *SourceDoc) GetId() string {
	if s == nil {
		return ""
	}
	return s.Id
}

func (s *SourceDoc) GetNotebookId() string {
	if s == nil {
		return ""
	}
	return s.NotebookId
}

func (s *SourceDoc) GetSourceId() string {
	if s == nil {
		return ""
	}
	return s.SourceId
}

func (s *SourceDoc) GetContent() string {
	if s == nil {
		return ""
	}
	return s.Content
}

func (s *SourceDoc) GetEmbedding() []float32 {
	if s == nil {
		return nil
	}
	return s.Embedding
}

func (s *SourceDoc) GetChunkPos() int32 {
	if s == nil {
		return -1
	}

	return s.ChunkPos
}

func (s *SourceDoc) GetScore() float32 {
	if s == nil {
		return 0
	}

	return s.Score
}

func (s *SourceDoc) PutMeta(key string, value any) {
	if s.Meta == nil {
		s.Meta = make(map[string]any)
	}
	s.Meta[key] = value
}

func (s *SourceDoc) GetMeta(key string) (any, bool) {
	if s.Meta == nil {
		return nil, false
	}
	value, ok := s.Meta[key]

	return value, ok
}

func (s *SourceDoc) GetStringMeta(key string) (string, bool) {
	raw, ok := s.GetMeta(key)
	if !ok {
		return "", false
	}
	value, err := spfcast.ToStringE(raw)
	if err != nil {
		return "", false
	}
	return value, true
}

func (s *SourceDoc) GetInt64Meta(key string) (int64, bool) {
	raw, ok := s.GetMeta(key)
	if !ok {
		return 0, false
	}
	value, err := spfcast.ToInt64E(raw)
	if err != nil {
		return 0, false
	}
	return value, true
}

func (s *SourceDoc) GetFloat64Meta(key string) (float64, bool) {
	raw, ok := s.GetMeta(key)
	if !ok {
		return 0, false
	}
	value, err := spfcast.ToFloat64E(raw)
	if err != nil {
		return 0, false
	}
	return value, true
}

func (s *SourceDoc) GetBoolMeta(key string) (bool, bool) {
	raw, ok := s.GetMeta(key)
	if !ok {
		return false, false
	}
	value, err := spfcast.ToBoolE(raw)
	if err != nil {
		return false, false
	}
	return value, true
}

// GetMetaBool reads a bool meta value safely.
func (s *SourceDoc) GetMetaBool(key string) (bool, bool) {
	return s.GetBoolMeta(key)
}

// GetMetaInt reads a numeric meta value as int safely.
func (s *SourceDoc) GetMetaInt(key string) (int, bool) {
	raw, ok := s.GetMeta(key)
	if !ok {
		return 0, false
	}
	val, err := spfcast.ToIntE(raw)
	if err != nil {
		return 0, false
	}
	return val, true
}

// GetMetaIntSlice reads a numeric slice meta value as []int safely.
// The returned bool indicates whether key exists.
func (s *SourceDoc) GetMetaIntSlice(key string) ([]int, bool, error) {
	raw, ok := s.GetMeta(key)
	if !ok {
		return nil, false, nil
	}
	val, err := spfcast.ToIntSliceE(raw)
	if err != nil {
		return nil, true, err
	}
	return val, true, nil
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

type SourceDocGetParams struct {
	NotebookId string
	SourceId   string
	DocId      string
}

type SourceDocBatchGetParams struct {
	// required
	NotebookId string
	// omit if empty
	SourceId   string
	DocIds     []string
}

type SourceDocQueryParams struct {
	// Target notebook id
	NotebookId string

	// Target source ids
	SourceIds []string

	// Target queried text
	Target string

	// Target embedding of queried text
	Embedding []float32

	// top-k returned docs
	Limit int
}

type SourceDocListParams struct {
	NotebookId string
	SourceId   string
	BatchSize  int
}

type SourceDocListByChunkPosParams struct {
	NotebookId string
	SourceId   string
	ChunkPoses []int32
	BatchSize  int
}
