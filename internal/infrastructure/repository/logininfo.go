package repository

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/gonotelm-lab/gonotelm/internal/domain/identity/entity"
	domainerr "github.com/gonotelm-lab/gonotelm/internal/domain/identity/errors"
	"github.com/gonotelm-lab/gonotelm/internal/domain/identity/repository"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/cache"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/repository/mapper"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/httpclient"
	"github.com/gonotelm-lab/gonotelm/pkg/idp"
	"github.com/gonotelm-lab/gonotelm/pkg/idp/github"
	"github.com/gonotelm-lab/gonotelm/pkg/idp/google"
)

type LoginInfoRepositoryImpl struct {
	mu        sync.RWMutex
	providers map[entity.ProviderType]entity.Provider
	cache     cache.TransientProviderLoginInfoCache
}

var _ repository.LoginInfoRepository = &LoginInfoRepositoryImpl{}

func NewLoginInfoRepository(ctx context.Context,
	cache cache.TransientProviderLoginInfoCache,
	providersConfig map[string]idp.Config,
) (repository.LoginInfoRepository, error) {
	impl := &LoginInfoRepositoryImpl{
		cache:     cache,
		providers: make(map[entity.ProviderType]entity.Provider),
	}

	if err := impl.initProviders(ctx, providersConfig); err != nil {
		return nil, errors.WithMessage(err, "failed to init login providers")
	}

	return impl, nil
}

func (r *LoginInfoRepositoryImpl) initProviders(ctx context.Context, providersConfig map[string]idp.Config) error {
	client := httpclient.NewBuilder(nil).Build()

	for k, v := range providersConfig {
		providerType := entity.ProviderType(k)

		// 未配置凭据的提供商直接跳过，避免前端拿到一个无法完成登录的入口
		if v.ClientID == "" || v.ClientSecret == "" {
			slog.WarnContext(ctx, "skip login provider without credentials", slog.String("provider_type", k))
			continue
		}

		var (
			impl idp.Provider
			err  error
		)

		switch providerType {
		case entity.ProviderTypeGithub:
			impl, err = github.New(v, github.WithHTTPClient(client))
		case entity.ProviderTypeGoogle:
			impl, err = google.New(v, google.WithHTTPClient(client))
		default:
			return errors.Errorf("invalid provider type: %s", k)
		}
		if err != nil {
			slog.ErrorContext(ctx, fmt.Sprintf("init login provider %s failed", k), slog.Any("err", err))
			continue
		}

		r.providers[providerType] = newOAuthLoginProvider(providerType, v, impl)
	}

	return nil
}

// 保存临时登录信息 一般保存的内容为 state code_verifier
func (r *LoginInfoRepositoryImpl) SaveTransientLoginInfo(ctx context.Context, loginInfo *entity.TransientProviderLoginInfo) error {
	err := r.cache.Set(ctx, loginInfo.State, mapper.LoginInfoToSchema(loginInfo))
	if err != nil {
		return errors.WithMessage(err, "failed to save transient login info")
	}

	return nil
}

func (r *LoginInfoRepositoryImpl) GetTransientLoginInfo(ctx context.Context, state string) (*entity.TransientProviderLoginInfo, error) {
	sch, err := r.cache.Get(ctx, state)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to get transient login info")
	}
	loginInfo := mapper.LoginInfoFromSchema(sch)

	return loginInfo, nil
}

func (r *LoginInfoRepositoryImpl) DeleteTransientLoginInfo(ctx context.Context, state string) error {
	err := r.cache.Delete(ctx, state)
	if err != nil {
		return errors.WithMessage(err, "failed to delete transient login info")
	}

	return nil
}

func (r *LoginInfoRepositoryImpl) GetProvider(ctx context.Context, providerType entity.ProviderType) (entity.Provider, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	provider, ok := r.providers[providerType]
	if ok {
		return provider, nil
	}

	return nil, domainerr.ErrProviderNotFound
}

func (r *LoginInfoRepositoryImpl) AvailableProviders() []entity.Provider {
	r.mu.RLock()
	defer r.mu.RUnlock()

	providers := make([]entity.Provider, 0, len(r.providers))
	for _, provider := range r.providers {
		providers = append(providers, provider)
	}

	return providers
}

// oauthLoginProvider adapts an idp.Provider to the identity domain Provider
// interface for all OAuth2/OIDC providers.
type oauthLoginProvider struct {
	providerType entity.ProviderType
	config       idp.Config
	impl         idp.Provider
}

func newOAuthLoginProvider(providerType entity.ProviderType, config idp.Config, impl idp.Provider) *oauthLoginProvider {
	return &oauthLoginProvider{
		providerType: providerType,
		config:       config,
		impl:         impl,
	}
}

var _ entity.Provider = &oauthLoginProvider{}

func (p *oauthLoginProvider) Type() entity.ProviderType {
	return p.providerType
}

func (p *oauthLoginProvider) LoginInfo(ctx context.Context, request *entity.LoginInfoRequest) (*entity.ProviderLoginInfo, error) {
	url, state, err := p.impl.AuthURL()
	if err != nil {
		return nil, errors.Wrapf(domainerr.ErrIDPError, "failed to get auth url, err=%s", err.Error())
	}

	return &entity.ProviderLoginInfo{
		URL:                 url.String(),
		ClientId:            p.config.ClientID,
		State:               state.State,
		Nonce:               state.Nonce,
		CodeVerifier:        state.CodeVerifier,
		CodeChallenge:       state.CodeChallenge,
		CodeChallengeMethod: state.CodeChallengeMethod,
		ProviderType:        p.providerType,
		ReturnTo:            request.ReturnTo,
		Device:              request.Device,
	}, nil
}

func (p *oauthLoginProvider) GetUserInfo(ctx context.Context, code string, state *entity.TransientProviderLoginInfo) (*entity.ProviderUserInfo, error) {
	userInfo, err := p.impl.GetUserInfo(ctx, code, state.CodeVerifier, state.Nonce)
	if err != nil {
		return nil, errors.Wrapf(domainerr.ErrIDPExchangeError, "get user info: %s", err.Error())
	}

	return &entity.ProviderUserInfo{
		Issuer:    userInfo.Issuer,
		Subject:   userInfo.Subject,
		Name:      userInfo.Name,
		Raw:       userInfo.Raw,
		AvatarURL: userInfo.AvatarURL,
	}, nil
}
