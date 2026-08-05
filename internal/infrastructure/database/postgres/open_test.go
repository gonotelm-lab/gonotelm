package postgres

import (
	"context"
	"os"
	"strconv"
	"testing"

	"github.com/gonotelm-lab/gonotelm/internal/conf/shared"
	"github.com/gonotelm-lab/gonotelm/pkg/sql/testsuite"
)

func TestOpen(t *testing.T) {
	port, err := strconv.Atoi(os.Getenv(testsuite.EnvGonotelmDBPort))
	if err != nil {
		t.Fatalf("invalid %s: %v", testsuite.EnvGonotelmDBPort, err)
	}

	dao, err := Open(shared.DatabaseConfig{
		Host:     os.Getenv(testsuite.EnvGonotelmDBHost),
		Port:     port,
		User:     os.Getenv(testsuite.EnvGonotelmDBUser),
		Password: os.Getenv(testsuite.EnvGonotelmDBPass),
		DBName:   os.Getenv(testsuite.EnvGonotelmDBDBName),
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
