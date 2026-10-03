package idp

const (
	CodeChallengeMethodS256 = "S256"
)

type State struct {
	State               string
	Nonce               string
	CodeVerifier        string
	CodeChallenge       string
	CodeChallengeMethod string // always will be CodeChallengeMethodS256
}
