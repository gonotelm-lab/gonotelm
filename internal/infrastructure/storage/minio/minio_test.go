package minio

import (
	"context"
	stderrors "errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/storage"
	pkgerrors "github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	oteltrace "go.opentelemetry.io/otel/trace"
)

func newSpanExporter(t *testing.T) *tracetest.InMemoryExporter {
	t.Helper()
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	otel.SetTracerProvider(tp)
	t.Cleanup(func() {
		_ = tp.Shutdown(context.Background())
		otel.SetTracerProvider(sdktrace.NewTracerProvider())
	})
	return exporter
}

// newTestStorage 用真实 minio client 指向本地的假 S3 服务，生产代码零抽象。
func newTestStorage(t *testing.T, handler http.HandlerFunc) (*Storage, *tracetest.InMemoryExporter) {
	t.Helper()
	exporter := newSpanExporter(t)

	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)

	client, err := minio.New(strings.TrimPrefix(ts.URL, "http://"), &minio.Options{
		Creds:  credentials.NewStaticV4("test-key", "test-secret", ""),
		Secure: false,
		Region: "us-east-1",
	})
	if err != nil {
		t.Fatalf("create minio client: %v", err)
	}

	return &Storage{client: client, bucket: "test-bucket", region: "us-east-1", presignExpiry: time.Minute}, exporter
}

func s3ObjectHeaders(w http.ResponseWriter, size int) {
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("ETag", `"abc"`)
	w.Header().Set("Last-Modified", time.Now().UTC().Format(http.TimeFormat))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", size))
}

func s3ErrorResponse(w http.ResponseWriter, status int, code string) {
	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(status)
	fmt.Fprintf(w, `<Error><Code>%s</Code><Message>%s</Message><RequestId>x</RequestId><HostId>y</HostId></Error>`, code, code)
}

func TestStatObjectSpan(t *testing.T) {
	s, exporter := newTestStorage(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodHead {
			t.Errorf("method: got %s want HEAD", r.Method)
		}
		s3ObjectHeaders(w, 42)
		w.WriteHeader(http.StatusOK)
	})

	resp, err := s.StatObject(context.Background(), &storage.StatObjectRequest{Key: "key-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Size != 42 {
		t.Fatalf("size: got %d want 42", resp.Size)
	}

	assertStatObjectSpan(t, exporter, codes.Unset)
}

func TestStatObjectNotFoundSpanRecordsError(t *testing.T) {
	s, exporter := newTestStorage(t, func(w http.ResponseWriter, r *http.Request) {
		s3ErrorResponse(w, http.StatusNotFound, "NoSuchKey")
	})

	_, err := s.StatObject(context.Background(), &storage.StatObjectRequest{Key: "key-1"})
	if err != storage.ErrObjectNotFound {
		t.Fatalf("want ErrObjectNotFound, got %v", err)
	}

	assertStatObjectSpan(t, exporter, codes.Error)
}

func assertStatObjectSpan(t *testing.T, exporter *tracetest.InMemoryExporter, wantStatus codes.Code) {
	t.Helper()
	spans := exporter.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("want 1 span, got %d", len(spans))
	}
	sp := spans[0]
	if sp.Name != "S3.StatObject" {
		t.Fatalf("span name: got %q want %q", sp.Name, "S3.StatObject")
	}
	if sp.SpanKind != oteltrace.SpanKindClient {
		t.Fatalf("span kind: got %v want client", sp.SpanKind)
	}
	assertSpanAttr(t, sp, "aws.s3.bucket", "test-bucket")
	assertSpanAttr(t, sp, "aws.s3.key", "key-1")
	assertSpanAttr(t, sp, "rpc.system.name", "aws-api")
	assertSpanAttr(t, sp, "rpc.method", "S3/StatObject")
	assertSpanAttr(t, sp, "aws.region", "us-east-1")
	if sp.Status.Code != wantStatus {
		t.Fatalf("status: got %v want %v", sp.Status.Code, wantStatus)
	}
	if wantStatus == codes.Error && len(sp.Events) == 0 {
		t.Fatal("want recorded error event")
	}
}

func TestGetObjectSpan(t *testing.T) {
	s, exporter := newTestStorage(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodHead:
			s3ObjectHeaders(w, 5)
			w.WriteHeader(http.StatusOK)
		case http.MethodGet:
			s3ObjectHeaders(w, 5)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("hello"))
		default:
			t.Errorf("unexpected method: %s", r.Method)
		}
	})

	resp, err := s.GetObject(context.Background(), &storage.GetObjectRequest{Key: "key-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(resp.Body) != "hello" {
		t.Fatalf("body: got %q want %q", resp.Body, "hello")
	}

	sp := exporter.GetSpans()[0]
	if sp.Name != "S3.GetObject" {
		t.Fatalf("span name: got %q want %q", sp.Name, "S3.GetObject")
	}
	if sp.SpanKind != oteltrace.SpanKindClient {
		t.Fatalf("span kind: got %v want client", sp.SpanKind)
	}
	assertSpanAttr(t, sp, "aws.s3.bucket", "test-bucket")
	assertSpanAttr(t, sp, "aws.s3.key", "key-1")
	if sp.Status.Code != codes.Unset {
		t.Fatalf("status: got %v want unset", sp.Status.Code)
	}
}

func TestUploadObjectSpan(t *testing.T) {
	s, exporter := newTestStorage(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("method: got %s want PUT", r.Method)
		}
		w.Header().Set("ETag", `"def"`)
		w.WriteHeader(http.StatusOK)
	})

	if err := s.UploadObject(context.Background(), &storage.UploadObjectRequest{Key: "key-1", Body: []byte("data")}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sp := exporter.GetSpans()[0]
	if sp.Name != "S3.PutObject" {
		t.Fatalf("span name: got %q want %q", sp.Name, "S3.PutObject")
	}
	assertSpanAttr(t, sp, "aws.s3.bucket", "test-bucket")
	assertSpanAttr(t, sp, "aws.s3.key", "key-1")
	if sp.Status.Code != codes.Unset {
		t.Fatalf("status: got %v want unset", sp.Status.Code)
	}
}

func TestDeleteObjectSpan(t *testing.T) {
	s, exporter := newTestStorage(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("method: got %s want DELETE", r.Method)
		}
		w.WriteHeader(http.StatusNoContent)
	})

	if err := s.DeleteObject(context.Background(), &storage.DeleteObjectRequest{Key: "key-1"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sp := exporter.GetSpans()[0]
	if sp.Name != "S3.DeleteObject" {
		t.Fatalf("span name: got %q want %q", sp.Name, "S3.DeleteObject")
	}
	assertSpanAttr(t, sp, "aws.s3.bucket", "test-bucket")
	assertSpanAttr(t, sp, "aws.s3.key", "key-1")
	if sp.Status.Code != codes.Unset {
		t.Fatalf("status: got %v want unset", sp.Status.Code)
	}
}

func TestDeleteObjectSpanRecordsError(t *testing.T) {
	s, exporter := newTestStorage(t, func(w http.ResponseWriter, r *http.Request) {
		s3ErrorResponse(w, http.StatusInternalServerError, "InternalError")
	})

	if err := s.DeleteObject(context.Background(), &storage.DeleteObjectRequest{Key: "key-1"}); err == nil {
		t.Fatal("want error")
	}

	sp := exporter.GetSpans()[0]
	if sp.Status.Code != codes.Error {
		t.Fatalf("status: got %v want error", sp.Status.Code)
	}
	if len(sp.Events) == 0 {
		t.Fatal("want recorded error event")
	}
}

func TestBatchDeleteObjectSpan(t *testing.T) {
	s, exporter := newTestStorage(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || !r.URL.Query().Has("delete") {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `<DeleteResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/"><Deleted><Key>a</Key></Deleted><Deleted><Key>b</Key></Deleted><Deleted><Key>c</Key></Deleted></DeleteResult>`)
	})

	if err := s.BatchDeleteObject(context.Background(), &storage.BatchDeleteObjectRequest{Keys: []string{"a", "b", "c"}}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sp := exporter.GetSpans()[0]
	if sp.Name != "S3.DeleteObjects" {
		t.Fatalf("span name: got %q want %q", sp.Name, "S3.DeleteObjects")
	}
	if sp.SpanKind != oteltrace.SpanKindClient {
		t.Fatalf("span kind: got %v want client", sp.SpanKind)
	}
	assertSpanAttr(t, sp, "aws.s3.bucket", "test-bucket")
	assertSpanIntAttr(t, sp, "s3.object_count", 3)
	assertSpanAttr(t, sp, "rpc.method", "S3/DeleteObjects")
	if sp.Status.Code != codes.Unset {
		t.Fatalf("status: got %v want unset", sp.Status.Code)
	}
}

func TestBatchDeleteObjectSpanRecordsError(t *testing.T) {
	s, exporter := newTestStorage(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `<DeleteResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/"><Error><Key>a</Key><Code>AccessDenied</Code><Message>denied</Message></Error></DeleteResult>`)
	})

	if err := s.BatchDeleteObject(context.Background(), &storage.BatchDeleteObjectRequest{Keys: []string{"a"}}); err == nil {
		t.Fatal("want error")
	}

	sp := exporter.GetSpans()[0]
	if sp.Status.Code != codes.Error {
		t.Fatalf("status: got %v want error", sp.Status.Code)
	}
	if len(sp.Events) == 0 {
		t.Fatal("want recorded error event")
	}
}

func TestPresignedGetObjectSpan(t *testing.T) {
	s, exporter := newTestStorage(t, func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
	})

	resp, err := s.PresignedGetObject(context.Background(), &storage.PresignedGetObjectRequest{Key: "key-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Url == "" {
		t.Fatal("want non-empty url")
	}

	sp := exporter.GetSpans()[0]
	if sp.Name != "S3.PresignedGetObject" {
		t.Fatalf("span name: got %q want %q", sp.Name, "S3.PresignedGetObject")
	}
	assertSpanAttr(t, sp, "aws.s3.bucket", "test-bucket")
	assertSpanAttr(t, sp, "aws.s3.key", "key-1")
}

func TestPresignedPostPolicySpan(t *testing.T) {
	s, exporter := newTestStorage(t, func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
	})

	resp, err := s.PresignedPostPolicy(context.Background(), &storage.PresignedPostPolicyRequest{Key: "key-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Url == "" {
		t.Fatal("want non-empty url")
	}

	sp := exporter.GetSpans()[0]
	if sp.Name != "S3.PresignedPostPolicy" {
		t.Fatalf("span name: got %q want %q", sp.Name, "S3.PresignedPostPolicy")
	}
	assertSpanAttr(t, sp, "aws.s3.bucket", "test-bucket")
	assertSpanAttr(t, sp, "aws.s3.key", "key-1")
}

func assertSpanAttr(t *testing.T, sp tracetest.SpanStub, key, want string) {
	t.Helper()
	for _, a := range sp.Attributes {
		if string(a.Key) == key {
			if got := a.Value.AsString(); got != want {
				t.Fatalf("attr %s: got %q want %q", key, got, want)
			}
			return
		}
	}
	t.Fatalf("attr %s not found", key)
}

func assertSpanIntAttr(t *testing.T, sp tracetest.SpanStub, key string, want int64) {
	t.Helper()
	for _, a := range sp.Attributes {
		if string(a.Key) == key {
			if got := a.Value.AsInt64(); got != want {
				t.Fatalf("attr %s: got %d want %d", key, got, want)
			}
			return
		}
	}
	t.Fatalf("attr %s not found", key)
}

func newTestLiveMinioStorage(t *testing.T) *Storage {
	t.Helper()

	accessKey := os.Getenv("GONOTELM_MINIO_ACCESS_KEY")
	secretKey := os.Getenv("GONOTELM_MINIO_SECRET_KEY")
	if accessKey == "" || secretKey == "" {
		t.Fatal("GONOTELM_MINIO_ACCESS_KEY / GONOTELM_MINIO_SECRET_KEY 未设置")
	}

	endpoint := os.Getenv("GONOTELM_MINIO_ENDPOINT")
	if endpoint == "" {
		endpoint = "127.0.0.1:9000"
	}
	region := os.Getenv("GONOTELM_MINIO_REGION")
	if region == "" {
		region = "us-east-1"
	}
	const bucket = "gonotelm-test"

	s, err := New(&storage.Config{
		Endpoint:  endpoint,
		Region:    region,
		Bucket:    bucket,
		AccessKey: accessKey,
		SecretKey: secretKey,
		Secure:    os.Getenv("GONOTELM_MINIO_SECURE") == "true",
	})
	if err != nil {
		t.Fatalf("create live minio storage: %v", err)
	}

	ctx := context.Background()
	exists, err := s.client.BucketExists(ctx, s.bucket)
	if err != nil {
		t.Fatalf("bucket exists: %v", err)
	}
	if !exists {
		if err := s.client.MakeBucket(ctx, s.bucket, minio.MakeBucketOptions{Region: s.region}); err != nil {
			t.Fatalf("make bucket: %v", err)
		}
	}

	return s
}

func TestGetPartialObject(t *testing.T) {
	ctx := context.Background()
	s := newTestLiveMinioStorage(t)

	key := fmt.Sprintf("test/get-partial-object/%d", time.Now().UnixNano())
	payload := []byte("abcdefghijklmnopqrstuvwxyz")
	const offset int64 = 3
	const length int64 = 5

	t.Cleanup(func() {
		_ = s.DeleteObject(context.Background(), &storage.DeleteObjectRequest{Key: key})
	})

	if err := s.UploadObject(ctx, &storage.UploadObjectRequest{
		Key:         key,
		Body:        payload,
		ContentType: "text/plain",
	}); err != nil {
		t.Fatalf("upload: %v", err)
	}

	resp, err := s.GetPartialObject(ctx, &storage.GetPartialObjectRequest{
		Key:    key,
		Offset: offset,
		Length: length,
	})
	if err != nil {
		t.Fatalf("GetPartialObject: %v", err)
	}

	t.Logf("total = %s\n", string(payload))
	t.Logf("partial = %s\n", string(resp.Body))
	t.Logf("bytesRead = %d\n", resp.BytesRead)
	t.Logf("info.size = %d\n", resp.Info.Size)
	t.Logf("info.key = %s\n", resp.Info.Key)

	want := payload[offset : offset+length]
	if string(resp.Body) != string(want) {
		t.Fatalf("body: got %q want %q", resp.Body, want)
	}
	if resp.BytesRead != len(want) {
		t.Fatalf("bytesRead: got %d want %d", resp.BytesRead, len(want))
	}
	if resp.Info.Size != int64(len(payload)) {
		t.Fatalf("info.size: got %d want %d", resp.Info.Size, len(payload))
	}
	if resp.Info.Key != key {
		t.Fatalf("info.key: got %q want %q", resp.Info.Key, key)
	}
}

func uploadTestObject(t *testing.T, s *Storage, payload []byte) string {
	t.Helper()
	key := fmt.Sprintf("test/get-partial-object/%d", time.Now().UnixNano())
	t.Cleanup(func() {
		_ = s.DeleteObject(context.Background(), &storage.DeleteObjectRequest{Key: key})
	})
	if err := s.UploadObject(context.Background(), &storage.UploadObjectRequest{
		Key:         key,
		Body:        payload,
		ContentType: "text/plain",
	}); err != nil {
		t.Fatalf("upload: %v", err)
	}
	return key
}

func TestGetPartialObjectReadToEnd(t *testing.T) {
	s := newTestLiveMinioStorage(t)
	payload := []byte("abcdefghijklmnopqrstuvwxyz")
	key := uploadTestObject(t, s, payload)

	const offset int64 = 20
	resp, err := s.GetPartialObject(context.Background(), &storage.GetPartialObjectRequest{
		Key:    key,
		Offset: offset,
		Length: 20,
	})
	if err != nil {
		t.Fatalf("GetPartialObject: %v", err)
	}

	want := payload[offset:]
	if string(resp.Body) != string(want) {
		t.Fatalf("body: got %q want %q", resp.Body, want)
	}
	if resp.BytesRead != len(want) {
		t.Fatalf("bytesRead: got %d want %d", resp.BytesRead, len(want))
	}
}

func TestGetPartialObjectOffsetBeyondSize(t *testing.T) {
	s := newTestLiveMinioStorage(t)
	payload := []byte("abcdefghijklmnopqrstuvwxyz")
	key := uploadTestObject(t, s, payload)

	_, err := s.GetPartialObject(context.Background(), &storage.GetPartialObjectRequest{
		Key:    key,
		Offset: int64(len(payload)),
		Length: 1,
	})
	if err == nil {
		t.Fatal("want error when offset is at/beyond object size")
	}
}

func TestGetPartialObjectInvalidRequest(t *testing.T) {
	s := &Storage{}
	ctx := context.Background()

	tests := []struct {
		name string
		req  *storage.GetPartialObjectRequest
	}{
		{name: "nil request"},
		{name: "empty key", req: &storage.GetPartialObjectRequest{Offset: 0, Length: 1}},
		{name: "negative offset", req: &storage.GetPartialObjectRequest{Key: "k", Offset: -1, Length: 1}},
		{name: "zero length", req: &storage.GetPartialObjectRequest{Key: "k", Offset: 0, Length: 0}},
		{name: "negative length", req: &storage.GetPartialObjectRequest{Key: "k", Offset: 0, Length: -1}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := s.GetPartialObject(ctx, tt.req)
			if err == nil {
				t.Fatal("want error")
			}
			if !stderrors.Is(err, pkgerrors.ErrParams) {
				t.Fatalf("want ErrParams, got %v", err)
			}
		})
	}
}
