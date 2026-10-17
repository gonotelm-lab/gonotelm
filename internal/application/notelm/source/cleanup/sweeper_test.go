package cleanup

import (
	"context"
	"io"
	"testing"

	"github.com/gonotelm-lab/gonotelm/internal/core/adapter"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	notebookentity "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/entity"
	notebookrepo "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/repository"
	sourceentity "github.com/gonotelm-lab/gonotelm/internal/domain/source/entity"
	sourceerr "github.com/gonotelm-lab/gonotelm/internal/domain/source/errors"
	sourcerepo "github.com/gonotelm-lab/gonotelm/internal/domain/source/repository"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	. "github.com/smartystreets/goconvey/convey"
)

const testBucket = "gonotelm-private"

// ── fakes ──────────────────────────────────────────────────────────────────

type fakeSourceRepo struct {
	source        *sourceentity.Source
	findErr       error
	deletedByNb   []valobj.Id
	deletedSource []valobj.Id
}

func (f *fakeSourceRepo) Create(context.Context, *sourceentity.Source) error { return nil }
func (f *fakeSourceRepo) UpdateExisting(context.Context, *sourceentity.Source) (int64, error) {
	return 0, nil
}
func (f *fakeSourceRepo) DeleteById(_ context.Context, id valobj.Id) error {
	f.deletedSource = append(f.deletedSource, id)

	return nil
}
func (f *fakeSourceRepo) FindById(context.Context, valobj.Id) (*sourceentity.Source, error) {
	if f.findErr != nil {
		return nil, f.findErr
	}
	if f.source != nil {
		return f.source, nil
	}

	return nil, sourceerr.ErrSourceNotFound
}
func (f *fakeSourceRepo) ListByNotebookId(context.Context, valobj.Id, *sourcerepo.ListSpec) ([]*sourceentity.Source, error) {
	return nil, nil
}
func (f *fakeSourceRepo) GetByNotebookIdAndIds(context.Context, valobj.Id, []valobj.Id) ([]*sourceentity.Source, error) {
	return nil, nil
}
func (f *fakeSourceRepo) DeleteByNotebookId(_ context.Context, id valobj.Id) error {
	f.deletedByNb = append(f.deletedByNb, id)

	return nil
}
func (f *fakeSourceRepo) BatchDeleteByIds(context.Context, []valobj.Id) error { return nil }

type fakeNotebookRepo struct {
	exists bool
	err    error
}

func (f *fakeNotebookRepo) Save(context.Context, *notebookentity.Notebook) error { return nil }
func (f *fakeNotebookRepo) FindById(context.Context, valobj.Id) (*notebookentity.Notebook, error) {
	return nil, errors.ErrNoRecord
}
func (f *fakeNotebookRepo) Exists(context.Context, valobj.Id) (bool, error) {
	return f.exists, f.err
}
func (f *fakeNotebookRepo) ListByOwner(context.Context, valobj.Uid, *notebookrepo.ListSpec) ([]*notebookentity.Notebook, error) {
	return nil, nil
}

type fakeSourceDocRepo struct {
	deletedNotebookId valobj.Id
	deletedSourceIds  []valobj.Id
	err               error
	calls             int
}

func (f *fakeSourceDocRepo) FindById(context.Context, valobj.Id, valobj.Id, valobj.Id) (*sourceentity.SourceDoc, error) {
	return nil, nil
}
func (f *fakeSourceDocRepo) BatchFind(context.Context, valobj.Id, valobj.Id, []valobj.Id) ([]*sourceentity.SourceDoc, error) {
	return nil, nil
}
func (f *fakeSourceDocRepo) BatchSave(context.Context, []*sourceentity.SourceDoc) error { return nil }
func (f *fakeSourceDocRepo) BatchDeleteBySourceId(
	_ context.Context,
	notebookId valobj.Id,
	sourceIds []valobj.Id,
) error {
	f.calls++
	f.deletedNotebookId = notebookId
	f.deletedSourceIds = sourceIds

	return f.err
}
func (f *fakeSourceDocRepo) Query(context.Context, *sourcerepo.SourceDocQueryParams) ([]*sourceentity.SourceDoc, error) {
	return nil, nil
}

type fakeObjectStore struct {
	deletedKeys []valobj.StoreKey
	calls       int
	err         error
}

func (f *fakeObjectStore) GetObject(context.Context, valobj.StoreKey) ([]byte, *adapter.ObjectInfo, error) {
	return nil, nil, nil
}
func (f *fakeObjectStore) GetPartialObject(context.Context, valobj.StoreKey, int64, int64) ([]byte, *adapter.ObjectInfo, error) {
	return nil, nil, nil
}
func (f *fakeObjectStore) Upload(context.Context, valobj.StoreKey, []byte, string) error { return nil }
func (f *fakeObjectStore) UploadReader(context.Context, valobj.StoreKey, string, io.Reader) error {
	return nil
}
func (f *fakeObjectStore) PresignUpload(context.Context, valobj.StoreKey, *adapter.UploadOptions) (*adapter.PresignUploadResult, error) {
	return nil, nil
}
func (f *fakeObjectStore) DeleteObject(context.Context, valobj.StoreKey) error { return nil }
func (f *fakeObjectStore) BatchDeleteObject(_ context.Context, keys []valobj.StoreKey) error {
	f.calls++
	f.deletedKeys = append(f.deletedKeys, keys...)

	return f.err
}
func (f *fakeObjectStore) PublicURL(context.Context, valobj.StoreKey) (string, error) { return "", nil }
func (f *fakeObjectStore) PresignGet(context.Context, valobj.StoreKey) (string, error) {
	return "", nil
}
func (f *fakeObjectStore) CheckExist(context.Context, valobj.StoreKey) (bool, error) {
	return false, nil
}

// ── tests ──────────────────────────────────────────────────────────────────

func newTestSweeper(
	source *fakeSourceRepo,
	notebook *fakeNotebookRepo,
	docs *fakeSourceDocRepo,
	objects *fakeObjectStore,
) *Sweeper {
	return NewSweeper(source, notebook, docs, objects)
}

func TestSweepSourceSkipsWhenRowStillExists(t *testing.T) {
	Convey("skips sweeping while the source row still exists", t, func() {
		source := &fakeSourceRepo{source: &sourceentity.Source{}}
		docs := &fakeSourceDocRepo{}
		objects := &fakeObjectStore{}
		sweeper := newTestSweeper(source, &fakeNotebookRepo{}, docs, objects)

		notebookId, sourceId := valobj.NewId(), valobj.NewId()
		So(sweeper.SweepSource(t.Context(), notebookId, sourceId, nil), ShouldBeNil)
		So(docs.calls, ShouldEqual, 0)
		So(objects.calls, ShouldEqual, 0)
	})
}

func TestSweepSourcePurgesDocsAndObjects(t *testing.T) {
	Convey("purges docs and objects once the source row is gone", t, func() {
		source := &fakeSourceRepo{}
		docs := &fakeSourceDocRepo{}
		objects := &fakeObjectStore{}
		sweeper := newTestSweeper(source, &fakeNotebookRepo{}, docs, objects)

		notebookId, sourceId := valobj.NewId(), valobj.NewId()
		fileKey, err := valobj.NewStoreKey(testBucket, "file/"+notebookId.String()+"/"+sourceId.String()+".pdf", false)
		So(err, ShouldBeNil)

		So(sweeper.SweepSource(t.Context(), notebookId, sourceId, []string{fileKey.Encode()}), ShouldBeNil)

		So(docs.calls, ShouldEqual, 1)
		So(docs.deletedNotebookId, ShouldEqual, notebookId)
		So(docs.deletedSourceIds, ShouldResemble, []valobj.Id{sourceId})

		// 只删载荷里带的 key，不做推导
		So(objects.deletedKeys, ShouldResemble, []valobj.StoreKey{fileKey})
	})
}

func TestSweepSourceIgnoresBadObjectKey(t *testing.T) {
	Convey("a bad object key in the payload is skipped", t, func() {
		docs := &fakeSourceDocRepo{}
		objects := &fakeObjectStore{}
		sweeper := newTestSweeper(&fakeSourceRepo{}, &fakeNotebookRepo{}, docs, objects)

		notebookId, sourceId := valobj.NewId(), valobj.NewId()
		goodKey, err := valobj.NewStoreKey(testBucket, "file/x/y.pdf", false)
		So(err, ShouldBeNil)

		So(sweeper.SweepSource(t.Context(), notebookId, sourceId, []string{"!!!not-a-storekey", goodKey.Encode()}), ShouldBeNil)
		So(docs.calls, ShouldEqual, 1)
		So(objects.deletedKeys, ShouldResemble, []valobj.StoreKey{goodKey})
	})
}

func TestSweepSourcePropagatesDependencyError(t *testing.T) {
	Convey("dependency failures bubble up so flow retries", t, func() {
		docs := &fakeSourceDocRepo{err: errors.ErrInner.Msg("milvus down")}
		sweeper := newTestSweeper(&fakeSourceRepo{}, &fakeNotebookRepo{}, docs, &fakeObjectStore{})

		err := sweeper.SweepSource(t.Context(), valobj.NewId(), valobj.NewId(), nil)
		So(err, ShouldNotBeNil)
	})
}

func TestSweepSourceDeletesObjectsEvenWhenDocsFail(t *testing.T) {
	Convey("objects are still deleted when docs deletion fails", t, func() {
		docs := &fakeSourceDocRepo{err: errors.ErrInner.Msg("milvus down")}
		objects := &fakeObjectStore{}
		sweeper := newTestSweeper(&fakeSourceRepo{}, &fakeNotebookRepo{}, docs, objects)

		notebookId, sourceId := valobj.NewId(), valobj.NewId()
		fileKey, err := valobj.NewStoreKey(testBucket, "file/x/y.pdf", false)
		So(err, ShouldBeNil)

		err = sweeper.SweepSource(t.Context(), notebookId, sourceId, []string{fileKey.Encode()})
		So(err, ShouldNotBeNil)
		So(objects.calls, ShouldEqual, 1)
		So(objects.deletedKeys, ShouldResemble, []valobj.StoreKey{fileKey})
	})
}

func TestSweepNotebookSkipsWhenNotebookStillExists(t *testing.T) {
	Convey("skips sweeping while the notebook row still exists", t, func() {
		docs := &fakeSourceDocRepo{}
		objects := &fakeObjectStore{}
		source := &fakeSourceRepo{}
		sweeper := newTestSweeper(source, &fakeNotebookRepo{exists: true}, docs, objects)

		So(sweeper.SweepNotebook(t.Context(), valobj.NewId(), nil), ShouldBeNil)
		So(docs.calls, ShouldEqual, 0)
		So(objects.calls, ShouldEqual, 0)
		So(source.deletedByNb, ShouldBeEmpty)
	})
}

func TestSweepNotebookPurgesAllSourcesAndRows(t *testing.T) {
	Convey("purges every source once the notebook row is gone", t, func() {
		docs := &fakeSourceDocRepo{}
		objects := &fakeObjectStore{}
		source := &fakeSourceRepo{}
		sweeper := newTestSweeper(source, &fakeNotebookRepo{}, docs, objects)

		notebookId := valobj.NewId()
		sourceId1, sourceId2 := valobj.NewId(), valobj.NewId()
		key1, err := valobj.NewStoreKey(testBucket, "file/"+notebookId.String()+"/"+sourceId1.String()+".txt", false)
		So(err, ShouldBeNil)

		refs := []SourceRef{
			{SourceId: sourceId1, Keys: []string{key1.Encode()}},
			{SourceId: sourceId2},
		}

		So(sweeper.SweepNotebook(t.Context(), notebookId, refs), ShouldBeNil)

		So(docs.deletedSourceIds, ShouldResemble, []valobj.Id{sourceId1, sourceId2})

		// 只删载荷里带的 key
		So(objects.deletedKeys, ShouldResemble, []valobj.StoreKey{key1})
		So(source.deletedByNb, ShouldResemble, []valobj.Id{notebookId})
	})
}

func TestSweepNotebookDeletesRowsEvenWhenPurgeFails(t *testing.T) {
	Convey("leftover rows are still deleted when object deletion fails", t, func() {
		objects := &fakeObjectStore{err: errors.ErrInner.Msg("minio down")}
		source := &fakeSourceRepo{}
		sweeper := newTestSweeper(source, &fakeNotebookRepo{}, &fakeSourceDocRepo{}, objects)

		notebookId := valobj.NewId()
		err := sweeper.SweepNotebook(t.Context(), notebookId, []SourceRef{{SourceId: valobj.NewId()}})
		So(err, ShouldNotBeNil)
		So(source.deletedByNb, ShouldResemble, []valobj.Id{notebookId})
	})
}

func TestSweepNotebookWithoutSources(t *testing.T) {
	Convey("still deletes leftover rows for a notebook without sources", t, func() {
		docs := &fakeSourceDocRepo{}
		source := &fakeSourceRepo{}
		sweeper := newTestSweeper(source, &fakeNotebookRepo{}, docs, &fakeObjectStore{})

		notebookId := valobj.NewId()
		So(sweeper.SweepNotebook(t.Context(), notebookId, nil), ShouldBeNil)
		So(docs.calls, ShouldEqual, 0)
		So(source.deletedByNb, ShouldResemble, []valobj.Id{notebookId})
	})
}

func TestSweepDispatchByKind(t *testing.T) {
	Convey("Sweep dispatches by kind", t, func() {
		docs := &fakeSourceDocRepo{}
		sweeper := newTestSweeper(&fakeSourceRepo{}, &fakeNotebookRepo{}, docs, &fakeObjectStore{})

		notebookId, sourceId := valobj.NewId(), valobj.NewId()
		So(sweeper.Sweep(t.Context(), &Payload{
			Kind:       KindSource,
			NotebookId: notebookId,
			SourceId:   sourceId,
		}), ShouldBeNil)
		So(docs.calls, ShouldEqual, 1)

		err := sweeper.Sweep(t.Context(), &Payload{Kind: "nope", NotebookId: notebookId})
		So(err, ShouldNotBeNil)
	})
}
