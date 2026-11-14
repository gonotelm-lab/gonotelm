package agent

import (
	"context"
	stderrors "errors"
	"io"

	einomodel "github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	pkgerrors "github.com/gonotelm-lab/gonotelm/pkg/errors"
)

type transport[S any] interface {
	call(
		ctx context.Context,
		model einomodel.ToolCallingChatModel,
		msgs []*schema.Message,
		opts []einomodel.Option,
		hooks Hooks[S],
		info RunInfo,
	) (*schema.Message, error)
}

type generateTransport[S any] struct{}

func (generateTransport[S]) call(
	ctx context.Context,
	model einomodel.ToolCallingChatModel,
	msgs []*schema.Message,
	opts []einomodel.Option,
	hooks Hooks[S],
	info RunInfo,
) (*schema.Message, error) {
	msg, err := model.Generate(ctx, msgs, opts...)
	if err != nil {
		return nil, pkgerrors.Wrapf(pkgerrors.ErrLLM, "round %d generate failed: %v", info.Round, err)
	}
	if msg == nil {
		return nil, ErrEmptyResponse
	}

	if err := emitMessageOutput(ctx, hooks, info, msg); err != nil {
		return nil, pkgerrors.WithMessagef(err, "round %d output hook failed", info.Round)
	}

	return msg, nil
}

type streamTransport[S any] struct{}

func (streamTransport[S]) call(
	ctx context.Context,
	model einomodel.ToolCallingChatModel,
	msgs []*schema.Message,
	opts []einomodel.Option,
	hooks Hooks[S],
	info RunInfo,
) (*schema.Message, error) {
	reader, err := model.Stream(ctx, msgs, opts...)
	if err != nil {
		return nil, pkgerrors.Wrapf(pkgerrors.ErrLLM, "round %d stream failed: %v", info.Round, err)
	}
	if reader == nil {
		return nil, ErrEmptyResponse
	}
	defer reader.Close()

	final, err := consumeStream(ctx, reader, hooks, info)
	if err != nil {
		return nil, err
	}
	return final, nil
}

func emitMessageOutput[S any](
	ctx context.Context,
	hooks Hooks[S],
	info RunInfo,
	msg *schema.Message,
) error {
	if msg.ReasoningContent != "" {
		if err := hooks.OnReasoningStart(ctx, &OutputContext{Info: info}); err != nil {
			return err
		}
		if err := hooks.OnReasoningDelta(ctx, &OutputContext{Info: info, Delta: msg.ReasoningContent}); err != nil {
			return err
		}
		if err := hooks.OnReasoningEnd(ctx, &OutputContext{Info: info}); err != nil {
			return err
		}
	}

	if msg.Content != "" {
		if err := hooks.OnContentStart(ctx, &OutputContext{Info: info}); err != nil {
			return err
		}
		if err := hooks.OnContentDelta(ctx, &OutputContext{Info: info, Delta: msg.Content}); err != nil {
			return err
		}
		if err := hooks.OnContentEnd(ctx, &OutputContext{Info: info}); err != nil {
			return err
		}
	}

	return nil
}

func consumeStream[S any](
	ctx context.Context,
	reader *schema.StreamReader[*schema.Message],
	hooks Hooks[S],
	info RunInfo,
) (*schema.Message, error) {
	var (
		msgs          []*schema.Message
		prevReasoning bool
		prevContent   bool
	)

	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		msg, err := reader.Recv()
		if stderrors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, pkgerrors.Wrapf(pkgerrors.ErrLLM, "round %d receive message failed: %v", info.Round, err)
		}
		if msg == nil {
			continue
		}

		curReasoning := msg.ReasoningContent != ""
		curContent := msg.Content != ""

		if prevReasoning && !curReasoning {
			if err := hooks.OnReasoningEnd(ctx, &OutputContext{Info: info}); err != nil {
				return nil, pkgerrors.WithMessagef(err, "round %d output hook failed", info.Round)
			}
		}
		if prevContent && !curContent {
			if err := hooks.OnContentEnd(ctx, &OutputContext{Info: info}); err != nil {
				return nil, pkgerrors.WithMessagef(err, "round %d output hook failed", info.Round)
			}
		}
		if !prevReasoning && curReasoning {
			if err := hooks.OnReasoningStart(ctx, &OutputContext{Info: info}); err != nil {
				return nil, pkgerrors.WithMessagef(err, "round %d output hook failed", info.Round)
			}
		}
		if !prevContent && curContent {
			if err := hooks.OnContentStart(ctx, &OutputContext{Info: info}); err != nil {
				return nil, pkgerrors.WithMessagef(err, "round %d output hook failed", info.Round)
			}
		}

		if curReasoning {
			if err := hooks.OnReasoningDelta(ctx, &OutputContext{Info: info, Delta: msg.ReasoningContent}); err != nil {
				return nil, pkgerrors.WithMessagef(err, "round %d output hook failed", info.Round)
			}
		}
		if curContent {
			if err := hooks.OnContentDelta(ctx, &OutputContext{Info: info, Delta: msg.Content}); err != nil {
				return nil, pkgerrors.WithMessagef(err, "round %d output hook failed", info.Round)
			}
		}

		prevReasoning = curReasoning
		prevContent = curContent
		msgs = append(msgs, msg)
	}

	if prevReasoning {
		if err := hooks.OnReasoningEnd(ctx, &OutputContext{Info: info}); err != nil {
			return nil, pkgerrors.WithMessagef(err, "round %d output hook failed", info.Round)
		}
	}
	if prevContent {
		if err := hooks.OnContentEnd(ctx, &OutputContext{Info: info}); err != nil {
			return nil, pkgerrors.WithMessagef(err, "round %d output hook failed", info.Round)
		}
	}

	if len(msgs) == 0 {
		return nil, ErrEmptyResponse
	}

	final, err := schema.ConcatMessages(msgs)
	if err != nil {
		return nil, pkgerrors.Wrapf(pkgerrors.ErrLLM, "round %d concat messages failed: %v", info.Round, err)
	}
	if final == nil {
		return nil, ErrEmptyResponse
	}
	if final.Content == "" && final.ReasoningContent == "" && len(final.ToolCalls) == 0 && final.ResponseMeta == nil {
		return nil, ErrEmptyResponse
	}

	return final, nil
}
