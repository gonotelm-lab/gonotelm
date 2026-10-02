package service

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gabriel-vasile/mimetype"
	"github.com/gonotelm-lab/gonotelm/internal/core/adapter"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	"github.com/gonotelm-lab/gonotelm/internal/domain/identity/entity"
	identityerrors "github.com/gonotelm-lab/gonotelm/internal/domain/identity/errors"
	"github.com/gonotelm-lab/gonotelm/internal/domain/identity/repository"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/httpclient"
)

const (
	maxAvatarBytes     = 5 << 20 // 头像下载上限 5MB
	avatarFetchTimeout = 10 * time.Second
)

type UserService struct {
	userRepo    repository.UserRepository
	distLock    adapter.DistributedLock
	keyFactory  adapter.StoreKeyFactory
	objectStore adapter.ObjectStore
	httpClient  *http.Client
}

func NewUserService(
	userRepo repository.UserRepository,
	distLock adapter.DistributedLock,
	keyFactory adapter.StoreKeyFactory,
	objectStore adapter.ObjectStore,
) *UserService {
	return &UserService{
		userRepo:    userRepo,
		distLock:    distLock,
		keyFactory:  keyFactory,
		objectStore: objectStore,
		httpClient:  httpclient.NewBuilder(nil).WithTimeout(avatarFetchTimeout).Build(),
	}
}

type RegisterParams struct {
	Provider       entity.ProviderType
	Subject        string
	Nickname       string
	OuterAvatarURL string // 三方用户的 avatar url
}

func registerLockKey(provider entity.ProviderType, subject string) string {
	return fmt.Sprintf("gonotelm:identity:user:register:%s:%s", provider, subject)
}

func (s *UserService) Get(
	ctx context.Context,
	provider entity.ProviderType,
	subject string,
) (*entity.User, error) {
	user, err := s.userRepo.GetByProviderSub(ctx, provider, subject)
	if err != nil {
		return nil, errors.WithMessagef(err, "get user failed, provider=%s", provider)
	}

	return user, nil
}

// Register 注册新用户
func (s *UserService) Register(ctx context.Context, params RegisterParams) (*entity.User, error) {
	lockKey := registerLockKey(params.Provider, params.Subject)
	if err := s.distLock.Lock(ctx, lockKey); err != nil {
		return nil, errors.WithMessagef(err, "acquire register lock failed, provider=%s", params.Provider)
	}
	defer func() {
		if err := s.distLock.Unlock(ctx, lockKey); err != nil {
			slog.ErrorContext(ctx, "release register lock failed",
				slog.String("lock_key", lockKey),
				slog.Any("err", err),
			)
		}
	}()

	existing, err := s.userRepo.GetByProviderSub(ctx, params.Provider, params.Subject)
	if err == nil {
		return existing, nil
	}
	if !errors.Is(err, identityerrors.ErrUserNotFound) {
		return nil, errors.WithMessagef(err, "get user failed, provider=%s", params.Provider)
	}

	user := entity.NewUser(params.Nickname, params.Provider, params.Subject)
	avatarKey, hasAvatar := s.attachAvatar(ctx, user, params.OuterAvatarURL)

	if err := s.userRepo.Save(ctx, user); err != nil {
		if hasAvatar {
			if delErr := s.objectStore.DeleteObject(ctx, avatarKey); delErr != nil {
				slog.ErrorContext(ctx, "rollback uploaded avatar failed",
					slog.String("user_id", user.Id.String()),
					slog.String("avatar", avatarKey.String()),
					slog.Any("err", delErr),
				)
			}
		}

		return nil, errors.WithMessage(err, "save user failed")
	}

	return user, nil
}

func (s *UserService) attachAvatar(
	ctx context.Context,
	user *entity.User,
	outerAvatarURL string,
) (valobj.StoreKey, bool) {
	if outerAvatarURL == "" {
		return valobj.StoreKey{}, false
	}

	avatarKey, err := user.NewAvatarKey(s.keyFactory)
	if err != nil {
		slog.WarnContext(ctx, "create avatar store key failed, register without avatar",
			slog.String("avatar_url", outerAvatarURL),
			slog.Any("err", err),
		)
		return valobj.StoreKey{}, false
	}

	body, contentType, err := s.fetchAvatar(ctx, outerAvatarURL)
	if err != nil {
		slog.WarnContext(ctx, "fetch outer avatar failed, register without avatar",
			slog.String("avatar_url", outerAvatarURL),
			slog.Any("err", err),
		)
		return valobj.StoreKey{}, false
	}

	if err := s.objectStore.Upload(ctx, avatarKey, body, contentType); err != nil {
		slog.WarnContext(ctx, "upload avatar failed, register without avatar",
			slog.String("avatar_url", outerAvatarURL),
			slog.Any("err", err),
		)
		return valobj.StoreKey{}, false
	}

	user.SetAvatar(avatarKey)

	return avatarKey, true
}

func (s *UserService) fetchAvatar(ctx context.Context, rawURL string) ([]byte, string, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return nil, "", errors.ErrParams.Msgf("invalid avatar url: %s", rawURL)
	}

	ctx, cancel := context.WithTimeout(ctx, avatarFetchTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, "", errors.WithMessage(err, "build avatar request failed")
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, "", errors.WithMessage(err, "request avatar failed")
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, "", errors.ErrInner.Msgf("avatar response status %d", resp.StatusCode)
	}

	// 多读一字节用于判断是否超限
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxAvatarBytes+1))
	if err != nil {
		return nil, "", errors.WithMessage(err, "read avatar body failed")
	}
	if len(body) == 0 {
		return nil, "", errors.ErrInner.Msg("avatar body is empty")
	}
	if len(body) > maxAvatarBytes {
		return nil, "", errors.ErrInner.Msgf("avatar body exceeds %d bytes", maxAvatarBytes)
	}

	contentType := resolveAvatarContentType(resp.Header.Get("Content-Type"), body)
	if contentType == "" {
		return nil, "", errors.ErrInner.Msgf("avatar is not an image, content_type=%q", resp.Header.Get("Content-Type"))
	}

	return body, contentType, nil
}

// resolveAvatarContentType 优先信响应头，响应头缺失或不是 image/* 时按内容嗅探。
func resolveAvatarContentType(header string, body []byte) string {
	if mediaType, _, err := mime.ParseMediaType(header); err == nil && strings.HasPrefix(mediaType, "image/") {
		return mediaType
	}

	detected := mimetype.Detect(body).String()
	if strings.HasPrefix(detected, "image/") {
		return detected
	}

	return ""
}
