package schema

// PasswordResetState 待确认的找回密码请求，确认后重设密码
type PasswordResetState struct {
	Email         string `json:"email"           msgpack:"email"`
	LinkTokenHash string `json:"link_token_hash" msgpack:"link_token_hash"`
	CreatedAt     int64  `json:"created_at"      msgpack:"created_at"`
}
