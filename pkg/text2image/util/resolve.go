package util

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"

	"github.com/gonotelm-lab/gonotelm/pkg/text2image/schema"
)

type resolveOption struct {
	ctx        context.Context
	httpClient *http.Client
}

type ResolveResponseOption func(*resolveOption)

func WithResolveContext(ctx context.Context) ResolveResponseOption {
	return func(o *resolveOption) {
		if ctx != nil {
			o.ctx = ctx
		}
	}
}

func WithResolveHttpClient(client *http.Client) ResolveResponseOption {
	return func(o *resolveOption) {
		if client != nil {
			o.httpClient = client
		}
	}
}

// 解析/下载返回图片
//
// 由于可能存在网络下载 因此使用方必须显式关闭放回的Reader
func ResolveResponse(r *schema.Response, opts ...ResolveResponseOption) (io.ReadCloser, error) {
	if r == nil {
		return nil, fmt.Errorf("empty text2image response")
	}

	switch r.ResponseFormat {
	case schema.ResponseFormatBase64:
		data, err := base64.StdEncoding.DecodeString(r.ImageBase64)
		if err != nil {
			return nil, fmt.Errorf("decode image base64 failed: %w", err)
		}
		return io.NopCloser(bytes.NewReader(data)), nil
	case schema.ResponseFormatURL:
		opt := &resolveOption{
			ctx:        context.Background(),
			httpClient: http.DefaultClient,
		}
		for _, o := range opts {
			o(opt)
		}

		// download image
		req, err := http.NewRequestWithContext(opt.ctx, http.MethodGet, r.ImageURL, nil)
		if err != nil {
			return nil, fmt.Errorf("build image download request failed: %w", err)
		}
		httpResp, err := opt.httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("download image failed: %w", err)
		}

		if httpResp.StatusCode != http.StatusOK {
			io.Copy(io.Discard, httpResp.Body)
			httpResp.Body.Close()
			return nil, fmt.Errorf("download image failed: status=%d", httpResp.StatusCode)
		}

		return httpResp.Body, nil
	}

	return nil, fmt.Errorf("text2image response format not supported: %s", r.ResponseFormat)
}
