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

func (s *UserService) GetUser(ctx context.Context, userId valobj.Uid) (*entity.User, error) {
	return s.userRepo.GetById(ctx, userId)
}

// GetAccount 读取账号聚合
func (s *UserService) GetAccount(ctx context.Context, userId valobj.Uid) (*entity.UserAggregate, error) {
	user, err := s.userRepo.GetById(ctx, userId)
	if err != nil {
		return nil, err
	}

	return s.assembleAccount(ctx, user)
}

func (s *UserService) GetAccountByEmail(ctx context.Context, email string) (*entity.UserAggregate, error) {
	user, err := s.userRepo.GetByEmail(ctx, entity.NormalizeEmail(email))
	if err != nil {
		return nil, err
	}

	return s.assembleAccount(ctx, user)
}

func (s *UserService) assembleAccount(ctx context.Context, user *entity.User) (*entity.UserAggregate, error) {
	identities, err := s.identityRepo.ListByUserId(ctx, user.Id)
	if err != nil {
		return nil, errors.WithMessage(err, "list user identities failed")
	}

	hash, err := s.userRepo.GetPasswordHash(ctx, user.Id)
	if err != nil {
		return nil, errors.WithMessage(err, "get password hash failed")
	}

	return entity.NewUserAggregate(user, identities, hash != ""), nil
}

// UnbindIdentity 解绑账号下的一条身份并落库
func (s *UserService) UnbindIdentity(
	ctx context.Context,
	agg *entity.UserAggregate,
	bindId valobj.Id,
	provider entity.ProviderType,
) error {
	removed, err := agg.UnbindIdentity(bindId, provider)
	if err != nil {
		return err
	}

	if err := s.identityRepo.Delete(ctx, removed.Id); err != nil {
		return errors.WithMessage(err, "delete identity failed")
	}

	return nil
}

type ResolveProviderParams struct {
	Provider       entity.ProviderType
	Subject        string
	Email          string
	EmailVerified  bool
	Nickname       string
	OuterAvatarURL string
}

// registerLockKey 首登创建账号的分布式锁，key 与身份键一致，避免同一 (provider, subject) 并发创建账号
func registerLockKey(provider entity.ProviderType, subject string) string {
	return fmt.Sprintf("gonotelm:identity:lock:login:%s:%s", provider, subject)
}

// ResolveOrCreateByProvider 三方登录入口：先按 (provider, subject) 查身份
// 查不到时只接受 provider 已验证的邮箱，邮箱已有账号则绑定，否则创建账号
func (s *UserService) ResolveOrCreateByProvider(
	ctx context.Context,
	params ResolveProviderParams,
) (*entity.UserAggregate, error) {
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

	agg, err := s.GetAccountByEmail(ctx, email)
	if err == nil {
		return s.linkIdentity(ctx, agg, params, email)
	}
	if !errors.Is(err, domainerr.ErrUserNotFound) {
		return nil, errors.WithMessage(err, "get user by email failed")
	}

	return s.createAccount(ctx, params, email)
}

func (s *UserService) loginWithIdentity(
	ctx context.Context,
	identity *entity.UserIdentity,
) (*entity.UserAggregate, error) {
	agg, err := s.GetAccount(ctx, identity.UserId)
	if err != nil {
		return nil, errors.WithMessage(err, "get user of identity failed")
	}

	if updated := agg.TouchIdentityLogin(identity.Id, valobj.NewTime()); updated != nil {
		if err := s.identityRepo.TouchLogin(ctx, updated); err != nil {
			slog.WarnContext(ctx, "touch identity login time failed", slog.Any("err", err))
		}
	}

	return agg, nil
}

// linkIdentity 邮箱验证过的账号已存在 → 直接把身份挂上去
func (s *UserService) linkIdentity(
	ctx context.Context,
	agg *entity.UserAggregate,
	params ResolveProviderParams,
	email string,
) (*entity.UserAggregate, error) {
	if agg.HasIdentityOfAnotherSubject(params.Provider, params.Subject) {
		// 邮箱已验证说明就是本人，登录放行，只是不再挂第二个同 provider 身份
		slog.WarnContext(ctx, "skip linking second identity of same provider",
			slog.String("user_id", agg.Id().String()),
			slog.String("provider", params.Provider.String()),
			slog.String("subject", params.Subject),
		)

		return agg, nil
	}

	identity := entity.NewUserIdentity(agg.Id(), params.Provider, params.Subject, email, true)
	if err := agg.BindIdentity(identity); err != nil {
		return nil, errors.WithMessage(err, "bind identity failed")
	}

	if err := s.identityRepo.Create(ctx, identity); err != nil {
		// 唯一索引冲突说明并发下身份已被建好，重查即可
		if existing, lookupErr := s.identityRepo.GetByProviderAndSubject(
			ctx, params.Provider, params.Subject,
		); lookupErr == nil {
			return s.loginWithIdentity(ctx, existing)
		}

		// 另一个并发请求刚给同一账号挂了同 provider 的别的身份
		if reloaded, checkErr := s.GetAccount(ctx, agg.Id()); checkErr == nil &&
			reloaded.HasIdentityOfAnotherSubject(params.Provider, params.Subject) {
			return reloaded, nil
		}

		return nil, errors.WithMessage(err, "link identity failed")
	}

	return agg, nil
}

func (s *UserService) createAccount(
	ctx context.Context,
	params ResolveProviderParams,
	email string,
) (*entity.UserAggregate, error) {
	user := entity.NewUser(email, params.Nickname, valobj.NewTime())
	avatarKey, hasAvatar := s.attachAvatar(ctx, user, params.OuterAvatarURL)

	identity := entity.NewUserIdentity(user.Id, params.Provider, params.Subject, email, true)
	if err := s.userRepo.CreateWithFirstIdentity(ctx, user, identity); err != nil {
		if hasAvatar {
			s.deleteAvatar(ctx, user.Id.String(), avatarKey)
		}

		// 并发下另一请求可能已建好身份
		if existing, lookupErr := s.identityRepo.GetByProviderAndSubject(
			ctx, params.Provider, params.Subject,
		); lookupErr == nil {
			return s.loginWithIdentity(ctx, existing)
		}

		return nil, errors.WithMessage(err, "create user failed")
	}

	return entity.NewUserAggregate(user, []*entity.UserIdentity{identity}, false), nil
}

// createVerifiedAccount 邮箱验证通过后创建账号，passwordHash 非空时一并写入
func (s *UserService) createVerifiedAccount(
	ctx context.Context,
	email, passwordHash string,
) (*entity.UserAggregate, error) {
	normalized := entity.NormalizeEmail(email)
	user := entity.NewUser(normalized, nicknameFromEmail(normalized), valobj.NewTime())

	var err error
	if passwordHash != "" {
		err = s.userRepo.CreateWithPassword(ctx, user, passwordHash)
	} else {
		err = s.userRepo.Create(ctx, user)
	}
	if err != nil {
		// 同一邮箱只允许一个账号，冲突统一按邮箱已注册返回
		if existing, lookupErr := s.userRepo.GetByEmail(ctx, normalized); lookupErr == nil && existing != nil {
			return nil, domainerr.ErrEmailTaken
		}

		return nil, errors.WithMessage(err, "create verified account failed")
	}

	return entity.NewUserAggregate(user, nil, passwordHash != ""), nil
}

// BindIdentity 已登录时主动绑定：同一个身份不会挂到两个账号
// 同一账号在每个 provider 上也只允许一个身份（(user_id, provider) 唯一）
func (s *UserService) BindIdentity(
	ctx context.Context,
	userId valobj.Uid,
	params ResolveProviderParams,
) (*entity.UserAggregate, error) {
	existing, err := s.identityRepo.GetByProviderAndSubject(ctx, params.Provider, params.Subject)
	if err == nil {
		if !existing.BelongsTo(userId) {
			return nil, domainerr.ErrIdentityAlreadyLinked
		}

		return s.GetAccount(ctx, userId)
	}
	if !errors.Is(err, domainerr.ErrIdentityNotFound) {
		return nil, errors.WithMessage(err, "lookup identity failed")
	}

	agg, err := s.GetAccount(ctx, userId)
	if err != nil {
		return nil, err
	}
	// (provider, subject) 未占用，此时同 provider 已存在的行必然是另一个三方账号
	if agg.HasIdentityOfAnotherSubject(params.Provider, params.Subject) {
		return nil, domainerr.ErrProviderAlreadyBound
	}

	identity := entity.NewUserIdentity(
		userId,
		params.Provider,
		params.Subject,
		params.Email,
		params.EmailVerified,
	)
	if err := agg.BindIdentity(identity); err != nil {
		return nil, errors.WithMessage(err, "bind identity failed")
	}

	if err := s.identityRepo.Create(ctx, identity); err != nil {
		// 唯一索引冲突说明并发下已被建好
		if again, lookupErr := s.identityRepo.GetByProviderAndSubject(
			ctx, params.Provider, params.Subject,
		); lookupErr == nil {
			if !again.BelongsTo(userId) {
				return nil, domainerr.ErrIdentityAlreadyLinked
			}

			return s.GetAccount(ctx, userId)
		}

		// 并发下同 provider 的另一个身份刚被挂到这个账号上
		if reloaded, checkErr := s.GetAccount(ctx, userId); checkErr == nil &&
			reloaded.HasIdentityOfAnotherSubject(params.Provider, params.Subject) {
			return nil, domainerr.ErrProviderAlreadyBound
		}

		return nil, errors.WithMessage(err, "link identity failed")
	}

	return agg, nil
}

// VerifyPassword 校验账号密码，无密码与密码错误一样返回 ErrInvalidCredentials
func (s *UserService) VerifyPassword(ctx context.Context, userId valobj.Uid, plain string) error {
	hash, err := s.userRepo.GetPasswordHash(ctx, userId)
	if err != nil {
		return errors.WithMessage(err, "get password hash failed")
	}
	if hash == "" {
		return domainerr.ErrInvalidCredentials
	}
	if err := s.hasher.Verify(ctx, hash, plain); err != nil {
		return domainerr.ErrInvalidCredentials
	}

	return nil
}

// SetPassword 改密码，已有密码时必须先校验旧密码
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
		// 与库里的哈希比，不比客户端传来的两个密码
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

// resetPassword 找回密码确认后写入新密码，不校验旧密码
// 只允许同包的 PasswordResetService 调用：它必须先确认过一次性凭证
// 否则这个方法等于无条件改号
// 跟 SetPassword 的区别：不比旧密码，也不判新旧是否相同
func (s *UserService) resetPassword(
	ctx context.Context,
	userId valobj.Uid,
	newPassword string,
) error {
	if err := entity.ValidatePassword(newPassword); err != nil {
		return err
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

// resolveAvatarContentType 先看响应头，再按内容嗅探
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
