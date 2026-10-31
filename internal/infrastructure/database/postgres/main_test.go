package postgres

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/gonotelm-lab/gonotelm/migration/db/postgres18"
	sqltestsuite "github.com/gonotelm-lab/gonotelm/pkg/testsuite/sql"

	"gorm.io/gorm"
)

var (
	testDB                        *gorm.DB
	testNotebookStore             *NotebookStoreImpl
	testNotebookMemberStore       *NotebookMemberStoreImpl
	testUserNotebookPinStore      *UserNotebookPinStoreImpl
	testFavouriteStore            *FavouriteStoreImpl
	testFavouriteNotebookStore    *FavouriteNotebookStoreImpl
	testSourceStore               *SourceStoreImpl
	testChatStore                 *ChatStoreImpl
	testChatMessageStore          *ChatMessageStoreImpl
	testArtifactStore             *ArtifactStoreImpl
	testArtifactStylePreviewStore *ArtifactStylePreviewStoreImpl
	testWorkerCheckpointStore     *WorkerCheckpointStoreImpl
	testInitJobStore              *InitJobStoreImpl
	testUserStore                 *UserStoreImpl
	testUserIdentityStore         *UserIdentityStoreImpl
	testTransactor                *TransactorImpl
)

func nowMilli() int64 { return time.Now().UnixMilli() }

func TestMain(m *testing.M) {
	testDatabase, err := sqltestsuite.NewTestGormDBFromEnv("pgsql")
	if err != nil {
		panic(err)
	}
	// 与 cmd/migrate 共用 goose 迁移文件
	if err := testDatabase.Setup(context.Background(), postgres18.Migrate); err != nil {
		panic(err)
	}
	testDB = testDatabase.GetDB()
	testNotebookStore = NewNotebookStoreImpl(testDB)
	testNotebookMemberStore = NewNotebookMemberStoreImpl(testDB)
	testUserNotebookPinStore = NewUserNotebookPinStoreImpl(testDB)
	testFavouriteStore = NewFavouriteStoreImpl(testDB)
	testFavouriteNotebookStore = NewFavouriteNotebookStoreImpl(testDB)
	testSourceStore = NewSourceStoreImpl(testDB)
	testChatStore = NewChatStoreImpl(testDB)
	testChatMessageStore = NewChatMessageStoreImpl(testDB)
	testArtifactStore = NewArtifactStoreImpl(testDB)
	testArtifactStylePreviewStore = NewArtifactStylePreviewStoreImpl(testDB)
	testWorkerCheckpointStore = NewWorkerCheckpointStoreImpl(testDB)
	testInitJobStore = NewInitJobStoreImpl(testDB)
	testUserStore = NewUserStoreImpl(testDB)
	testUserIdentityStore = NewUserIdentityStoreImpl(testDB)
	testTransactor = NewTransactorImpl(testDB)

	m.Run()

	if err := testDatabase.Cleanup(); err != nil {
		fmt.Fprintf(os.Stderr, "cleanup test database failed: %v\n", err)
	}
}
