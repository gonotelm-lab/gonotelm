package schema

type TransientProviderLoginInfo struct {
	State               string `msgpack:"state"`
	CodeVerifier        string `msgpack:"code_verifier"`
	CodeChallenge       string `msgpack:"code_challenge"`
	CodeChallengeMethod string `msgpack:"code_challenge_method"`
	ReturnTo            string `msgpack:"return_to"`
	ProviderType        string `msgpack:"provider_type"`
}
