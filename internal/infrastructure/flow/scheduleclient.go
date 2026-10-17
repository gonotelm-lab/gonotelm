package flow

import (
	"context"
	"net/http"
	"time"

	flowschedule "github.com/gonotelm-lab/flow/client/schedule"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

// ScheduleClient 负责一次性回调的注册与回调签名校验。
type ScheduleClient interface {
	RegisterOneShot(
		ctx context.Context,
		fireAt time.Time,
		callbackURL string,
		payload []byte,
		idempotencyKey string,
		name string,
	) (scheduleId string, err error)
	Verify(req *http.Request) error
	Close() error
}

type ScheduleClientImpl struct {
	client *flowschedule.Client
}

var _ ScheduleClient = &ScheduleClientImpl{}

func NewScheduleClient(addr, namespace, apiKey string) (*ScheduleClientImpl, error) {
	c, err := flowschedule.New(addr, flowschedule.Config{Namespace: namespace, APIKey: apiKey})
	if err != nil {
		return nil, errors.Wrap(err, "create flow schedule client failed")
	}

	return &ScheduleClientImpl{client: c}, nil
}

func (s *ScheduleClientImpl) RegisterOneShot(
	ctx context.Context,
	fireAt time.Time,
	callbackURL string,
	payload []byte,
	idempotencyKey string,
	name string,
) (string, error) {
	spec := flowschedule.OneShot(fireAt, callbackURL, payload,
		flowschedule.WithIdempotencyKey(idempotencyKey),
		flowschedule.WithName(name),
		flowschedule.WithContentType("application/json"),
	)

	sch, err := s.client.Register(ctx, spec)
	if err != nil {
		return "", errors.Wrap(err, "register schedule failed")
	}

	return sch.ID, nil
}

func (s *ScheduleClientImpl) Close() error { return s.client.Close() }

func (s *ScheduleClientImpl) Verify(req *http.Request) error {
	return s.client.VerifyRequest(req)
}
