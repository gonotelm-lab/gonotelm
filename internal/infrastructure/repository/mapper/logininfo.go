package mapper

import (
	"github.com/gonotelm-lab/gonotelm/internal/domain/identity/entity"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/cache/schema"
)

func LoginInfoToSchema(l *entity.TransientProviderLoginInfo) *schema.TransientProviderLoginInfo {
	return &schema.TransientProviderLoginInfo{
		State:               l.State,
		Nonce:               l.Nonce,
		CodeVerifier:        l.CodeVerifier,
		CodeChallenge:       l.CodeChallenge,
		CodeChallengeMethod: l.CodeChallengeMethod,
		ReturnTo:            l.ReturnTo,
		ProviderType:        string(l.ProviderType),
	}
}

func LoginInfoFromSchema(s *schema.TransientProviderLoginInfo) *entity.TransientProviderLoginInfo {
	return &entity.TransientProviderLoginInfo{
		State:               s.State,
		Nonce:               s.Nonce,
		CodeVerifier:        s.CodeVerifier,
		CodeChallenge:       s.CodeChallenge,
		CodeChallengeMethod: s.CodeChallengeMethod,
		ReturnTo:            s.ReturnTo,
		ProviderType:        entity.ProviderType(s.ProviderType),
	}
}
