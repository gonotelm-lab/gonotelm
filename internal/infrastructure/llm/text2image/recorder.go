package text2image

import (
	"context"
	"time"

	pkgt2i "github.com/gonotelm-lab/multimodal/image"
	pkgt2ischema "github.com/gonotelm-lab/multimodal/image/schema"

	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
)

type Text2ImageParameters struct {
	Model          string `json:"model"`
	Size           string `json:"size,omitempty"`
	ResponseFormat string `json:"response_format,omitempty"`
}

func toText2ImageParameters(req *pkgt2ischema.Request) *Text2ImageParameters {
	if req == nil {
		return nil
	}
	return &Text2ImageParameters{
		Model:          req.Model,
		Size:           req.Size,
		ResponseFormat: string(req.ResponseFormat),
	}
}

type RecordUsage struct {
	OutputCount int
}

type Record struct {
	Provider   Text2ImageProvider
	Scene      pkgcontext.SceneType
	Prompt     string
	Parameters *Text2ImageParameters
	Usage      *RecordUsage

	StartTime time.Time
	EndTime   time.Time

	Metadatas map[string]any
	Error     error
}

// Recorder records text2image calls.
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
	input *pkgt2i.CallbackInput,
	output *pkgt2i.CallbackOutput,
	endTime time.Time,
) *Record {
	r := buildRecord(ctx, endTime)
	if input != nil && input.Request != nil {
		r.Prompt = input.Request.Prompt
		r.Parameters = toText2ImageParameters(input.Request)
	}
	if output != nil && output.Response != nil {
		for k, v := range output.Response.Extras {
			r.Metadatas[k] = v
		}
		r.Usage = toRecordUsage(output.Response.Extras)
	}
	return r
}

func buildErrorRecord(ctx context.Context, err error, endTime time.Time) *Record {
	r := buildRecord(ctx, endTime)
	r.Error = err
	if input := getOnStartInput(ctx); input != nil && input.Request != nil {
		r.Prompt = input.Request.Prompt
		r.Parameters = toText2ImageParameters(input.Request)
	}
	return r
}

func toRecordUsage(extras map[string]any) *RecordUsage {
	usage := &RecordUsage{OutputCount: 1}
	if extras == nil {
		return usage
	}
	raw, ok := extras["image_count"]
	if !ok {
		return usage
	}
	switch n := raw.(type) {
	case int:
		if n > 0 {
			usage.OutputCount = n
		}
	case int32:
		if n > 0 {
			usage.OutputCount = int(n)
		}
	case int64:
		if n > 0 {
			usage.OutputCount = int(n)
		}
	case float64:
		if n > 0 {
			usage.OutputCount = int(n)
		}
	}
	return usage
}
