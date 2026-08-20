package types

import (
	"context"
	"io"

	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/storage"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

// UploadReader 将 src 流式上传到对象存储（pipe 边读边写，不整包进内存）。
// 不关闭 src，所有权仍在调用方。
// 对于需要将大量数据从一个reader搬移到另一个reader的的场景较为有效
func UploadReader(
	ctx context.Context,
	sto storage.ObjectUploader,
	key, contentType string,
	src io.Reader,
) error {
	pr, pw := io.Pipe()
	errCh := make(chan error, 1)

	go func() {
		defer pw.Close()
		_, err := io.Copy(pw, src)
		select {
		case errCh <- err:
		case <-ctx.Done():
		}
	}()

	uploadErr := sto.UploadObject(ctx, &storage.UploadObjectRequest{
		Key:         key,
		BodyReader:  pr,
		ContentType: contentType,
	})

	var copyErr error
	select {
	case copyErr = <-errCh:
	case <-ctx.Done():
		if err := ctx.Err(); err != nil {
			return errors.WithStack(err)
		}
	}

	if copyErr != nil {
		return errors.Wrap(copyErr, "copy source to upload pipe failed")
	}
	if uploadErr != nil {
		return errors.Wrapf(uploadErr, "upload object failed, key=%s", key)
	}
	return nil
}
