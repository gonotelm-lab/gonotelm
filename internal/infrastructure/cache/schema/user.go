package schema

type User struct {
	Id              string `json:"id"                msgpack:"id"`
	Email           string `json:"email"             msgpack:"email"`
	EmailVerifiedAt int64  `json:"email_verified_at" msgpack:"email_verified_at"`
	Nickname        string `json:"nickname"          msgpack:"nickname"`
	Status          string `json:"status"            msgpack:"status"`
	Avatar          string `json:"avatar"            msgpack:"avatar"`
	CreatedAt       int64  `json:"created_at"        msgpack:"created_at"`
	UpdatedAt       int64  `json:"updated_at"        msgpack:"updated_at"`
}
