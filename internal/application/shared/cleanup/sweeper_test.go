package cleanup

import (
	"context"
	"io"
	"testing"

	"github.com/gonotelm-lab/gonotelm/internal/core/adapter"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	artifactentity "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"
	artifacterrors "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/errors"
	artifactrepo "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/repository"
	notebookentity "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/entity"
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
	sources       []*sourceentity.Source
	findErr       error
	listErr       error
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
func (f *fakeSourceRepo) ListByNotebookId(_ context.Context, _ valobj.Id, spec *sourcerepo.ListSpec) ([]*sourceentity.Source, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}

	return pageOf(f.sources, spec.Offset, spec.Limit), nil
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
func (f *fakeNotebookRepo) ListByOwner(context.Context, valobj.Uid) ([]*notebookentity.Notebook, error) {
	return nil, nil
}
func (f *fakeNotebookRepo) FindByIds(context.Context, []valobj.Id) ([]*notebookentity.Notebook, error) {
	return nil, nil
}

type fakeArtifactRepo struct {
	artifact    *artifactentity.Artifact
	artifacts   []*artifactentity.Artifact
	findErr     error
	listErr     error
	deletedByNb []valobj.Id
	err         error
}

func (f *fakeArtifactRepo) Create(context.Context, *artifactentity.Artifact) error { return nil }
func (f *fakeArtifactRepo) UpdateExisting(context.Context, *artifactentity.Artifact) (int64, error) {
	return 1, nil
}
func (f *fakeArtifactRepo) FindById(context.Context, valobj.Id) (*artifactentity.Artifact, error) {
	if f.findErr != nil {
		return nil, f.findErr
	}
	if f.artifact != nil {
		return f.artifact, nil
	}

	return nil, artifacterrors.ErrArtifactNotFound
}
func (f *fakeArtifactRepo) ListByNotebookId(_ context.Context, _ valobj.Id, spec *artifactrepo.ListSpec) ([]*artifactentity.Artifact, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}

	return pageOf(f.artifacts, spec.Offset, spec.Limit), nil
}
func (f *fakeArtifactRepo) ListByStatus(context.Context, *artifactrepo.ListByStatusSpec) ([]*artifactentity.Artifact, error) {
	return nil, nil
}
func (f *fakeArtifactRepo) DeleteById(context.Context, valobj.Id) error { return nil }
func (f *fakeArtifactRepo) DeleteByNotebookId(_ context.Context, id valobj.Id) error {
	f.deletedByNb = append(f.deletedByNb, id)

	return f.err
}

var _ artifactrepo.Repository = &fakeArtifactRepo{}

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

func pageOf[T any](all []T, offset, limit int) []T {
	if offset >= len(all) {
		return nil
	}
	end := min(offset+limit, len(all))

	return all[offset:end]
}

// ── tests ──────────────────────────────────────────────────────────────────

func newTestSweeper(
	source *fakeSourceRepo,
	notebook *fakeNotebookRepo,
	artifacts *fakeArtifactRepo,
	docs *fakeSourceDocRepo,
	objects *fakeObjectStore,
) *sweeper {
	return newSweeper(source, notebook, artifacts, docs, objects)
}

func newDefaultTestSweeper(source *fakeSourceRepo, notebook *fakeNotebookRepo, docs *fakeSourceDocRepo, objects *fakeObjectStore) *sweeper {
	return newTestSweeper(source, notebook, &fakeArtifactRepo{}, docs, objects)
}

func TestSweepSourceSkipsWhenRowStillExists(t *testing.T) {
	Convey("skips sweeping while the source row still exists", t, func() {
		source := &fakeSourceRepo{source: &sourceentity.Source{}}
		docs := &fakeSourceDocRepo{}
		objects := &fakeObjectStore{}
		sweeper := newDefaultTestSweeper(source, &fakeNotebookRepo{}, docs, objects)

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
		sweeper := newDefaultTestSweeper(source, &fakeNotebookRepo{}, docs, objects)

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
		sweeper := newDefaultTestSweeper(&fakeSourceRepo{}, &fakeNotebookRepo{}, docs, objects)

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
		sweeper := newDefaultTestSweeper(&fakeSourceRepo{}, &fakeNotebookRepo{}, docs, &fakeObjectStore{})

		err := sweeper.SweepSource(t.Context(), valobj.NewId(), valobj.NewId(), nil)
		So(err, ShouldNotBeNil)
	})
}

func TestSweepSourceDeletesObjectsEvenWhenDocsFail(t *testing.T) {
	Convey("objects are still deleted when docs deletion fails", t, func() {
		docs := &fakeSourceDocRepo{err: errors.ErrInner.Msg("milvus down")}
		objects := &fakeObjectStore{}
		sweeper := newDefaultTestSweeper(&fakeSourceRepo{}, &fakeNotebookRepo{}, docs, objects)

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
		sweeper := newDefaultTestSweeper(source, &fakeNotebookRepo{exists: true}, docs, objects)

		So(sweeper.SweepNotebook(t.Context(), valobj.NewId(), nil, nil), ShouldBeNil)
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
		sweeper := newDefaultTestSweeper(source, &fakeNotebookRepo{}, docs, objects)

		notebookId := valobj.NewId()
		sourceId1, sourceId2 := valobj.NewId(), valobj.NewId()
		key1, err := valobj.NewStoreKey(testBucket, "file/"+notebookId.String()+"/"+sourceId1.String()+".txt", false)
		So(err, ShouldBeNil)

		refs := []SourceRef{
			{SourceId: sourceId1, Keys: []string{key1.Encode()}},
			{SourceId: sourceId2},
		}

		So(sweeper.SweepNotebook(t.Context(), notebookId, refs, nil), ShouldBeNil)

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
		sweeper := newDefaultTestSweeper(source, &fakeNotebookRepo{}, &fakeSourceDocRepo{}, objects)

		notebookId := valobj.NewId()
		err := sweeper.SweepNotebook(t.Context(), notebookId, []SourceRef{{SourceId: valobj.NewId()}}, nil)
		So(err, ShouldNotBeNil)
		So(source.deletedByNb, ShouldResemble, []valobj.Id{notebookId})
	})
}

func TestSweepNotebookWithoutSources(t *testing.T) {
	Convey("still deletes leftover rows for a notebook without sources", t, func() {
		docs := &fakeSourceDocRepo{}
		source := &fakeSourceRepo{}
		sweeper := newDefaultTestSweeper(source, &fakeNotebookRepo{}, docs, &fakeObjectStore{})

		notebookId := valobj.NewId()
		So(sweeper.SweepNotebook(t.Context(), notebookId, nil, nil), ShouldBeNil)
		So(docs.calls, ShouldEqual, 0)
		So(source.deletedByNb, ShouldResemble, []valobj.Id{notebookId})
	})
}

func TestSweepDispatchByKind(t *testing.T) {
	Convey("Sweep dispatches by kind", t, func() {
		docs := &fakeSourceDocRepo{}
		sweeper := newDefaultTestSweeper(&fakeSourceRepo{}, &fakeNotebookRepo{}, docs, &fakeObjectStore{})

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

func TestSweepNotebookPurgesArtifactObjectsAndRows(t *testing.T) {
	Convey("purges artifact objects once the notebook row is gone", t, func() {
		artifacts := &fakeArtifactRepo{}
		objects := &fakeObjectStore{}
		sweeper := newTestSweeper(&fakeSourceRepo{}, &fakeNotebookRepo{}, artifacts, &fakeSourceDocRepo{}, objects)

		notebookId, artifactId := valobj.NewId(), valobj.NewId()
		key, err := valobj.NewStoreKey(testBucket, "artifact/"+notebookId.String()+"/"+artifactId.String()+".pptx", false)
		So(err, ShouldBeNil)

		refs := []ArtifactRef{{ArtifactId: artifactId, Keys: []string{key.Encode()}}}

		So(sweeper.SweepNotebook(t.Context(), notebookId, nil, refs), ShouldBeNil)

		So(objects.deletedKeys, ShouldResemble, []valobj.StoreKey{key})
		So(artifacts.deletedByNb, ShouldResemble, []valobj.Id{notebookId})
	})
}

func TestSweepNotebookDeletesArtifactRowsWhenObjectDeletionFails(t *testing.T) {
	Convey("artifact rows are still deleted when object deletion fails", t, func() {
		artifacts := &fakeArtifactRepo{}
		objects := &fakeObjectStore{err: errors.ErrInner.Msg("minio down")}
		sweeper := newTestSweeper(&fakeSourceRepo{}, &fakeNotebookRepo{}, artifacts, &fakeSourceDocRepo{}, objects)

		notebookId := valobj.NewId()
		err := sweeper.SweepNotebook(t.Context(), notebookId, nil, []ArtifactRef{{ArtifactId: valobj.NewId()}})
		So(err, ShouldNotBeNil)
		So(artifacts.deletedByNb, ShouldResemble, []valobj.Id{notebookId})
	})
}

func TestSweepNotebookSkipsArtifactsWhileNotebookExists(t *testing.T) {
	Convey("artifact purge is skipped while the notebook row still exists", t, func() {
		artifacts := &fakeArtifactRepo{}
		objects := &fakeObjectStore{}
		sweeper := newTestSweeper(&fakeSourceRepo{}, &fakeNotebookRepo{exists: true}, artifacts, &fakeSourceDocRepo{}, objects)

		notebookId := valobj.NewId()
		key, err := valobj.NewStoreKey(testBucket, "artifact/x/y.pptx", false)
		So(err, ShouldBeNil)

		refs := []ArtifactRef{{ArtifactId: valobj.NewId(), Keys: []string{key.Encode()}}}

		So(sweeper.SweepNotebook(t.Context(), notebookId, nil, refs), ShouldBeNil)
		So(objects.calls, ShouldEqual, 0)
		So(artifacts.deletedByNb, ShouldBeEmpty)
	})
}

func TestSweepNotebookIgnoresBadArtifactKey(t *testing.T) {
	Convey("a bad artifact object key is skipped", t, func() {
		artifacts := &fakeArtifactRepo{}
		objects := &fakeObjectStore{}
		sweeper := newTestSweeper(&fakeSourceRepo{}, &fakeNotebookRepo{}, artifacts, &fakeSourceDocRepo{}, objects)

		goodKey, err := valobj.NewStoreKey(testBucket, "artifact/x/y.pptx", false)
		So(err, ShouldBeNil)

		refs := []ArtifactRef{{
			ArtifactId: valobj.NewId(),
			Keys:       []string{"!!!not-a-storekey", goodKey.Encode()},
		}}

		So(sweeper.SweepNotebook(t.Context(), valobj.NewId(), nil, refs), ShouldBeNil)
		So(objects.deletedKeys, ShouldResemble, []valobj.StoreKey{goodKey})
	})
}

func TestSweepNotebookPayloadCarriesArtifacts(t *testing.T) {
	Convey("Sweep routes notebook payload artifacts to the artifact purge", t, func() {
		artifacts := &fakeArtifactRepo{}
		objects := &fakeObjectStore{}
		sweeper := newTestSweeper(&fakeSourceRepo{}, &fakeNotebookRepo{}, artifacts, &fakeSourceDocRepo{}, objects)

		notebookId, artifactId := valobj.NewId(), valobj.NewId()
		key, err := valobj.NewStoreKey(testBucket, "artifact/x/y.pptx", false)
		So(err, ShouldBeNil)

		So(sweeper.Sweep(t.Context(), &Payload{
			Kind:       KindNotebook,
			NotebookId: notebookId,
			Artifacts:  []ArtifactRef{{ArtifactId: artifactId, Keys: []string{key.Encode()}}},
		}), ShouldBeNil)

		So(objects.deletedKeys, ShouldResemble, []valobj.StoreKey{key})
		So(artifacts.deletedByNb, ShouldResemble, []valobj.Id{notebookId})
	})
}

func TestSweepArtifactDeletesObjectOnceRowIsGone(t *testing.T) {
	Convey("single artifact fallback deletes the object once the row is gone", t, func() {
		objects := &fakeObjectStore{}
		artifacts := &fakeArtifactRepo{}
		sweeper := newTestSweeper(&fakeSourceRepo{}, &fakeNotebookRepo{}, artifacts, &fakeSourceDocRepo{}, objects)

		key, err := valobj.NewStoreKey(testBucket, "artifact/x/y.pptx", false)
		So(err, ShouldBeNil)

		So(sweeper.SweepArtifact(t.Context(), valobj.NewId(), valobj.NewId(), []string{key.Encode()}), ShouldBeNil)
		So(objects.deletedKeys, ShouldResemble, []valobj.StoreKey{key})
	})
}

func TestSweepArtifactSkipsWhileRowExists(t *testing.T) {
	Convey("single artifact fallback skips while the row still exists", t, func() {
		objects := &fakeObjectStore{}
		artifacts := &fakeArtifactRepo{artifact: &artifactentity.Artifact{}}
		sweeper := newTestSweeper(&fakeSourceRepo{}, &fakeNotebookRepo{}, artifacts, &fakeSourceDocRepo{}, objects)

		key, err := valobj.NewStoreKey(testBucket, "artifact/x/y.pptx", false)
		So(err, ShouldBeNil)

		So(sweeper.SweepArtifact(t.Context(), valobj.NewId(), valobj.NewId(), []string{key.Encode()}), ShouldBeNil)
		So(objects.calls, ShouldEqual, 0)
	})
}

func TestSweepDeletedNotebookSourcesOnlyTouchesSources(t *testing.T) {
	Convey("notebook delete handler purges only source resources", t, func() {
		objects := &fakeObjectStore{}
		docs := &fakeSourceDocRepo{}
		artifacts := &fakeArtifactRepo{}
		sweeper := newTestSweeper(&fakeSourceRepo{}, &fakeNotebookRepo{}, artifacts, docs, objects)

		notebookId := valobj.NewId()
		key, err := valobj.NewStoreKey(testBucket, "file/nb/source.txt", false)
		So(err, ShouldBeNil)
		source := &sourceentity.Source{NotebookId: notebookId, ParsedContentKey: key}

		So(sweeper.SweepDeletedNotebookSources(t.Context(), notebookId, []*sourceentity.Source{source}), ShouldBeNil)
		So(docs.calls, ShouldEqual, 1)
		So(objects.deletedKeys, ShouldResemble, []valobj.StoreKey{key})
		So(artifacts.deletedByNb, ShouldBeEmpty)
	})
}

func TestSweepDeletedNotebookArtifactsOnlyTouchesArtifacts(t *testing.T) {
	Convey("notebook delete handler purges only artifact resources", t, func() {
		objects := &fakeObjectStore{}
		docs := &fakeSourceDocRepo{}
		sources := &fakeSourceRepo{}
		artifacts := &fakeArtifactRepo{}
		sweeper := newTestSweeper(sources, &fakeNotebookRepo{}, artifacts, docs, objects)

		notebookId := valobj.NewId()
		artifact := &artifactentity.Artifact{
			Result:     []byte(`{"store_key":{"bucket":"` + testBucket + `","key":"artifact/nb/art.pptx","is_public":false}}`),
			ResultKind: artifactentity.ResultKindStorage,
		}
		artifact.NotebookId = notebookId

		So(sweeper.SweepDeletedNotebookArtifacts(t.Context(), notebookId, []*artifactentity.Artifact{artifact}), ShouldBeNil)
		So(docs.calls, ShouldEqual, 0)
		So(objects.deletedKeys, ShouldHaveLength, 1)
		So(artifacts.deletedByNb, ShouldResemble, []valobj.Id{notebookId})
		So(sources.deletedByNb, ShouldBeEmpty)
	})
}
