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
	domainerr "github.com/gonotelm-lab/gonotelm/internal/domain/identity/errors"
	"github.com/gonotelm-lab/gonotelm/internal/domain/identity/repository"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/httpclient"
)

const (
	maxAvatarBytes     = 2 << 20
	avatarFetchTimeout = 10 * time.Second
)

type UserService struct {
	userRepo     repository.UserRepository
	identityRepo repository.UserIdentityRepository
	distLock     adapter.DistributedLock
	keyFactory   adapter.StoreKeyFactory
	objectStore  adapter.ObjectStore
	hasher       adapter.PasswordHasher
	httpClient   *http.Client
}

func NewUserService(
	userRepo repository.UserRepository,
	identityRepo repository.UserIdentityRepository,
	distLock adapter.DistributedLock,
	keyFactory adapter.StoreKeyFactory,
	objectStore adapter.ObjectStore,
	hasher adapter.PasswordHasher,
) *UserService {
	return &UserService{
		userRepo:     userRepo,
		identityRepo: identityRepo,
		distLock:     distLock,
		keyFactory:   keyFactory,
		objectStore:  objectStore,
		hasher:       hasher,
		httpClient:   httpclient.NewBuilder(nil).WithTimeout(avatarFetchTimeout).Build(),
	}
}

type ResolveProviderParams struct {
	Provider       entity.ProviderType
	Subject        string
	Email          string
	EmailVerified  bool
	Nickname       string
	OuterAvatarURL string
}

// registerLockKey 首登建号的分布式锁：key 与身份键一致，避免同一 (provider, subject) 并发建号。
func registerLockKey(provider entity.ProviderType, subject string) string {
	return fmt.Sprintf("gonotelm:identity:lock:login:%s:%s", provider, subject)
}

// ResolveOrCreateByProvider 三方登录入口：先按 (provider, subject) 认身份；
// 未命中时只认 provider 已验证的邮箱，命中既有账号则关联，否则建号。
func (s *UserService) ResolveOrCreateByProvider(
	ctx context.Context,
	params ResolveProviderParams,
) (*entity.User, error) {
	identity, err := s.identityRepo.GetByProviderAndSubject(ctx, params.Provider, params.Subject)
	if err == nil {
		return s.loginWithIdentity(ctx, identity)
	}
	if !errors.Is(err, domainerr.ErrIdentityNotFound) {
		return nil, errors.WithMessage(err, "lookup identity failed")
	}

	email := entity.NormalizeEmail(params.Email)
	if !params.EmailVerified || email == "" {
		return nil, domainerr.ErrIdpEmailUnavailable
	}

	lockKey := registerLockKey(params.Provider, params.Subject)
	if err := s.distLock.Lock(ctx, lockKey); err != nil {
		return nil, errors.WithMessagef(err, "acquire login lock failed, provider=%s", params.Provider)
	}
	defer func() {
		if err := s.distLock.Unlock(ctx, lockKey); err != nil {
			slog.ErrorContext(ctx, "release login lock failed",
				slog.String("lock_key", lockKey),
				slog.Any("err", err),
			)
		}
	}()

	// 拿到锁后重查：并发请求可能已经建好了身份
	if identity, err := s.identityRepo.GetByProviderAndSubject(ctx, params.Provider, params.Subject); err == nil {
		return s.loginWithIdentity(ctx, identity)
	}

	user, err := s.userRepo.GetByEmail(ctx, email)
	if err == nil {
		return s.linkIdentity(ctx, user, params, email)
	}
	if !errors.Is(err, domainerr.ErrUserNotFound) {
		return nil, errors.WithMessage(err, "get user by email failed")
	}

	return s.createAccount(ctx, params, email)
}

func (s *UserService) loginWithIdentity(
	ctx context.Context,
	identity *entity.UserIdentity,
) (*entity.User, error) {
	user, err := s.userRepo.GetById(ctx, identity.UserId)
	if err != nil {
		return nil, errors.WithMessage(err, "get user of identity failed")
	}

	identity.TouchLogin(valobj.NewTime())
	if err := s.identityRepo.TouchLogin(ctx, identity); err != nil {
		slog.WarnContext(ctx, "touch identity login time failed", slog.Any("err", err))
	}

	return user, nil
}

// linkIdentity 邮箱验证过的账号已存在 → 直接把身份挂上去。
func (s *UserService) linkIdentity(
	ctx context.Context,
	user *entity.User,
	params ResolveProviderParams,
	email string,
) (*entity.User, error) {
	taken, err := s.providerBoundByOtherSubject(ctx, user.Id, params.Provider, params.Subject)
	if err != nil {
		return nil, err
	}
	if taken {
		// 邮箱已验证说明就是本人，登录放行；只是不再挂第二个同 provider 身份
		slog.WarnContext(ctx, "skip linking second identity of same provider",
			slog.String("user_id", user.Id.String()),
			slog.String("provider", params.Provider.String()),
			slog.String("subject", params.Subject),
		)

		return user, nil
	}

	identity := entity.NewUserIdentity(user.Id, params.Provider, params.Subject, email, true)
	if err := s.identityRepo.Create(ctx, identity); err != nil {
		// 并发兜底：唯一索引冲突说明身份已被建好，重查即可
		if existing, lookupErr := s.identityRepo.GetByProviderAndSubject(
			ctx, params.Provider, params.Subject,
		); lookupErr == nil {
			return s.loginWithIdentity(ctx, existing)
		}

		// 另一个并发请求刚给同一账号挂了同 provider 的别的身份
		if taken, checkErr := s.providerBoundByOtherSubject(
			ctx, user.Id, params.Provider, params.Subject,
		); checkErr == nil && taken {
			return user, nil
		}

		return nil, errors.WithMessage(err, "link identity failed")
	}

	return user, nil
}

func (s *UserService) createAccount(
	ctx context.Context,
	params ResolveProviderParams,
	email string,
) (*entity.User, error) {
	user := entity.NewUser(email, params.Nickname, valobj.NewTime())
	avatarKey, hasAvatar := s.attachAvatar(ctx, user, params.OuterAvatarURL)

	identity := entity.NewUserIdentity(user.Id, params.Provider, params.Subject, email, true)
	if err := s.userRepo.CreateWithFirstIdentity(ctx, user, identity); err != nil {
		if hasAvatar {
			s.deleteAvatar(ctx, user.Id.String(), avatarKey)
		}

		// 并发兜底：另一请求可能已建好身份
		if existing, lookupErr := s.identityRepo.GetByProviderAndSubject(
			ctx, params.Provider, params.Subject,
		); lookupErr == nil {
			return s.loginWithIdentity(ctx, existing)
		}

		return nil, errors.WithMessage(err, "create user failed")
	}

	return user, nil
}

// CreateVerifiedAccount 邮箱被证明后建号；passwordHash 非空时一并写入
func (s *UserService) CreateVerifiedAccount(
	ctx context.Context,
	email, passwordHash string,
) (*entity.User, error) {
	normalized := entity.NormalizeEmail(email)
	user := entity.NewUser(normalized, nicknameFromEmail(normalized), valobj.NewTime())

	var err error
	if passwordHash != "" {
		err = s.userRepo.CreateWithPassword(ctx, user, passwordHash)
	} else {
		err = s.userRepo.Create(ctx, user)
	}
	if err != nil {
		// 同一邮箱只允许一个账号：冲突统一按"邮箱已注册"返回
		if existing, lookupErr := s.userRepo.GetByEmail(ctx, normalized); lookupErr == nil && existing != nil {
			return nil, domainerr.ErrEmailTaken
		}

		return nil, errors.WithMessage(err, "create verified account failed")
	}

	return user, nil
}

// LinkProviderIdentity 已登录态下的显式绑定：同一个身份不会挂到两个账号，
// 同一账号在每个 provider 上也只允许一个身份（(user_id, provider) 唯一）。
func (s *UserService) LinkProviderIdentity(
	ctx context.Context,
	userId valobj.Uid,
	params ResolveProviderParams,
) (*entity.User, error) {
	existing, err := s.identityRepo.GetByProviderAndSubject(ctx, params.Provider, params.Subject)
	if err == nil {
		if !existing.BelongsTo(userId) {
			return nil, domainerr.ErrIdentityAlreadyLinked
		}

		return s.userRepo.GetById(ctx, userId)
	}
	if !errors.Is(err, domainerr.ErrIdentityNotFound) {
		return nil, errors.WithMessage(err, "lookup identity failed")
	}

	// (provider, subject) 未占用，此时同 provider 已存在的行必然是另一个三方账号
	taken, err := s.providerBoundByOtherSubject(ctx, userId, params.Provider, params.Subject)
	if err != nil {
		return nil, err
	}
	if taken {
		return nil, domainerr.ErrProviderAlreadyBound
	}

	identity := entity.NewUserIdentity(
		userId,
		params.Provider,
		params.Subject,
		params.Email,
		params.EmailVerified,
	)
	if err := s.identityRepo.Create(ctx, identity); err != nil {
		// 并发兜底：唯一索引冲突说明已被建好
		if again, lookupErr := s.identityRepo.GetByProviderAndSubject(
			ctx, params.Provider, params.Subject,
		); lookupErr == nil {
			if !again.BelongsTo(userId) {
				return nil, domainerr.ErrIdentityAlreadyLinked
			}

			return s.userRepo.GetById(ctx, userId)
		}

		// 并发兜底：同 provider 的另一个身份刚被挂到这个账号上
		if taken, checkErr := s.providerBoundByOtherSubject(
			ctx, userId, params.Provider, params.Subject,
		); checkErr == nil && taken {
			return nil, domainerr.ErrProviderAlreadyBound
		}

		return nil, errors.WithMessage(err, "link identity failed")
	}

	return s.userRepo.GetById(ctx, userId)
}

// SetPassword 设置或修改口令：无口令时直接设置；已有口令必须给出正确的旧口令，
func (s *UserService) SetPassword(
	ctx context.Context,
	userId valobj.Uid,
	oldPassword, newPassword string,
) error {
	if err := entity.ValidatePassword(newPassword); err != nil {
		return err
	}

	current, err := s.userRepo.GetPasswordHash(ctx, userId)
	if err != nil {
		return errors.WithMessage(err, "get password hash failed")
	}

	if current != "" {
		if oldPassword == "" {
			return domainerr.ErrInvalidCredentials
		}
		if err := s.hasher.Verify(ctx, current, oldPassword); err != nil {
			return domainerr.ErrInvalidCredentials
		}
		// 与库那份哈希比，而不是比客户端传来的两个明文：这才是"当前口令"的权威来源
		if err := s.hasher.Verify(ctx, current, newPassword); err == nil {
			return domainerr.ErrPasswordUnchanged
		}
	}

	newHash, err := s.hasher.Hash(ctx, newPassword)
	if err != nil {
		return errors.WithMessage(err, "hash password failed")
	}

	if err := s.userRepo.UpdatePasswordHash(ctx, userId, newHash); err != nil {
		return errors.WithMessage(err, "update password failed")
	}

	return nil
}

// EnsureNotLastLoginMethod 无口令且只剩一条身份时不允许解绑：解完账号就再也登不进来了。
func (s *UserService) EnsureNotLastLoginMethod(ctx context.Context, userId valobj.Uid) error {
	passwordHash, err := s.userRepo.GetPasswordHash(ctx, userId)
	if err != nil {
		return errors.WithMessage(err, "get password hash failed")
	}
	if passwordHash != "" {
		return nil
	}

	identities, err := s.identityRepo.ListByUserId(ctx, userId)
	if err != nil {
		return errors.WithMessage(err, "list identities failed")
	}
	if len(identities) <= 1 {
		return domainerr.ErrLastLoginMethod
	}

	return nil
}

// providerBoundByOtherSubject 账号在该 provider 上是否已挂了 subject 不同的身份。
// 唯一约束要求每个 provider 每账号最多一个身份，所以这类写入必须先拦在应用层，
// 否则唯一索引冲突会以 500 而不是业务错误码返回。
func (s *UserService) providerBoundByOtherSubject(
	ctx context.Context,
	userId valobj.Uid,
	provider entity.ProviderType,
	subject string,
) (bool, error) {
	identities, err := s.identityRepo.ListByUserId(ctx, userId)
	if err != nil {
		return false, errors.WithMessage(err, "list identities failed")
	}

	for _, identity := range identities {
		if identity.Provider == provider && identity.Subject != subject {
			return true, nil
		}
	}

	return false, nil
}

func nicknameFromEmail(email string) string {
	if at := strings.IndexByte(email, '@'); at > 0 {
		return email[:at]
	}

	return email
}

func (s *UserService) deleteAvatar(ctx context.Context, userId string, avatarKey valobj.StoreKey) {
	if err := s.objectStore.DeleteObject(ctx, avatarKey); err != nil {
		slog.ErrorContext(ctx, "rollback uploaded avatar failed",
			slog.String("user_id", userId),
			slog.String("avatar", avatarKey.String()),
			slog.Any("err", err),
		)
	}
}

func (s *UserService) attachAvatar(
	ctx context.Context,
	user *entity.User,
	outerAvatarURL string,
) (valobj.StoreKey, bool) {
	if outerAvatarURL == "" {
		return valobj.StoreKey{}, false
	}

	avatarKey, err := user.GenerateAvatarKey(s.keyFactory)
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
