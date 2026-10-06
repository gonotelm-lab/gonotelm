package postgres

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/gonotelm-lab/gonotelm/migration/db/postgres18"
	sqltestsuite "github.com/gonotelm-lab/gonotelm/pkg/testsuite/sql"

	"gorm.io/gorm"
)

var (
	testDB                        *gorm.DB
	testNotebookStore             *NotebookStoreImpl
	testSourceStore               *SourceStoreImpl
	testChatMessageStore          *ChatMessageStoreImpl
	testArtifactStore             *ArtifactStoreImpl
	testArtifactStylePreviewStore *ArtifactStylePreviewStoreImpl
	testWorkerCheckpointStore     *WorkerCheckpointStoreImpl
	testInitJobStore              *InitJobStoreImpl
	testUserStore                 *UserStoreImpl
)

func TestMain(m *testing.M) {
	testDatabase, err := sqltestsuite.NewTestGormDBFromEnv("pgsql")
	if err != nil {
		panic(err)
	}
	// 与 cmd/migrate 共用 goose 迁移文件。
	if err := testDatabase.Setup(context.Background(), postgres18.Migrate); err != nil {
		panic(err)
	}
	testDB = testDatabase.GetDB()
	testNotebookStore = NewNotebookStoreImpl(testDB)
	testSourceStore = NewSourceStoreImpl(testDB)
	testChatMessageStore = NewChatMessageStoreImpl(testDB)
	testArtifactStore = NewArtifactStoreImpl(testDB)
	testArtifactStylePreviewStore = NewArtifactStylePreviewStoreImpl(testDB)
	testWorkerCheckpointStore = NewWorkerCheckpointStoreImpl(testDB)
	testInitJobStore = NewInitJobStoreImpl(testDB)
	testUserStore = NewUserStoreImpl(testDB)

	m.Run()

	if err := testDatabase.Cleanup(); err != nil {
		fmt.Fprintf(os.Stderr, "cleanup test database failed: %v\n", err)
	}
}
