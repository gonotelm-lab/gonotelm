package repository

import (
	"context"
	"fmt"
	"os"
	"testing"

	artifactentity "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"
	artifacterrors "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/errors"
	artifactrepo "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/repository"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/database/postgres"
	"github.com/gonotelm-lab/gonotelm/pkg/sql/testsuite"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

var (
	testDB          *gorm.DB
	artifactStore   *postgres.ArtifactStoreImpl
	artifactRepo    artifactrepo.Repository
	artifactTestCtx = context.Background()
)

func TestMain(m *testing.M) {
	const migrationFilePath = "../../../migration/db/postgres18.sql"

	testDatabase, err := testsuite.NewTestGormDBFromEnv("pgsql")
	if err != nil {
		panic(err)
	}
	if err := testDatabase.Setup(migrationFilePath); err != nil {
		panic(err)
	}
	testDB = testDatabase.GetDB()
	artifactStore = postgres.NewArtifactStoreImpl(testDB)
	artifactRepo = NewArtifactRepository(artifactStore)

	m.Run()

	if testDatabase != nil {
		if err := testDatabase.Cleanup(); err != nil {
			fmt.Fprintf(os.Stderr, "cleanup test database failed: %v\n", err)
		}
	}
}

func TestArtifactRepository_SaveAndFindById(t *testing.T) {
	payload := &artifactentity.MindmapPayload{NotebookId: uuid.NewV7(), SourceIds: nil}
	a, err := artifactentity.NewArtifact(uuid.NewV7(), "u-repo-1", artifactentity.KindMindmap, payload)
	require.NoError(t, err)
	a.BindFlowTaskId(uuid.NewV7().String())
	require.NoError(t, artifactRepo.Save(artifactTestCtx, a))

	got, err := artifactRepo.FindById(artifactTestCtx, a.Id)
	require.NoError(t, err)
	assert.Equal(t, a.Id, got.Id)
	assert.Equal(t, artifactentity.StatusPending, got.Status)
	assert.Equal(t, a.FlowTaskId, got.FlowTaskId)
	assert.Equal(t, artifactentity.KindMindmap, got.Kind)
}

func TestArtifactRepository_FindById_NotFound(t *testing.T) {
	_, err := artifactRepo.FindById(artifactTestCtx, uuid.NewV7())
	require.Error(t, err)
	assert.ErrorIs(t, err, artifacterrors.ErrArtifactNotFound)
}

func TestArtifactRepository_ListByNotebookId(t *testing.T) {
	notebookId := uuid.NewV7()
	a, err := artifactentity.NewArtifact(notebookId, "u-repo-list", artifactentity.KindReport, &artifactentity.ReportPayload{NotebookId: notebookId})
	require.NoError(t, err)
	a.BindFlowTaskId(uuid.NewV7().String())
	require.NoError(t, artifactRepo.Save(artifactTestCtx, a))

	got, err := artifactRepo.ListByNotebookId(artifactTestCtx, notebookId, 50, 0)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(got), 1)
	found := false
	for _, row := range got {
		if row.Id == a.Id {
			found = true
			break
		}
	}
	assert.True(t, found, "saved artifact not present in ListByNotebookId result")
}

func TestArtifactRepository_ListByStatus_And_UpdateStatus(t *testing.T) {
	a, err := artifactentity.NewArtifact(uuid.NewV7(), "u-repo-2", artifactentity.KindReport, &artifactentity.ReportPayload{})
	require.NoError(t, err)
	a.BindFlowTaskId(uuid.NewV7().String())
	require.NoError(t, artifactRepo.Save(artifactTestCtx, a))

	got, err := artifactRepo.ListByStatus(artifactTestCtx, []artifactentity.Status{artifactentity.StatusPending}, 50)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(got), 1)

	require.NoError(t, artifactRepo.UpdateStatus(artifactTestCtx, a.Id, artifactentity.StatusRunning, nil, "", ""))
	got2, err := artifactRepo.FindById(artifactTestCtx, a.Id)
	require.NoError(t, err)
	assert.Equal(t, artifactentity.StatusRunning, got2.Status)
}

func TestArtifactRepository_UpdateStatus_PopulatesResult(t *testing.T) {
	a, err := artifactentity.NewArtifact(uuid.NewV7(), "u-repo-3", artifactentity.KindMindmap, &artifactentity.MindmapPayload{})
	require.NoError(t, err)
	a.BindFlowTaskId(uuid.NewV7().String())
	require.NoError(t, artifactRepo.Save(artifactTestCtx, a))

	result := []byte(`{"hello":"world"}`)
	require.NoError(t, artifactRepo.UpdateStatus(artifactTestCtx, a.Id, artifactentity.StatusCompleted, result, artifactentity.ResultKindInline, "title-1"))

	got, err := artifactRepo.FindById(artifactTestCtx, a.Id)
	require.NoError(t, err)
	assert.Equal(t, artifactentity.StatusCompleted, got.Status)
	assert.Equal(t, "title-1", got.Title)
	assert.Equal(t, result, got.Result)
	assert.Equal(t, artifactentity.ResultKindInline, got.ResultKind)
}

func TestArtifactRepository_DeleteById(t *testing.T) {
	a, err := artifactentity.NewArtifact(uuid.NewV7(), "u-repo-del", artifactentity.KindReport, &artifactentity.ReportPayload{})
	require.NoError(t, err)
	a.BindFlowTaskId(uuid.NewV7().String())
	require.NoError(t, artifactRepo.Save(artifactTestCtx, a))

	require.NoError(t, artifactRepo.DeleteById(artifactTestCtx, a.Id))
	_, err = artifactRepo.FindById(artifactTestCtx, a.Id)
	assert.ErrorIs(t, err, artifacterrors.ErrArtifactNotFound)
}
