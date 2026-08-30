package postgres

import (
	"fmt"
	"os"
	"testing"

	sqltestsuite "github.com/gonotelm-lab/gonotelm/pkg/testsuite/sql"

	"gorm.io/gorm"
)

var (
	testDB                    *gorm.DB
	testNotebookStore         *NotebookStoreImpl
	testSourceStore           *SourceStoreImpl
	testChatMessageStore      *ChatMessageStoreImpl
	testArtifactStore         *ArtifactStoreImpl
	testWorkerCheckpointStore *WorkerCheckpointStoreImpl
)

func TestMain(m *testing.M) {
	const migrationFilePath = "../../../../migration/db/postgres18/0001.sql"

	testDatabase, err := sqltestsuite.NewTestGormDBFromEnv("pgsql")
	if err != nil {
		panic(err)
	}
	if err := testDatabase.Setup(migrationFilePath); err != nil {
		panic(err)
	}
	testDB = testDatabase.GetDB()
	testNotebookStore = NewNotebookStoreImpl(testDB)
	testSourceStore = NewSourceStoreImpl(testDB)
	testChatMessageStore = NewChatMessageStoreImpl(testDB)
	testArtifactStore = NewArtifactStoreImpl(testDB)
	testWorkerCheckpointStore = NewWorkerCheckpointStoreImpl(testDB)

	m.Run()

	if err := testDatabase.Cleanup(); err != nil {
		fmt.Fprintf(os.Stderr, "cleanup test database failed: %v\n", err)
	}
}
