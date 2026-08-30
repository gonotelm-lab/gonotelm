package convertdoc

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gonotelm-lab/gonotelm/internal/domain/source/entity"
	sourceerr "github.com/gonotelm-lab/gonotelm/internal/domain/source/errors"
	"github.com/gonotelm-lab/gonotelm/internal/domain/source/service/index/convertdoc/transformer"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/httpclient"

	"github.com/JohannesKaufmann/html-to-markdown/v2"
	"github.com/JohannesKaufmann/html-to-markdown/v2/converter"
	"github.com/cloudwego/eino/components/document/parser"
)

const (
	userAgent    = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/145.0.0.0 Safari/537.36"
	acceptHeader = "text/markdown;q=1.0, text/x-markdown;q=1.0, text/plain;q=1.0, text/html;q=1.0, application/xhtml+xml;q=0.9"

	maxHeaderBytes        = 1024 * 1024     // 1MB
	maxFetchContentLength = 5 * 1024 * 1024 // 5MB
)

var _ Handler = (*UrlHandler)(nil)

type UrlHandler struct {
	httpClient  *http.Client
	baseHandler *baseHandler
}

func NewUrlHandler(c HandlerConfig) *UrlHandler {
	clientBuilder := httpclient.NewBuilder(&http.Transport{
		TLSHandshakeTimeout:    time.Second * 3,
		ResponseHeaderTimeout:  time.Second * 3,
		MaxResponseHeaderBytes: maxHeaderBytes,
		ExpectContinueTimeout:  1 * time.Second,
	},
	).WithTimeout(time.Second * 30)
	client := clientBuilder.Build()

	return &UrlHandler{
		httpClient:  client,
		baseHandler: newBaseHandler("url-pipe", parser.TextParser{}, c),
	}
}

func (h *UrlHandler) Handle(
	ctx context.Context,
	src *entity.Source,
	opts ...HandleOption,
) (*HandleResult, error) {
	urlContent, err := src.GetUrlContent()
	if err != nil {
		return nil, errors.Wrap(err, "get url content failed")
	}

	targetUrl, err := url.Parse(urlContent.Url)
	if err != nil {
		return nil, errors.Wrapf(
			sourceerr.ErrSourceInvalidURL,
			"parse url failed, url=%s, err=%v",
			urlContent.Url,
			err,
		)
	}

	if targetUrl.Scheme != "http" && targetUrl.Scheme != "https" {
		return nil, sourceerr.ErrSourceInvalidURL.Msgf("invalid url scheme, url=%s", urlContent.Url)
	}

	content, err := h.defaultUrlFetcher(ctx, targetUrl)
	if err != nil {
		return nil, errors.Wrapf(err, "fetch url content failed, url=%s", urlContent.Url)
	}

	docs, converted, err := h.baseHandler.doHandle(
		ctx,
		src,
		bytes.NewReader(content),
		append([]HandleOption{}, opts...),
		nil,
		transformer.WithChunkSplitMethodByMime(entity.MimeTypeMarkdown),
	)
	if err != nil {
		return nil, errors.Wrapf(err, "convert url content failed, url=%s", urlContent.Url)
	}

	return &HandleResult{
		Docs:              docs,
		ParsedContent:     converted,
		ParsedContentType: entity.MimeTypeMarkdown,
	}, nil
}

// TODO 区分targetUrl是什么来源 如果是支持的内置来源 进行特殊处理 如果不是就执行普通的webfetch
func (h *UrlHandler) defaultUrlFetcher(ctx context.Context, url *url.URL) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url.String(), nil)
	if err != nil {
		return nil, errors.Wrap(err, "new request failed")
	}
	req.Header.Set("User-Agent", userAgent)
	// 尽量限制接收的内容
	req.Header.Set("Accept", acceptHeader)

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "do request failed")
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusBadRequest {
		buf := make([]byte, 1024)
		_, err = io.ReadAtLeast(resp.Body, buf, 1024)
		if err != nil {
			slog.Error("read response body failed", slog.Any("error", err))
		}

		return nil, errors.ErrParams.Msgf("request failed, status=%d, body=%s", resp.StatusCode, string(buf))
	}

	contentLengthStr := resp.Header.Get("Content-Length")
	if contentLengthStr != "" {
		contentLength, err := strconv.ParseInt(contentLengthStr, 10, 64)
		if err != nil {
			return nil, errors.Wrap(err, "parse content length failed")
		}
		if contentLength > maxFetchContentLength {
			return nil, errors.Wrapf(
				sourceerr.ErrSourceContentTooLarge,
				"content length too large, contentLength=%d",
				contentLength,
			)
		}
	}

	contentType := resp.Header.Get("Content-Type")
	parts := strings.Split(contentType, ";")
	if len(parts) == 0 {
		return nil, sourceerr.ErrSourceUnsupportedContentType.Msgf(
			"invalid content type, contentType=%s", contentType,
		)
	}
	mimeType := strings.ToLower(parts[0])
	if isImageAttachment(mimeType) {
		return nil, sourceerr.ErrSourceUnsupportedContentType.Msgf(
			"image attachment not supported, contentType=%s", contentType,
		)
	}

	if !isTextMime(mimeType) {
		return nil, sourceerr.ErrSourceUnsupportedContentType.Msgf(
			"not text mime type, contentType=%s", contentType,
		)
	}

	bodyReader := io.LimitReader(resp.Body, maxFetchContentLength)
	// 直接转换 不区分html还是其它
	markdownContent, err := htmltomarkdown.ConvertReader(
		bodyReader,
		converter.WithContext(ctx),
		converter.WithDomain(req.URL.Host),
	)
	if err != nil {
		return nil, errors.Wrap(err, "convert body to markdown failed")
	}

	return markdownContent, nil
}

func isImageAttachment(mimeType string) bool {
	return strings.HasPrefix(mimeType, "image/") && mimeType != "image/svg+xml" && mimeType != "image/vnd.fastbidsheet"
}

func isTextMime(mimeType string) bool {
	if mimeType == "" {
		return true
	}

	if strings.HasPrefix(mimeType, "text/") {
		return true
	}

	if strings.HasSuffix(mimeType, "+json") || strings.HasSuffix(mimeType, "+xml") {
		return true
	}

	switch mimeType {
	case "application/json", "application/xml",
		"application/javascript", "application/x-javascript":
		return true
	}

	return false
}
