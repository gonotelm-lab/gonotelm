package worker

import (
	"context"
	"io"
	"testing"

	"github.com/bytedance/sonic"
	flowworker "github.com/gonotelm-lab/flow/client/worker"
	"github.com/gonotelm-lab/gonotelm/internal/application/shared/contract"
	"github.com/gonotelm-lab/gonotelm/internal/application/worker/artifact/types"
	"github.com/gonotelm-lab/gonotelm/internal/core/adapter"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	artifactentity "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"
	artifacterrors "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/errors"
	artifactrepo "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/repository"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	. "github.com/smartystreets/goconvey/convey"
)

type guardTestRepo struct {
	err error
}

func (r *guardTestRepo) Save(context.Context, *artifactentity.Artifact) error { return nil }
func (r *guardTestRepo) FindById(context.Context, valobj.Id) (*artifactentity.Artifact, error) {
	if r.err != nil {
		return nil, r.err
	}

	return &artifactentity.Artifact{}, nil
}
func (r *guardTestRepo) ListByNotebookId(context.Context, valobj.Id, *artifactrepo.ListSpec) ([]*artifactentity.Artifact, error) {
	return nil, nil
}
func (r *guardTestRepo) ListByStatus(context.Context, *artifactrepo.ListByStatusSpec) ([]*artifactentity.Artifact, error) {
	return nil, nil
}
func (r *guardTestRepo) DeleteById(context.Context, valobj.Id) error { return nil }
func (r *guardTestRepo) DeleteByNotebookId(context.Context, valobj.Id) error {
	return nil
}

var _ artifactrepo.Repository = &guardTestRepo{}

type guardTestStorage struct {
	deletedKeys []valobj.StoreKey
	err         error
}

func (s *guardTestStorage) GetObject(context.Context, valobj.StoreKey) ([]byte, *adapter.ObjectInfo, error) {
	return nil, nil, nil
}
func (s *guardTestStorage) GetPartialObject(context.Context, valobj.StoreKey, int64, int64) ([]byte, *adapter.ObjectInfo, error) {
	return nil, nil, nil
}
func (s *guardTestStorage) Upload(context.Context, valobj.StoreKey, []byte, string) error { return nil }
func (s *guardTestStorage) UploadReader(context.Context, valobj.StoreKey, string, io.Reader) error {
	return nil
}
func (s *guardTestStorage) PresignUpload(context.Context, valobj.StoreKey, *adapter.UploadOptions) (*adapter.PresignUploadResult, error) {
	return nil, nil
}
func (s *guardTestStorage) DeleteObject(context.Context, valobj.StoreKey) error { return nil }
func (s *guardTestStorage) BatchDeleteObject(_ context.Context, keys []valobj.StoreKey) error {
	s.deletedKeys = append(s.deletedKeys, keys...)

	return s.err
}
func (s *guardTestStorage) PublicURL(context.Context, valobj.StoreKey) (string, error) {
	return "", nil
}
func (s *guardTestStorage) PresignGet(context.Context, valobj.StoreKey) (string, error) {
	return "", nil
}
func (s *guardTestStorage) CheckExist(context.Context, valobj.StoreKey) (bool, error) {
	return false, nil
}

var _ adapter.ObjectStore = &guardTestStorage{}

func TestArtifactGone(t *testing.T) {
	Convey("artifactGone only reports a deleted row", t, func() {
		Convey("row gone", func() {
			deps := &types.WorkerDeps{ArtifactRepo: &guardTestRepo{err: artifacterrors.ErrArtifactNotFound}}
			So(artifactGone(t.Context(), deps, valobj.NewId()), ShouldBeTrue)
		})

		Convey("row still there", func() {
			deps := &types.WorkerDeps{ArtifactRepo: &guardTestRepo{}}
			So(artifactGone(t.Context(), deps, valobj.NewId()), ShouldBeFalse)
		})

		Convey("query failure fails open", func() {
			deps := &types.WorkerDeps{ArtifactRepo: &guardTestRepo{err: errors.ErrInner.Msg("db down")}}
			So(artifactGone(t.Context(), deps, valobj.NewId()), ShouldBeFalse)
		})

		Convey("no repo configured", func() {
			So(artifactGone(t.Context(), &types.WorkerDeps{}, valobj.NewId()), ShouldBeFalse)
		})
	})
}

func TestDiscardStorageResult(t *testing.T) {
	Convey("discardStorageResult deletes the uploaded object", t, func() {
		key, err := valobj.NewStoreKey("gonotelm-private", "artifact/x/y.pptx", false)
		So(err, ShouldBeNil)

		storageResult, err := sonic.Marshal(contract.StorageResult{StoreKey: key, ContentType: "application/pdf"})
		So(err, ShouldBeNil)

		storage := &guardTestStorage{}
		deps := &types.WorkerDeps{ObjectStorage: storage}

		Convey("storage result", func() {
			discardStorageResult(t.Context(), deps, &types.Response{Result: storageResult, ResultKind: artifactentity.ResultKindStorage})
			So(storage.deletedKeys, ShouldResemble, []valobj.StoreKey{key})
		})

		Convey("inline result", func() {
			discardStorageResult(t.Context(), deps, &types.Response{Result: []byte("text"), ResultKind: artifactentity.ResultKindInline})
			So(storage.deletedKeys, ShouldBeEmpty)
		})

		Convey("nil response", func() {
			discardStorageResult(t.Context(), deps, nil)
			So(storage.deletedKeys, ShouldBeEmpty)
		})
	})
}

func TestWorkerFnSkipsDeletedArtifact(t *testing.T) {
	Convey("generation is skipped once the artifact row is gone", t, func() {
		deps := &types.WorkerDeps{ArtifactRepo: &guardTestRepo{err: artifacterrors.ErrArtifactNotFound}}
		fn := workerFn(deps)

		res, err := fn(t.Context(), contract.WorkerInput{
			ArtifactId: valobj.NewId().String(),
			NotebookId: valobj.NewId().String(),
			UserId:     valobj.NewUid().String(),
			Kind:       string(artifactentity.KindMindmap),
			Payload:    []byte("{}"),
		})
		So(err, ShouldBeNil)

		errResult, ok := res.(flowworker.ErrorResult)
		So(ok, ShouldBeTrue)
		So(errResult.SkipRetry, ShouldBeTrue)
	})
}
