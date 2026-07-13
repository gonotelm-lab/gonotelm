CREATE DATABASE gonotelm;

\c gonotelm;

CREATE TABLE notebooks (
  id UUID PRIMARY KEY DEFAULT uuidv7(),
  name VARCHAR(128) NOT NULL DEFAULT '',
  description VARCHAR(1024) NOT NULL DEFAULT '',
  owner_id VARCHAR(255) NOT NULL DEFAULT '',
  updated_at BIGINT NOT NULL DEFAULT 0
);

COMMENT ON TABLE notebooks IS 'notebooks table';
COMMENT ON COLUMN notebooks.id IS 'notebook id, primary key';
COMMENT ON COLUMN notebooks.name IS 'notebook name';
COMMENT ON COLUMN notebooks.description IS 'notebook description';
COMMENT ON COLUMN notebooks.owner_id IS 'notebook owner id';
COMMENT ON COLUMN notebooks.updated_at IS 'notebook updated time (unix ms)';

CREATE TABLE sources (
  id UUID PRIMARY KEY DEFAULT uuidv7(),
  notebook_id UUID NOT NULL DEFAULT uuidv7(),
  kind VARCHAR(16) NOT NULL DEFAULT '',
  status VARCHAR(16) NOT NULL DEFAULT '',
  title VARCHAR(255) NOT NULL DEFAULT '',
  content BYTEA,
  owner_id VARCHAR(255) NOT NULL DEFAULT '',
  parsed_content_key VARCHAR(255) NOT NULL DEFAULT '',
  updated_at BIGINT NOT NULL DEFAULT 0
);

CREATE INDEX idx_notebook_id ON sources (notebook_id);

COMMENT ON TABLE sources IS 'sources table';
COMMENT ON COLUMN sources.id IS 'source ID, primary key';
COMMENT ON COLUMN sources.notebook_id IS 'notebook id';
COMMENT ON COLUMN sources.kind IS 'source kind';
COMMENT ON COLUMN sources.status IS 'source processing state';
COMMENT ON COLUMN sources.title IS 'source title';
COMMENT ON COLUMN sources.content IS 'source content payload (file source stores format in content)';
COMMENT ON COLUMN sources.owner_id IS 'source owner id';
COMMENT ON COLUMN sources.parsed_content_key IS 'source parsed content key';
COMMENT ON COLUMN sources.updated_at IS 'source updated time (unix ms)';

ALTER TABLE sources ADD COLUMN abstract TEXT;
COMMENT ON COLUMN sources.abstract IS 'generated abstract for the source';

CREATE TABLE chats (
  id UUID PRIMARY KEY DEFAULT uuidv7(),
  notebook_id UUID NOT NULL DEFAULT uuidv7(),
  owner_id VARCHAR(255) NOT NULL DEFAULT '',
  updated_at BIGINT NOT NULL DEFAULT 0
);

CREATE UNIQUE INDEX uk_notebook_id_owner_id ON chats (notebook_id, owner_id);

COMMENT ON TABLE chats IS 'chats table';
COMMENT ON COLUMN chats.id IS 'chat id, primary key';
COMMENT ON COLUMN chats.notebook_id IS 'associated notebook id';
COMMENT ON COLUMN chats.owner_id IS 'chat owner id';
COMMENT ON COLUMN chats.updated_at IS 'chat updated time (unix ms)';

CREATE TABLE chat_messages (
  id UUID PRIMARY KEY DEFAULT uuidv7(),
  chat_id UUID NOT NULL DEFAULT uuidv7(),
  user_id VARCHAR(255) NOT NULL DEFAULT '',
  msg_role SMALLINT NOT NULL DEFAULT 0,
  content JSONB,
  seq_no BIGINT NOT NULL DEFAULT 0,
  extra JSONB
);

CREATE INDEX idx_chat_id ON chat_messages (chat_id);

COMMENT ON TABLE chat_messages IS 'notebook chat messages history';
COMMENT ON COLUMN chat_messages.id IS 'primary key';
COMMENT ON COLUMN chat_messages.chat_id IS 'chat id, which is a notebook id';
COMMENT ON COLUMN chat_messages.user_id IS 'user id';
COMMENT ON COLUMN chat_messages.msg_role IS 'message role: 0-user, 1-assistant';
COMMENT ON COLUMN chat_messages.content IS 'message content';
COMMENT ON COLUMN chat_messages.seq_no IS 'message sequence number(unix nano)';
COMMENT ON COLUMN chat_messages.extra IS 'message extra information';

CREATE TABLE IF NOT EXISTS artifacts (
  id            UUID        PRIMARY KEY DEFAULT uuidv7(),
  notebook_id   UUID        NOT NULL,
  user_id       VARCHAR(128) NOT NULL,
  kind          VARCHAR(32) NOT NULL,
  status        VARCHAR(16) NOT NULL,
  flow_task_id  VARCHAR(64) NOT NULL,
  title         VARCHAR(256) NULL,
  result        BYTEA       NULL,
  result_kind   VARCHAR(16) NULL,
  payload       JSONB       NOT NULL,
  created_at    BIGINT NOT NULL DEFAULT 0,
  updated_at    BIGINT NOT NULL DEFAULT 0
);

COMMENT ON TABLE artifacts IS 'artifacts table';
COMMENT ON COLUMN artifacts.id IS 'artifact id, primary key';
COMMENT ON COLUMN artifacts.notebook_id IS 'associated notebook id';
COMMENT ON COLUMN artifacts.user_id IS 'artifact user id';
COMMENT ON COLUMN artifacts.kind IS 'artifact kind';
COMMENT ON COLUMN artifacts.status IS 'artifact processing state';
COMMENT ON COLUMN artifacts.flow_task_id IS 'artifact flow task id';
COMMENT ON COLUMN artifacts.title IS 'artifact title';
COMMENT ON COLUMN artifacts.result IS 'artifact result';
COMMENT ON COLUMN artifacts.result_kind IS 'artifact result kind';
COMMENT ON COLUMN artifacts.payload IS 'artifact payload';
COMMENT ON COLUMN artifacts.created_at IS 'artifact created time (unix ms)';
COMMENT ON COLUMN artifacts.updated_at IS 'artifact updated time (unix ms)';

CREATE INDEX IF NOT EXISTS idx_artifacts_notebook ON artifacts(notebook_id);
CREATE INDEX IF NOT EXISTS idx_artifacts_status  ON artifacts(status);
CREATE UNIQUE INDEX IF NOT EXISTS uk_artifacts_flow_task_id ON artifacts(flow_task_id);

CREATE TABLE IF NOT EXISTS worker_artifact_checkpoints (
  artifact_id UUID PRIMARY KEY,
  field1 BYTEA,
  field2 BYTEA,
  field3 BYTEA,
  field4 BYTEA,
  field5 BYTEA,
  field6 BYTEA,
  field7 BYTEA,
  field8 BYTEA,
  created_at BIGINT NOT NULL DEFAULT 0,
  updated_at BIGINT NOT NULL DEFAULT 0
);

COMMENT ON TABLE worker_artifact_checkpoints IS 'worker artifact checkpoint table';
COMMENT ON COLUMN worker_artifact_checkpoints.artifact_id IS 'artifact id, primary key';
COMMENT ON COLUMN worker_artifact_checkpoints.field1 IS 'worker artifact checkpoint field1';
COMMENT ON COLUMN worker_artifact_checkpoints.field2 IS 'worker artifact checkpoint field2';
COMMENT ON COLUMN worker_artifact_checkpoints.field3 IS 'worker artifact checkpoint field3';
COMMENT ON COLUMN worker_artifact_checkpoints.field4 IS 'worker artifact checkpoint field4';
COMMENT ON COLUMN worker_artifact_checkpoints.field5 IS 'worker artifact checkpoint field5';
COMMENT ON COLUMN worker_artifact_checkpoints.field6 IS 'worker artifact checkpoint field6';
COMMENT ON COLUMN worker_artifact_checkpoints.field7 IS 'worker artifact checkpoint field7';
COMMENT ON COLUMN worker_artifact_checkpoints.field8 IS 'worker artifact checkpoint field8';
COMMENT ON COLUMN worker_artifact_checkpoints.created_at IS 'worker artifact checkpoint created time (unix ms)';
COMMENT ON COLUMN worker_artifact_checkpoints.updated_at IS 'worker artifact checkpoint updated time (unix ms)';
