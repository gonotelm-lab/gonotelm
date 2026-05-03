package postgres

import (
	"fmt"
	"os"
	"testing"

	"github.com/gonotelm-lab/gonotelm/pkg/sql/testsuite"

	"gorm.io/gorm"
)

var (
	testDB               *gorm.DB
	testNotebookStore    *NotebookStoreImpl
	testSourceStore      *SourceStoreImpl
	testChatMessageStore *ChatMessageStoreImpl
)

func TestMain(m *testing.M) {
	const migrationFilePath = "../../../../../migration/db/postgres18.sql"

	var testDatabase *testsuite.TestDb
	var err error
	testDatabase, err = testsuite.NewTestGormDBFromEnv("pgsql")
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

	m.Run()

	if testDatabase != nil {
		if err := testDatabase.Cleanup(); err != nil {
			fmt.Fprintf(os.Stderr, "cleanup test database failed: %v\n", err)
		}
	}
}
