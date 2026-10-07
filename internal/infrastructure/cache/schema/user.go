package schema

type User struct {
	Id        string `json:"id"         msgpack:"id"`
	Email     string `json:"email"      msgpack:"email"`
	Nickname  string `json:"nickname"   msgpack:"nickname"`
	Status    string `json:"status"     msgpack:"status"`
	Avatar    string `json:"avatar"     msgpack:"avatar"`
	Provider  string `json:"provider"   msgpack:"provider"`
	Sub       string `json:"sub"        msgpack:"sub"`
	CreatedAt int64  `json:"created_at" msgpack:"created_at"`
	UpdatedAt int64  `json:"updated_at" msgpack:"updated_at"`
}
