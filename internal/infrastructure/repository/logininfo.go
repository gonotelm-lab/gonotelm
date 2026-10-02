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
	for k, v := range providersConfig {
		switch entity.ProviderType(k) {
		case entity.ProviderTypeGithub:
			provider, err := newGithubLoginProvider(v)
			if err != nil {
				slog.ErrorContext(ctx, fmt.Sprintf("init login provider %s failed", k), slog.Any("err", err))
				continue
			}
			r.providers[entity.ProviderTypeGithub] = provider
		default:
			return errors.Errorf("invalid provider type: %s", k)
		}
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

type githubLoginProvider struct {
	config idp.Config
	impl   idp.Provider
}

func newGithubLoginProvider(config idp.Config) (*githubLoginProvider, error) {
	client := httpclient.NewBuilder(nil).Build()
	impl, err := github.New(config, github.WithHTTPClient(client))
	if err != nil {
		return nil, err
	}
	return &githubLoginProvider{
		config: config,
		impl:   impl,
	}, nil
}

var _ entity.Provider = &githubLoginProvider{}

func (p *githubLoginProvider) Type() entity.ProviderType {
	return entity.ProviderTypeGithub
}

func (p *githubLoginProvider) LoginInfo(ctx context.Context, request *entity.LoginInfoRequest) (*entity.ProviderLoginInfo, error) {
	url, state, err := p.impl.AuthURL()
	if err != nil {
		return nil, errors.Wrapf(domainerr.ErrIDPError, "failed to get auth url, err=%s", err.Error())
	}

	return &entity.ProviderLoginInfo{
		URL:                 url.String(),
		ClientId:            p.config.ClientID,
		State:               state.State,
		CodeVerifier:        state.CodeVerifier,
		CodeChallenge:       state.CodeChallenge,
		CodeChallengeMethod: state.CodeChallengeMethod,
		ProviderType:        entity.ProviderTypeGithub,
		ReturnTo:            request.ReturnTo,
	}, nil
}

func (p *githubLoginProvider) GetUserInfo(ctx context.Context, code string, state *entity.TransientProviderLoginInfo) (*entity.ProviderUserInfo, error) {
	userInfo, err := p.impl.GetUserInfo(ctx, code, state.CodeVerifier)
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
