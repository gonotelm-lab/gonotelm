package database

import (
	"context"
	stderrors "errors"
	"testing"

	"github.com/gonotelm-lab/gonotelm/pkg/misc"
	. "github.com/smartystreets/goconvey/convey"
)

// the embedded interfaces keep these fakes from implementing every store method
type fakeUserStore struct{ UserStore }

type fakeUserIdentityStore struct{ UserIdentityStore }

type fakeTransactor struct{ Transactor }

func TestNewDaoWiresStores(t *testing.T) {
	Convey("NewDao wires every store it is given", t, func() {
		closer := misc.CloserFunc(func(context.Context) error { return nil })
		userStore := &fakeUserStore{}
		identityStore := &fakeUserIdentityStore{}
		transactor := &fakeTransactor{}

		dao := NewDao(
			closer,
			nil, // NotebookStore
			nil, // NotebookMaintenanceStore
			nil, // NotebookMemberStore
			nil, // UserNotebookPinStore
			nil, // SourceStore
			nil, // ChatStore
			nil, // ChatMessageStore
			nil, // ArtifactStore
			nil, // ArtifactStylePreviewStore
			nil, // WorkerCheckpointStore
			nil, // InitJobStore
			userStore,
			identityStore,
			transactor,
		)

		So(dao, ShouldNotBeNil)
		So(dao.Closer, ShouldEqual, closer)
		So(dao.UserStore, ShouldEqual, userStore)
		So(dao.UserIdentityStore, ShouldEqual, identityStore)
		So(dao.Transactor, ShouldEqual, transactor)
		So(dao.NotebookStore, ShouldBeNil)
		So(dao.NotebookMaintenanceStore, ShouldBeNil)
	})
}

func TestDaoClose(t *testing.T) {
	Convey("Dao.Close delegates to the closer", t, func() {
		closed := false
		closer := misc.CloserFunc(func(context.Context) error {
			closed = true
			return nil
		})
		dao := newDaoForCloseTest(closer)

		So(dao.Close(t.Context()), ShouldBeNil)
		So(closed, ShouldBeTrue)
	})

	Convey("Dao.Close returns the closer error", t, func() {
		want := stderrors.New("close failed")
		closer := misc.CloserFunc(func(context.Context) error { return want })
		dao := newDaoForCloseTest(closer)

		err := dao.Close(t.Context())
		So(stderrors.Is(err, want), ShouldBeTrue)
	})
}

func newDaoForCloseTest(closer misc.Closer) *Dao {
	return NewDao(
		closer,
		nil, // NotebookStore
		nil, // NotebookMaintenanceStore
		nil, // NotebookMemberStore
		nil, // UserNotebookPinStore
		nil, // SourceStore
		nil, // ChatStore
		nil, // ChatMessageStore
		nil, // ArtifactStore
		nil, // ArtifactStylePreviewStore
		nil, // WorkerCheckpointStore
		nil, // InitJobStore
		nil, // UserStore
		nil, // UserIdentityStore
		nil, // Transactor
	)
}
