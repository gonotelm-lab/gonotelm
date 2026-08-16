package types

import (
	"bytes"
	"context"
	"io"

	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/storage"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

// UploadReader 将 src 流式上传到对象存储（pipe 边读边写，不整包进内存）。
// 不关闭 src，所有权仍在调用方。
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

// PeekPrefix 预读最多 n 字节，并返回可再次读出「完整原文」的 Reader（prefix + 剩余流）。
// 若原文不足 n 字节，full 仅包含已读内容。
func PeekPrefix(r io.Reader, n int) (prefix []byte, full io.Reader, err error) {
	if n <= 0 {
		return nil, r, nil
	}

	buf := make([]byte, n)
	nr, readErr := io.ReadFull(r, buf)
	prefix = buf[:nr]

	switch {
	case readErr == nil:
		return prefix, io.MultiReader(bytes.NewReader(prefix), r), nil
	case readErr == io.EOF || readErr == io.ErrUnexpectedEOF:
		return prefix, bytes.NewReader(prefix), nil
	default:
		return nil, nil, readErr
	}
}
