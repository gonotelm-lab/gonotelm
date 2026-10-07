package repository

import (
	"context"
	"errors"
	"testing"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	artifactentity "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"
	artifacterrors "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/errors"
	cacheschema "github.com/gonotelm-lab/gonotelm/internal/infrastructure/cache/schema"
	dbschema "github.com/gonotelm-lab/gonotelm/internal/infrastructure/database/schema"
	pkgerrors "github.com/gonotelm-lab/gonotelm/pkg/errors"
	. "github.com/smartystreets/goconvey/convey"
)

// fakeStylePreviewStore 记录每次查询，用来断言缓存命中时没有回源。
type fakeStylePreviewStore struct {
	rows map[string]*dbschema.ArtifactStylePreview

	getCalls  []string
	listCalls [][]string
	upserts   []*dbschema.ArtifactStylePreview
	deletes   []string
}

var _ interface {
	Upsert(ctx context.Context, preview *dbschema.ArtifactStylePreview) error
	GetByIdentifier(ctx context.Context, identifier string) (*dbschema.ArtifactStylePreview, error)
	ListByIdentifiers(ctx context.Context, identifiers []string) ([]*dbschema.ArtifactStylePreview, error)
	DeleteByIdentifier(ctx context.Context, identifier string) error
} = &fakeStylePreviewStore{}

func newFakeStylePreviewStore(rows ...*dbschema.ArtifactStylePreview) *fakeStylePreviewStore {
	store := &fakeStylePreviewStore{rows: map[string]*dbschema.ArtifactStylePreview{}}
	for _, row := range rows {
		store.rows[row.Identifier] = row
	}

	return store
}

func (s *fakeStylePreviewStore) Upsert(_ context.Context, preview *dbschema.ArtifactStylePreview) error {
	s.upserts = append(s.upserts, preview)
	s.rows[preview.Identifier] = preview

	return nil
}

func (s *fakeStylePreviewStore) GetByIdentifier(
	_ context.Context, identifier string,
) (*dbschema.ArtifactStylePreview, error) {
	s.getCalls = append(s.getCalls, identifier)

	row, ok := s.rows[identifier]
	if !ok {
		return nil, pkgerrors.ErrNoRecord
	}

	return row, nil
}

func (s *fakeStylePreviewStore) ListByIdentifiers(
	_ context.Context, identifiers []string,
) ([]*dbschema.ArtifactStylePreview, error) {
	// 拷贝一份，避免调用方后续复用切片影响断言。
	s.listCalls = append(s.listCalls, append([]string(nil), identifiers...))

	out := make([]*dbschema.ArtifactStylePreview, 0, len(identifiers))
	for _, identifier := range identifiers {
		if row, ok := s.rows[identifier]; ok {
			out = append(out, row)
		}
	}

	return out, nil
}

func (s *fakeStylePreviewStore) DeleteByIdentifier(_ context.Context, identifier string) error {
	s.deletes = append(s.deletes, identifier)
	delete(s.rows, identifier)

	return nil
}

// fakeStylePreviewCache 是内存版 StylePreviewCache，可注入故障。
type fakeStylePreviewCache struct {
	items  map[string]*cacheschema.StylePreview
	getErr error
	setErr error
}

var _ interface {
	Set(ctx context.Context, identifier string, preview *cacheschema.StylePreview) error
	SetMulti(ctx context.Context, previews []*cacheschema.StylePreview) error
	Get(ctx context.Context, identifier string) (*cacheschema.StylePreview, error)
	GetMulti(ctx context.Context, identifiers []string) (map[string]*cacheschema.StylePreview, error)
	Delete(ctx context.Context, identifier string) error
} = &fakeStylePreviewCache{}

func newFakeStylePreviewCache() *fakeStylePreviewCache {
	return &fakeStylePreviewCache{items: map[string]*cacheschema.StylePreview{}}
}

func (c *fakeStylePreviewCache) Set(
	_ context.Context, identifier string, preview *cacheschema.StylePreview,
) error {
	if c.setErr != nil {
		return c.setErr
	}
	c.items[identifier] = preview

	return nil
}

func (c *fakeStylePreviewCache) SetMulti(_ context.Context, previews []*cacheschema.StylePreview) error {
	if c.setErr != nil {
		return c.setErr
	}
	for _, preview := range previews {
		c.items[preview.Identifier] = preview
	}

	return nil
}

func (c *fakeStylePreviewCache) Get(
	_ context.Context, identifier string,
) (*cacheschema.StylePreview, error) {
	if c.getErr != nil {
		return nil, c.getErr
	}

	return c.items[identifier], nil
}

func (c *fakeStylePreviewCache) GetMulti(
	_ context.Context, identifiers []string,
) (map[string]*cacheschema.StylePreview, error) {
	if c.getErr != nil {
		return nil, c.getErr
	}

	out := make(map[string]*cacheschema.StylePreview, len(identifiers))
	for _, identifier := range identifiers {
		if preview, ok := c.items[identifier]; ok {
			out[identifier] = preview
		}
	}

	return out, nil
}

func (c *fakeStylePreviewCache) Delete(_ context.Context, identifier string) error {
	delete(c.items, identifier)

	return nil
}

func stylePreviewRow(t *testing.T, identifier string) *dbschema.ArtifactStylePreview {
	t.Helper()

	key, err := valobj.NewStoreKey("gonotelm", "artifact-preview/"+identifier, true)
	So(err, ShouldBeNil)

	return &dbschema.ArtifactStylePreview{
		Identifier:       identifier,
		StoreKey:         key.Encode(),
		Md5sum:           []byte("sum"),
		OriginalFilename: identifier + ".webp",
		CreatedAt:        1700000000000,
		UpdatedAt:        1700000000001,
	}
}

func TestStylePreviewRepositoryFindByIdentifierCacheAside(t *testing.T) {
	Convey("FindByIdentifier 回源后回填缓存，第二次读不再回源", t, func() {
		ctx := context.Background()
		store := newFakeStylePreviewStore(stylePreviewRow(t, "slides.style.default"))
		previewCache := newFakeStylePreviewCache()
		repo := NewStylePreviewRepository(store, previewCache)

		got, err := repo.FindByIdentifier(ctx, artifactentity.StylePreviewSlidesDefault)
		So(err, ShouldBeNil)
		So(got.Identifier, ShouldEqual, artifactentity.StylePreviewSlidesDefault)
		So(got.StoreKey.Valid(), ShouldBeTrue)
		So(store.getCalls, ShouldHaveLength, 1)
		So(previewCache.items, ShouldContainKey, "slides.style.default")

		got, err = repo.FindByIdentifier(ctx, artifactentity.StylePreviewSlidesDefault)
		So(err, ShouldBeNil)
		So(got.Identifier, ShouldEqual, artifactentity.StylePreviewSlidesDefault)
		So(store.getCalls, ShouldHaveLength, 1) // 缓存命中时不应回源
	})
}

func TestStylePreviewRepositoryFindByIdentifierNotFound(t *testing.T) {
	Convey("FindByIdentifier 库里也没有时返回 ErrStylePreviewNotFound 且不写缓存", t, func() {
		store := newFakeStylePreviewStore()
		previewCache := newFakeStylePreviewCache()
		repo := NewStylePreviewRepository(store, previewCache)

		got, err := repo.FindByIdentifier(context.Background(), artifactentity.StylePreviewSlidesDefault)
		So(got, ShouldBeNil)
		So(err, ShouldNotBeNil)
		So(errors.Is(err, artifacterrors.ErrStylePreviewNotFound), ShouldBeTrue)
		So(previewCache.items, ShouldBeEmpty)
	})
}

func TestStylePreviewRepositoryFindByIdentifierCacheFailureFallsBack(t *testing.T) {
	Convey("缓存读失败时降级回源，不影响结果", t, func() {
		ctx := context.Background()
		store := newFakeStylePreviewStore(stylePreviewRow(t, "slides.style.default"))
		previewCache := newFakeStylePreviewCache()
		previewCache.getErr = pkgerrors.ErrCache
		repo := NewStylePreviewRepository(store, previewCache)

		got, err := repo.FindByIdentifier(ctx, artifactentity.StylePreviewSlidesDefault)
		So(err, ShouldBeNil)
		So(got, ShouldNotBeNil)
		So(store.getCalls, ShouldHaveLength, 1)
	})
}

func TestStylePreviewRepositoryListByIdentifiersPartialCacheHit(t *testing.T) {
	Convey("ListByIdentifiers 只回源未命中的 identifier 并回填", t, func() {
		ctx := context.Background()
		store := newFakeStylePreviewStore(
			stylePreviewRow(t, "slides.style.default"),
			stylePreviewRow(t, "slides.style.cute"),
		)
		previewCache := newFakeStylePreviewCache()
		previewCache.items["slides.style.default"] = &cacheschema.StylePreview{
			Identifier: "slides.style.default",
			StoreKey:   store.rows["slides.style.default"].StoreKey,
			CreateTime: 1700000000000,
			UpdateTime: 1700000000001,
		}
		repo := NewStylePreviewRepository(store, previewCache)

		rows, err := repo.ListByIdentifiers(ctx, []artifactentity.StylePreviewIdentifier{
			artifactentity.StylePreviewSlidesCute,
			artifactentity.StylePreviewSlidesDefault,
		})
		So(err, ShouldBeNil)
		So(rows, ShouldHaveLength, 2)
		So(rows[0].Identifier, ShouldEqual, artifactentity.StylePreviewSlidesCute) // 返回顺序与入参一致
		So(rows[1].Identifier, ShouldEqual, artifactentity.StylePreviewSlidesDefault)

		So(store.listCalls, ShouldHaveLength, 1)
		So(store.listCalls[0], ShouldResemble, []string{"slides.style.cute"})
		So(previewCache.items, ShouldContainKey, "slides.style.cute")
	})
}

func TestStylePreviewRepositoryListByIdentifiersAllCached(t *testing.T) {
	Convey("ListByIdentifiers 全部命中时不回源", t, func() {
		store := newFakeStylePreviewStore(stylePreviewRow(t, "slides.style.default"))
		previewCache := newFakeStylePreviewCache()
		previewCache.items["slides.style.default"] = &cacheschema.StylePreview{
			Identifier: "slides.style.default",
			StoreKey:   store.rows["slides.style.default"].StoreKey,
		}
		repo := NewStylePreviewRepository(store, previewCache)

		rows, err := repo.ListByIdentifiers(context.Background(), []artifactentity.StylePreviewIdentifier{
			artifactentity.StylePreviewSlidesDefault,
		})
		So(err, ShouldBeNil)
		So(rows, ShouldHaveLength, 1)
		So(store.listCalls, ShouldBeEmpty)
	})
}

func TestStylePreviewRepositorySaveRefreshesCache(t *testing.T) {
	Convey("Save 先落库再写穿缓存", t, func() {
		store := newFakeStylePreviewStore()
		previewCache := newFakeStylePreviewCache()
		repo := NewStylePreviewRepository(store, previewCache)

		key, err := valobj.NewStoreKey("gonotelm", "artifact-preview/slides/default.webp", true)
		So(err, ShouldBeNil)
		p := artifactentity.NewStylePreview(artifactentity.StylePreviewSlidesDefault, key)
		p.UpdateAsset(key, []byte("sum"), "default.webp")

		So(repo.Save(context.Background(), p), ShouldBeNil)
		So(store.upserts, ShouldHaveLength, 1)
		So(previewCache.items, ShouldContainKey, "slides.style.default")
	})
}

func TestStylePreviewRepositorySaveCacheFailureStillSucceeds(t *testing.T) {
	Convey("Save 时缓存失败不影响已落库的结果", t, func() {
		store := newFakeStylePreviewStore()
		previewCache := newFakeStylePreviewCache()
		previewCache.setErr = pkgerrors.ErrCache
		repo := NewStylePreviewRepository(store, previewCache)

		key, err := valobj.NewStoreKey("gonotelm", "artifact-preview/slides/default.webp", true)
		So(err, ShouldBeNil)
		p := artifactentity.NewStylePreview(artifactentity.StylePreviewSlidesDefault, key)

		So(repo.Save(context.Background(), p), ShouldBeNil)
		So(store.upserts, ShouldHaveLength, 1)
	})
}

func TestStylePreviewRepositoryDeleteInvalidatesCache(t *testing.T) {
	Convey("DeleteByIdentifier 删库后让缓存失效", t, func() {
		store := newFakeStylePreviewStore(stylePreviewRow(t, "slides.style.default"))
		previewCache := newFakeStylePreviewCache()
		previewCache.items["slides.style.default"] = &cacheschema.StylePreview{Identifier: "slides.style.default"}
		repo := NewStylePreviewRepository(store, previewCache)

		So(repo.DeleteByIdentifier(context.Background(), artifactentity.StylePreviewSlidesDefault), ShouldBeNil)
		So(store.deletes, ShouldResemble, []string{"slides.style.default"})
		So(previewCache.items, ShouldNotContainKey, "slides.style.default")
	})
}

func TestStylePreviewRepositoryWithoutCache(t *testing.T) {
	Convey("未配置缓存时退化为直连 store", t, func() {
		ctx := context.Background()
		store := newFakeStylePreviewStore(stylePreviewRow(t, "slides.style.default"))
		repo := NewStylePreviewRepository(store, nil)

		got, err := repo.FindByIdentifier(ctx, artifactentity.StylePreviewSlidesDefault)
		So(err, ShouldBeNil)
		So(got, ShouldNotBeNil)

		rows, err := repo.ListByIdentifiers(ctx, []artifactentity.StylePreviewIdentifier{
			artifactentity.StylePreviewSlidesDefault,
		})
		So(err, ShouldBeNil)
		So(rows, ShouldHaveLength, 1)

		key, err := valobj.NewStoreKey("gonotelm", "artifact-preview/slides/default.webp", true)
		So(err, ShouldBeNil)
		So(repo.Save(ctx, artifactentity.NewStylePreview(artifactentity.StylePreviewSlidesDefault, key)), ShouldBeNil)
		So(repo.DeleteByIdentifier(ctx, artifactentity.StylePreviewSlidesDefault), ShouldBeNil)
	})
}
