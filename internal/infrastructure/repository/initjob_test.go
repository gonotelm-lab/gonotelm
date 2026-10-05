package repository

import (
	"testing"

	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/database/schema"
	"github.com/gonotelm-lab/gonotelm/pkg/initjob"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The stored vocabulary must not drift from the framework's: the row-level
// store queries by these exact strings, so a mismatch would silently break
// "skip what already succeeded".
func TestInitJobStoreStatus_CoversFrameworkVocabulary(t *testing.T) {
	cases := []struct {
		in   initjob.Status
		want string
	}{
		{initjob.StatusRunning, schema.InitJobStatusRunning},
		{initjob.StatusSucceeded, schema.InitJobStatusSucceeded},
		{initjob.StatusFailed, schema.InitJobStatusFailed},
		{initjob.StatusAborted, schema.InitJobStatusAborted},
	}

	for _, tc := range cases {
		got, err := initJobStoreStatus(tc.in)
		require.NoError(t, err, "status %q must be mapped", tc.in)
		assert.Equal(t, tc.want, got)
		assert.Equal(t, string(tc.in), tc.want, "framework and stored vocabularies drifted")
	}

	_, err := initJobStoreStatus(initjob.Status("unknown"))
	assert.Error(t, err)
}

// pkg/uuid renders without dashes, so the run id handed back by StartRun must
// parse in that form; otherwise FinishRun could never close a run.
func TestParseInitJobRunID_RoundTripsDashlessUUID(t *testing.T) {
	id := uuid.NewV7()
	raw := id.String()
	require.Len(t, raw, 32)
	assert.NotContains(t, raw, "-")

	got, err := parseInitJobRunID(raw)
	require.NoError(t, err)
	assert.Equal(t, id, got)

	_, err = parseInitJobRunID("not-a-uuid")
	assert.Error(t, err)
}
