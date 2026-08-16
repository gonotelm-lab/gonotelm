package types

import (
	"bytes"
	"context"
	"io"
	"runtime"
	"strings"
	"testing"

	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/storage"

	"github.com/stretchr/testify/require"
)

type captureUploader struct {
	key         string
	contentType string
	body        []byte
}

func (u *captureUploader) UploadObject(_ context.Context, req *storage.UploadObjectRequest) error {
	u.key = req.Key
	u.contentType = req.ContentType
	b, err := io.ReadAll(req.BodyReader)
	if err != nil {
		return err
	}
	u.body = b
	return nil
}

func TestUploadReader_StreamsBody(t *testing.T) {
	up := &captureUploader{}
	src := strings.NewReader("hello-pptx")

	err := UploadReader(context.Background(), up, "artifact/a/b.pptx", "application/vnd.openxmlformats-officedocument.presentationml.presentation", src)
	require.NoError(t, err)
	require.Equal(t, "artifact/a/b.pptx", up.key)
	require.Equal(t, "application/vnd.openxmlformats-officedocument.presentationml.presentation", up.contentType)
	require.Equal(t, []byte("hello-pptx"), up.body)
}

func TestUploadReader_PropagatesCopyError(t *testing.T) {
	up := &captureUploader{}
	err := UploadReader(context.Background(), up, "k", "application/octet-stream", errReader{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "copy source to upload pipe failed")
}

type errReader struct{}

func (errReader) Read([]byte) (int, error) {
	return 0, io.ErrUnexpectedEOF
}

func TestUploadReader_DoesNotCloseSource(t *testing.T) {
	up := &captureUploader{}
	src := io.NopCloser(bytes.NewReader([]byte("x")))
	require.NoError(t, UploadReader(context.Background(), up, "k", "ct", src))
	// NopCloser has no observable closed state; just ensure call succeeds without panic.
	require.Equal(t, []byte("x"), up.body)
}

func TestPeekPrefix_ReassemblesFullStream(t *testing.T) {
	src := strings.NewReader("abcdefghij")
	prefix, full, err := PeekPrefix(src, 4)
	require.NoError(t, err)
	require.Equal(t, []byte("abcd"), prefix)

	all, err := io.ReadAll(full)
	require.NoError(t, err)
	require.Equal(t, []byte("abcdefghij"), all)
}

func TestPeekPrefix_ShortSource(t *testing.T) {
	src := strings.NewReader("hi")
	prefix, full, err := PeekPrefix(src, 8)
	require.NoError(t, err)
	require.Equal(t, []byte("hi"), prefix)

	all, err := io.ReadAll(full)
	require.NoError(t, err)
	require.Equal(t, []byte("hi"), all)
}

// discardUploader 只消费上传体，不把整份对象留在内存，避免上传侧掩盖对比结果。
type discardUploader struct{}

func (discardUploader) UploadObject(_ context.Context, req *storage.UploadObjectRequest) error {
	var r io.Reader
	switch {
	case req.Body != nil:
		r = bytes.NewReader(req.Body)
	case req.BodyReader != nil:
		r = req.BodyReader
	default:
		return nil
	}
	_, err := io.Copy(io.Discard, r)
	return err
}

// genReader 按需生成 size 字节，自身不持有整份缓冲（模拟沙箱流式读大文件）。
type genReader struct {
	remain int64
}

func newGenReader(size int64) *genReader {
	return &genReader{remain: size}
}

func (r *genReader) Read(p []byte) (int, error) {
	if r.remain <= 0 {
		return 0, io.EOF
	}
	n := len(p)
	if int64(n) > r.remain {
		n = int(r.remain)
	}
	for i := 0; i < n; i++ {
		p[i] = byte(i)
	}
	r.remain -= int64(n)
	return n, nil
}

// uploadViaReadAll 模拟「先 ReadAll 再 Upload Body」的老路径。
func uploadViaReadAll(ctx context.Context, sto storage.ObjectUploader, key, contentType string, src io.Reader) error {
	body, err := io.ReadAll(src)
	if err != nil {
		return err
	}
	return sto.UploadObject(ctx, &storage.UploadObjectRequest{
		Key:         key,
		Body:        body,
		ContentType: contentType,
	})
}

func allocDeltaBytes(fn func()) uint64 {
	runtime.GC()
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	fn()
	runtime.ReadMemStats(&after)
	return after.TotalAlloc - before.TotalAlloc
}

// TestUploadLarge_PipeVsReadAll_Memory 对比大数据下 pipe 流式 vs ReadAll 的堆分配。
// 跑法: go test ./internal/application/worker/artifact/types/ -run PipeVsReadAll -v -count=1
func TestUploadLarge_PipeVsReadAll_Memory(t *testing.T) {
	if testing.Short() {
		t.Skip("skip large memory comparison in -short")
	}

	const size = 32 << 20 // 32 MiB
	ctx := context.Background()
	up := discardUploader{}

	pipeBytes := allocDeltaBytes(func() {
		require.NoError(t, UploadReader(ctx, up, "k-pipe", "application/octet-stream", newGenReader(size)))
	})
	readAllBytes := allocDeltaBytes(func() {
		require.NoError(t, uploadViaReadAll(ctx, up, "k-readall", "application/octet-stream", newGenReader(size)))
	})

	t.Logf("payload=%d MiB", size>>20)
	t.Logf("pipe UploadReader TotalAllocΔ = %d bytes (%.2f MiB)", pipeBytes, float64(pipeBytes)/(1<<20))
	t.Logf("ReadAll+Upload Body TotalAllocΔ = %d bytes (%.2f MiB)", readAllBytes, float64(readAllBytes)/(1<<20))
	if pipeBytes > 0 {
		t.Logf("ReadAll / pipe ≈ %.1fx", float64(readAllBytes)/float64(pipeBytes))
	}

	// ReadAll 至少应接近整份 payload；pipe 应远小于整份（允许管道/栈等开销）。
	require.Greater(t, readAllBytes, uint64(size)*8/10, "ReadAll path should allocate ~payload size")
	require.Less(t, pipeBytes, uint64(size)/4, "pipe path should allocate much less than payload")
	require.Less(t, pipeBytes*4, readAllBytes, "pipe should be clearly cheaper than ReadAll")
}

func BenchmarkUploadLarge_Pipe(b *testing.B) {
	const size = 32 << 20
	ctx := context.Background()
	up := discardUploader{}
	b.SetBytes(size)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := UploadReader(ctx, up, "k", "application/octet-stream", newGenReader(size)); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkUploadLarge_ReadAll(b *testing.B) {
	const size = 32 << 20
	ctx := context.Background()
	up := discardUploader{}
	b.SetBytes(size)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := uploadViaReadAll(ctx, up, "k", "application/octet-stream", newGenReader(size)); err != nil {
			b.Fatal(err)
		}
	}
}

// 防止编译器把对照路径优化掉。
var sink int

func BenchmarkUploadLarge_ReadAll_FromBytesBuffer(b *testing.B) {
	const size = 32 << 20
	payload := make([]byte, size)
	ctx := context.Background()
	up := discardUploader{}
	b.SetBytes(size)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// 源已是整包时，ReadAll 仍会再拷一份。
		if err := uploadViaReadAll(ctx, up, "k", "application/octet-stream", bytes.NewReader(payload)); err != nil {
			b.Fatal(err)
		}
		sink++
	}
}
