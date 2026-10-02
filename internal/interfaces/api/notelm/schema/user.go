package schema

type MeResponse struct {
	UserId    string `json:"user_id"`
	Nickname  string `json:"nickname"`
	AvatarUrl string `json:"avatar_url"`
}
