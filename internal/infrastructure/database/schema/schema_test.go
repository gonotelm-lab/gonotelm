package schema

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestTableNames(t *testing.T) {
	Convey("every schema maps to its migrated table name", t, func() {
		tables := map[string]string{
			Notebook{}.TableName():             "notebooks",
			Source{}.TableName():               "sources",
			Chat{}.TableName():                 "chats",
			Artifact{}.TableName():             "artifacts",
			ArtifactStylePreview{}.TableName(): "artifact_style_previews",
			ArtifactTask{}.TableName():         "artifact_tasks",
			WorkerCheckpoint{}.TableName():     "worker_artifact_checkpoints",
			InitJobRun{}.TableName():           "initjob_runs",
			InitJobTask{}.TableName():          "initjob_tasks",
			User{}.TableName():                 "users_v2",
			UserIdentity{}.TableName():         "user_identities_v2",
		}

		seen := map[string]bool{}
		for actual, want := range tables {
			So(actual, ShouldEqual, want)
			So(seen[actual], ShouldBeFalse)
			seen[actual] = true
		}
	})
}
