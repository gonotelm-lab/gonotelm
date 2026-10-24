package storekey

import (
	"context"
	"log/slog"

	"github.com/gonotelm-lab/gonotelm/internal/core/adapter"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
)

// Resolver 将 StoreKey 解析为可访问 URL，供 application 层拼装响应使用。
type Resolver struct {
	store adapter.ObjectPublicURLer
}

func New(store adapter.ObjectPublicURLer) *Resolver {
	return &Resolver{store: store}
}

// ResolvePublic 解析公有读 URL；无效 key 或解析失败返回空串。
func (r *Resolver) ResolvePublic(ctx context.Context, key valobj.StoreKey) string {
	if r == nil || r.store == nil || !key.Valid() {
		return ""
	}

	url, err := r.store.PublicURL(ctx, key)
	if err != nil {
		slog.WarnContext(ctx, "resolve public url failed",
			slog.String("store_key", key.String()),
			slog.Any("err", err),
		)
		return ""
	}

	return url
}
