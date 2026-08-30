package postgres

import (
	"context"
	"os"
	"strconv"
	"testing"

	pkgsql "github.com/gonotelm-lab/gonotelm/pkg/sql"
	sqltestsuite "github.com/gonotelm-lab/gonotelm/pkg/testsuite/sql"
)

func TestOpen(t *testing.T) {
	port, err := strconv.Atoi(os.Getenv(sqltestsuite.EnvGonotelmTestDBPort))
	if err != nil {
		t.Fatalf("invalid %s: %v", sqltestsuite.EnvGonotelmTestDBPort, err)
	}

	dao, err := Open(&pkgsql.Config{
		Host:     os.Getenv(sqltestsuite.EnvGonotelmTestDBHost),
		Port:     port,
		User:     os.Getenv(sqltestsuite.EnvGonotelmTestDBUser),
		Password: os.Getenv(sqltestsuite.EnvGonotelmTestDBPass),
		DBName:   "postgres",
	})
	if err != nil {
		t.Fatalf("Open() failed: %v", err)
	}
	defer func() {
		if err := dao.Close(context.Background()); err != nil {
			t.Errorf("Close() failed: %v", err)
		}
	}()

	if dao.NotebookStore == nil || dao.SourceStore == nil || dao.ChatStore == nil ||
		dao.ChatMessageStore == nil || dao.ArtifactStore == nil || dao.WorkerCheckpointStore == nil {
		t.Errorf("Open() returned dao with missing stores: %+v", dao)
	}
}
