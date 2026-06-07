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
  msg_type SMALLINT NOT NULL DEFAULT 0,
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
COMMENT ON COLUMN chat_messages.msg_type IS 'message type: 0-normal, 1-system';
COMMENT ON COLUMN chat_messages.content IS 'message content';
COMMENT ON COLUMN chat_messages.seq_no IS 'message sequence number(unix nano)';
COMMENT ON COLUMN chat_messages.extra IS 'message extra information';

CREATE TABLE artifact_tasks (
  id UUID PRIMARY KEY DEFAULT uuidv7(),
  notebook_id UUID NOT NULL DEFAULT uuidv7(),
  kind VARCHAR(16) NOT NULL DEFAULT '',
  status VARCHAR(16) NOT NULL DEFAULT '',
  title VARCHAR(128) NOT NULL DEFAULT '',
  result BYTEA,
  result_kind VARCHAR(16) NOT NULL DEFAULT '',
  user_id VARCHAR(255) NOT NULL DEFAULT '',
  run_id VARCHAR(36) NOT NULL DEFAULT '',
  lock_no INTEGER NOT NULL DEFAULT 0,
  payload BYTEA,
  created_at BIGINT NOT NULL DEFAULT 0,
  updated_at BIGINT NOT NULL DEFAULT 0,
  expired_at BIGINT NOT NULL DEFAULT 0
);

CREATE INDEX idx_artifact_tasks_notebook_id ON artifact_tasks (notebook_id);
CREATE INDEX idx_artifact_tasks_status_created_at ON artifact_tasks (status, created_at);
CREATE INDEX idx_artifact_tasks_expired_at ON artifact_tasks (expired_at);

COMMENT ON TABLE artifact_tasks IS 'studio artifact tasks table';
COMMENT ON COLUMN artifact_tasks.id IS 'artifact task id, primary key';
COMMENT ON COLUMN artifact_tasks.notebook_id IS 'associated notebook id';
COMMENT ON COLUMN artifact_tasks.kind IS 'artifact task kind';
COMMENT ON COLUMN artifact_tasks.status IS 'artifact task processing state';
COMMENT ON COLUMN artifact_tasks.title IS 'artifact task title';
COMMENT ON COLUMN artifact_tasks.result IS 'artifact task result';
COMMENT ON COLUMN artifact_tasks.result_kind IS 'artifact task result kind';
COMMENT ON COLUMN artifact_tasks.user_id IS 'artifact task user id';
COMMENT ON COLUMN artifact_tasks.updated_at IS 'artifact task updated time (unix ms)';
COMMENT ON COLUMN artifact_tasks.run_id IS 'artifact task run id';
COMMENT ON COLUMN artifact_tasks.lock_no IS 'artifact task lock number for locking';
COMMENT ON COLUMN artifact_tasks.payload IS 'artifact task payload';
COMMENT ON COLUMN artifact_tasks.created_at IS 'artifact task created time (unix ms)';
COMMENT ON COLUMN artifact_tasks.expired_at IS 'task expired time (unix ms)';
