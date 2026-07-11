CREATE TABLE IF NOT EXISTS artifacts (
  id            UUID        PRIMARY KEY,
  notebook_id   UUID        NOT NULL,
  user_id       VARCHAR(128) NOT NULL,
  kind          VARCHAR(32) NOT NULL,
  status        VARCHAR(16) NOT NULL,
  flow_task_id  VARCHAR(64) NOT NULL,
  title         VARCHAR(256) NULL,
  result        BYTEA       NULL,
  result_kind   VARCHAR(16) NULL,
  payload       JSONB       NOT NULL,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_artifacts_notebook ON artifacts(notebook_id);
CREATE INDEX IF NOT EXISTS idx_artifacts_status   ON artifacts(status);
ALTER TABLE artifacts ADD CONSTRAINT uq_artifacts_flow_task_id UNIQUE (flow_task_id);