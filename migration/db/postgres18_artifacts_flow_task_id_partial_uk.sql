-- Allow multiple artifacts without a flow task (empty flow_task_id), e.g. kind=note.
DROP INDEX IF EXISTS uk_artifacts_flow_task_id;
CREATE UNIQUE INDEX IF NOT EXISTS uk_artifacts_flow_task_id
  ON artifacts(flow_task_id)
  WHERE flow_task_id <> '';
