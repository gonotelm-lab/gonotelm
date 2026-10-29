-- +goose Up
CREATE TABLE user_notebook_pins (
    user_id      BYTEA NOT NULL,
    notebook_id  UUID  NOT NULL,
    pinned_at    BIGINT NOT NULL,
    PRIMARY KEY (user_id, notebook_id),
    CONSTRAINT chk_user_notebook_pins_user_id_len CHECK (octet_length(user_id) = 16)
);

CREATE INDEX idx_user_notebook_pins_user_pinned_at ON user_notebook_pins (user_id, pinned_at DESC);
CREATE INDEX idx_user_notebook_pins_notebook_id ON user_notebook_pins (notebook_id);

COMMENT ON TABLE user_notebook_pins IS 'per-user notebook pins';
COMMENT ON COLUMN user_notebook_pins.user_id IS 'user id (ulid 16 bytes)';
COMMENT ON COLUMN user_notebook_pins.pinned_at IS 'unix ms';

-- +goose Down
DROP TABLE IF EXISTS user_notebook_pins;
