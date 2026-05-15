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
  display_name VARCHAR(255) NOT NULL DEFAULT '',
  content BYTEA,
  owner_id VARCHAR(255) NOT NULL DEFAULT '',
  updated_at BIGINT NOT NULL DEFAULT 0
);

CREATE INDEX idx_notebook_id ON sources (notebook_id);

COMMENT ON TABLE sources IS 'sources table';
COMMENT ON COLUMN sources.id IS 'source ID, primary key';
COMMENT ON COLUMN sources.notebook_id IS 'notebook id';
COMMENT ON COLUMN sources.kind IS 'source kind';
COMMENT ON COLUMN sources.status IS 'source processing state';
COMMENT ON COLUMN sources.display_name IS 'source display name';
COMMENT ON COLUMN sources.content IS 'source content payload (file source stores format in content)';
COMMENT ON COLUMN sources.owner_id IS 'source owner id';
COMMENT ON COLUMN sources.updated_at IS 'source updated time (unix ms)';

ALTER TABLE sources ADD COLUMN parsed_content BYTEA;
COMMENT ON COLUMN sources.parsed_content IS 'source parsed content';

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
