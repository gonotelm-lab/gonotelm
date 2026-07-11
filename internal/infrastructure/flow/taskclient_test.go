package flow

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

type fakeClient struct {
	SubmitFn func(ctx context.Context, taskType string, payload []byte) (string, error)
	GetFn    func(ctx context.Context, id string) (*TaskInfo, error)
	CancelFn func(ctx context.Context, id string) error
}

func (f *fakeClient) Submit(ctx context.Context, t string, p []byte) (string, error) {
	return f.SubmitFn(ctx, t, p)
}
func (f *fakeClient) Get(ctx context.Context, id string) (*TaskInfo, error) { return f.GetFn(ctx, id) }
func (f *fakeClient) Cancel(ctx context.Context, id string) error           { return f.CancelFn(ctx, id) }
func (f *fakeClient) Close() error                                          { return nil }

// validates interface shape only — real client is exercised in integration test
func TestTaskClientInterface(t *testing.T) {
	var _ TaskClient = &fakeClient{}
	var _ TaskClient = (*TaskClientImpl)(nil)
	_ = context.Background()
	_ = assert.True
}
