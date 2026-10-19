package handler

import (
	"context"
	"log/slog"

	"github.com/gonotelm-lab/gonotelm/internal/core/adapter"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
)

// 将storeKey转成public url
type URLExchanger interface {
	Exchange(ctx context.Context, storeKey valobj.StoreKey) string
}

func NewURLExchanger(objStore adapter.ObjectPublicURLer) URLExchanger {
	return &urlExchangerImpl{o: objStore}
}

type urlExchangerImpl struct {
	o adapter.ObjectPublicURLer
}

func (u *urlExchangerImpl) Exchange(ctx context.Context, key valobj.StoreKey) string {
	url, err := u.o.PublicURL(ctx, key)
	if err != nil {
		slog.WarnContext(ctx, "url exchanger can not exchange public url", slog.Any("err", err))
	}

	return url
}
