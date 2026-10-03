package schema

type MeResponse struct {
	UserId      string `json:"user_id"`
	Nickname    string `json:"nickname"`
	AvatarUrl   string `json:"avatar_url"`
	UpdatedAt   int64  `json:"updated_at"`   // unix ms
	CreatedAt   int64  `json:"created_at"`   // unix ms
	LoginSource string `json:"login_source"` // provider
}
