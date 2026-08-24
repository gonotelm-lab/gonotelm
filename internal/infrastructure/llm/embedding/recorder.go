package embedding

import (
	"context"
	"time"

	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"

	"github.com/cloudwego/eino/components/embedding"
)

type RecordTokenUsage struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
}

func toRecordTokenUsage(u *embedding.TokenUsage) *RecordTokenUsage {
	if u == nil {
		return nil
	}
	return &RecordTokenUsage{
		PromptTokens:     u.PromptTokens,
		CompletionTokens: u.CompletionTokens,
		TotalTokens:      u.TotalTokens,
	}
}

type EmbeddingParameters struct {
	Model          string `json:"model"`
	EncodingFormat string `json:"encoding_format,omitempty"`
}

func toEmbeddingParameters(c *embedding.Config) *EmbeddingParameters {
	if c == nil {
		return nil
	}

	return &EmbeddingParameters{
		Model:          c.Model,
		EncodingFormat: c.EncodingFormat,
	}
}

type RecordOutput struct {
	Count      int `json:"count"`
	Dimensions int `json:"dimensions,omitempty"`
}

func toRecordOutput(embeddings [][]float64) *RecordOutput {
	if len(embeddings) == 0 {
		return &RecordOutput{Count: 0}
	}

	dims := 0
	if len(embeddings[0]) > 0 {
		dims = len(embeddings[0])
	}

	return &RecordOutput{
		Count:      len(embeddings),
		Dimensions: dims,
	}
}

type Record struct {
	Provider   EmbeddingType
	Scene      pkgcontext.SceneType
	Input      []string
	Output     *RecordOutput
	Parameters *EmbeddingParameters
	Usage      *RecordTokenUsage

	StartTime time.Time
	EndTime   time.Time

	Metadatas map[string]any
	Error     error
}

// Recorder records embedding calls, such as token usage and billing stats.
type Recorder interface {
	Record(ctx context.Context, rec *Record) error
}

func buildRecord(ctx context.Context, endTime time.Time) *Record {
	return &Record{
		Provider:  getProvider(ctx),
		Scene:     pkgcontext.GetSceneType(ctx),
		StartTime: getStartTime(ctx),
		EndTime:   endTime,
		Metadatas: make(map[string]any),
	}
}

func buildEndRecord(
	ctx context.Context,
	input *embedding.CallbackInput,
	output *embedding.CallbackOutput,
	endTime time.Time,
) *Record {
	r := buildRecord(ctx, endTime)
	if input != nil {
		r.Input = input.Texts
		r.Parameters = toEmbeddingParameters(input.Config)
	}
	if output != nil {
		r.Output = toRecordOutput(output.Embeddings)
		r.Usage = toRecordTokenUsage(output.TokenUsage)
		if r.Parameters == nil {
			r.Parameters = toEmbeddingParameters(output.Config)
		}
	}

	return r
}

func buildErrorRecord(ctx context.Context, err error, endTime time.Time) *Record {
	r := buildRecord(ctx, endTime)
	r.Error = err

	return r
}
