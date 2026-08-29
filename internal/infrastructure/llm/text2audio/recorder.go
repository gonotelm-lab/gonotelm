package text2audio

import (
	"context"
	"time"

	audios "github.com/gonotelm-lab/multimodal/audio"
	audioschema "github.com/gonotelm-lab/multimodal/audio/schema"

	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
)

type Text2AudioParameters struct {
	Model          string `json:"model"`
	Voice          string `json:"voice,omitempty"`
	Language       string `json:"language,omitempty"`
	Instruction    string `json:"instruction,omitempty"`
	AudioFormat    string `json:"audio_format,omitempty"`
	ResponseFormat string `json:"response_format,omitempty"`
}

func toText2AudioParameters(req *audioschema.Request, resp *audioschema.Response) *Text2AudioParameters {
	if req == nil && resp == nil {
		return nil
	}
	p := &Text2AudioParameters{}
	if req != nil {
		p.Model = req.Model
		p.Voice = req.Voice
		p.Language = string(req.Language)
		p.Instruction = req.Instruction
	}
	if resp != nil {
		p.AudioFormat = resp.AudioFormat
		p.ResponseFormat = string(resp.ResponseFormat)
	}
	return p
}

// RecordUsage mirrors multimodal/audio Usage for billing and OLAP.
// Characters and TokenUsage are independently optional per provider/model.
type RecordUsage struct {
	Characters *int64
	TokenUsage *RecordTokenUsage
}

type RecordTokenUsage struct {
	InputTokens       int64
	CachedInputTokens int64
	OutputTokens      int64
	TotalTokens       int64
}

func toRecordUsage(u *audioschema.Usage) *RecordUsage {
	if u == nil {
		return nil
	}
	ru := &RecordUsage{}
	if u.Characters != nil {
		c := *u.Characters
		ru.Characters = &c
	}
	if u.TokenUsage != nil {
		ru.TokenUsage = &RecordTokenUsage{
			InputTokens:       u.TokenUsage.InputTokens,
			CachedInputTokens: u.TokenUsage.CachedInputTokens,
			OutputTokens:      u.TokenUsage.OutputTokens,
			TotalTokens:       u.TokenUsage.TotalTokens,
		}
	}
	if ru.Characters == nil && ru.TokenUsage == nil {
		return nil
	}
	return ru
}

type Record struct {
	Provider   Text2AudioProvider
	Scene      pkgcontext.SceneType
	Text       string
	Parameters *Text2AudioParameters
	Usage      *RecordUsage

	StartTime time.Time
	EndTime   time.Time

	Metadatas map[string]any
	Error     error
}

// Recorder records text2audio calls.
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
	input *audios.CallbackInput,
	output *audios.CallbackOutput,
	endTime time.Time,
) *Record {
	r := buildRecord(ctx, endTime)
	var req *audioschema.Request
	var resp *audioschema.Response
	if input != nil {
		req = input.Request
		if req != nil {
			r.Text = req.Text
		}
	}
	if output != nil {
		resp = output.Response
		if resp != nil {
			for k, v := range resp.Extras {
				r.Metadatas[k] = v
			}
			r.Usage = toRecordUsage(resp.Usage)
		}
	}
	r.Parameters = toText2AudioParameters(req, resp)
	return r
}

func buildErrorRecord(ctx context.Context, err error, endTime time.Time) *Record {
	r := buildRecord(ctx, endTime)
	r.Error = err
	if input := getOnStartInput(ctx); input != nil && input.Request != nil {
		r.Text = input.Request.Text
		r.Parameters = toText2AudioParameters(input.Request, nil)
	}
	return r
}
