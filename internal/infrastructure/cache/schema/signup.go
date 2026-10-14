package schema

// SignupState 待验证的邮箱注册，账号在验证通过后才建立
type SignupState struct {
	Email        string `json:"email"          msgpack:"email"`
	PasswordHash string `json:"password_hash"  msgpack:"password_hash"`
	TicketHash   string `json:"ticket_hash"    msgpack:"ticket_hash"`
	CodeHash     string `json:"code_hash"      msgpack:"code_hash"`
	CodeExpireAt int64  `json:"code_expire_at" msgpack:"code_expire_at"`
	CreatedAt    int64  `json:"created_at"     msgpack:"created_at"`
}
