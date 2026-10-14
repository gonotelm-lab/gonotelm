package mapper

import (
	"github.com/gonotelm-lab/gonotelm/internal/domain/identity/entity"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/cache/schema"
)

func OAuthStateToSchema(l *entity.OAuthState) *schema.OAuthState {
	return &schema.OAuthState{
		State:               l.State,
		Nonce:               l.Nonce,
		CodeVerifier:        l.CodeVerifier,
		CodeChallenge:       l.CodeChallenge,
		CodeChallengeMethod: l.CodeChallengeMethod,
		ReturnTo:            l.ReturnTo,
		ProviderType:        string(l.ProviderType),
		Device:              string(l.Device),
		Mode:                string(l.Mode),
		SessionId:           l.SessionId,
	}
}

func OAuthStateFromSchema(s *schema.OAuthState) *entity.OAuthState {
	return &entity.OAuthState{
		State:               s.State,
		Nonce:               s.Nonce,
		CodeVerifier:        s.CodeVerifier,
		CodeChallenge:       s.CodeChallenge,
		CodeChallengeMethod: s.CodeChallengeMethod,
		ReturnTo:            s.ReturnTo,
		ProviderType:        entity.ProviderType(s.ProviderType),
		Device:              entity.DeviceType(s.Device),
		Mode:                entity.ProviderLoginMode(s.Mode),
		SessionId:           s.SessionId,
	}
}
