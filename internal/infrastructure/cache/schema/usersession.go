package schema

type UserSession struct {
	UserId    string `msgpack:"user_id"`
	CreatedAt int64  `msgpack:"created_at"`
	ExpireAt  int64  `msgpack:"expire_at"`
	Device    string `msgpack:"device"`
}
