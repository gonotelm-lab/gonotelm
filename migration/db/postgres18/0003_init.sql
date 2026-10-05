-- +goose Up
CREATE TABLE IF NOT EXISTS initjob_runs (
  id          UUID        PRIMARY KEY DEFAULT uuidv7(),
  status      VARCHAR(16) NOT NULL, -- running | succeeded | failed | aborted
  run_mode    VARCHAR(16) NOT NULL, -- strict | loose
  pod         VARCHAR(255) NOT NULL DEFAULT '',
  exit_code   INT          NOT NULL DEFAULT 0,
  error       TEXT         NOT NULL DEFAULT '',
  started_at  BIGINT       NOT NULL DEFAULT 0,
  finished_at BIGINT       NOT NULL DEFAULT 0,
  created_at  BIGINT       NOT NULL DEFAULT 0,
  updated_at  BIGINT       NOT NULL DEFAULT 0
);

COMMENT ON TABLE initjob_runs IS 'initjob process run table';
COMMENT ON COLUMN initjob_runs.status IS 'running | succeeded | failed | aborted';
COMMENT ON COLUMN initjob_runs.run_mode IS 'failure policy: strict | loose';
COMMENT ON COLUMN initjob_runs.pod IS 'hostname of the process, pod name on k8s';
COMMENT ON COLUMN initjob_runs.exit_code IS 'process exit code, 0 on success';
COMMENT ON COLUMN initjob_runs.error IS 'error summary of the round';
COMMENT ON COLUMN initjob_runs.started_at IS 'run start time (unix ms)';
COMMENT ON COLUMN initjob_runs.finished_at IS 'run finish time (unix ms), 0 while running';

CREATE INDEX IF NOT EXISTS idx_initjob_runs_started_at ON initjob_runs (started_at DESC);
CREATE INDEX IF NOT EXISTS idx_initjob_runs_status ON initjob_runs (status);

CREATE TABLE IF NOT EXISTS initjob_tasks (
  task_id     VARCHAR(32) PRIMARY KEY, -- YYYYMMDD + 4-digit daily sequence
  description VARCHAR(512) NOT NULL DEFAULT '', -- what the task does, from the code
  run_id      UUID        NOT NULL,    -- run that last actually executed the task
  status      VARCHAR(16) NOT NULL,    -- running | succeeded | failed | aborted
  attempt     INT         NOT NULL DEFAULT 0,
  error       TEXT        NOT NULL DEFAULT '',
  started_at  BIGINT      NOT NULL DEFAULT 0,
  finished_at BIGINT      NOT NULL DEFAULT 0,
  created_at  BIGINT      NOT NULL DEFAULT 0,
  updated_at  BIGINT      NOT NULL DEFAULT 0,
  CONSTRAINT chk_initjob_tasks_task_id_format CHECK (task_id ~ '^[0-9]{12}$')
);

COMMENT ON TABLE initjob_tasks IS 'initjob task table, holding the latest result only';
COMMENT ON COLUMN initjob_tasks.task_id IS 'task id, also the idempotency key: YYYYMMDD + 4-digit sequence';
COMMENT ON COLUMN initjob_tasks.description IS 'what the task does, refreshed from the code on every execution';
COMMENT ON COLUMN initjob_tasks.run_id IS 'run that last actually executed this task; skipped tasks keep their previous run_id';
COMMENT ON COLUMN initjob_tasks.status IS 'running | succeeded | failed | aborted';
COMMENT ON COLUMN initjob_tasks.attempt IS 'number of times the task was started';
COMMENT ON COLUMN initjob_tasks.error IS 'error summary of the latest attempt';
COMMENT ON COLUMN initjob_tasks.started_at IS 'latest attempt start time (unix ms)';
COMMENT ON COLUMN initjob_tasks.finished_at IS 'latest attempt finish time (unix ms), 0 while running';

CREATE INDEX IF NOT EXISTS idx_initjob_tasks_status ON initjob_tasks (status);

-- +goose Down
DROP TABLE IF EXISTS initjob_tasks;
DROP TABLE IF EXISTS initjob_runs;
