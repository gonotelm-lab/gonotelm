package schema

// OAuthState OAuth 握手状态写入 Redis 的内容
type OAuthState struct {
	State               string `msgpack:"state"`
	Nonce               string `msgpack:"nonce"`
	CodeVerifier        string `msgpack:"code_verifier"`
	CodeChallenge       string `msgpack:"code_challenge"`
	CodeChallengeMethod string `msgpack:"code_challenge_method"`
	ReturnTo            string `msgpack:"return_to"`
	ProviderType        string `msgpack:"provider_type"`
	Device              string `msgpack:"device"`
	Mode                string `msgpack:"mode"`
	SessionId           string `msgpack:"session_id"`
}
