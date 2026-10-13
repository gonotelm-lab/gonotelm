package service

import (
	"context"
	stderrors "errors"
	"testing"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	"github.com/gonotelm-lab/gonotelm/internal/domain/identity/entity"
	identerr "github.com/gonotelm-lab/gonotelm/internal/domain/identity/errors"
	pkgerrors "github.com/gonotelm-lab/gonotelm/pkg/errors"
)

var errUnexpectedCall = stderrors.New("unexpected repository call")

// ── fakes ─────────────────────────────────────────────────────────────────

type fakeResetRepo struct {
	pending   map[string]*entity.PasswordResetPending
	saved     int
	cooldowns map[string]bool
}

func newFakeResetRepo() *fakeResetRepo {
	return &fakeResetRepo{
		pending:   map[string]*entity.PasswordResetPending{},
		cooldowns: map[string]bool{},
	}
}

func (f *fakeResetRepo) Save(
	_ context.Context,
	pending *entity.PasswordResetPending,
	_ string,
) error {
	// 与真实实现一致：同一个邮箱只有一条记录
	f.pending[pending.Email] = pending
	f.saved++

	return nil
}

func (f *fakeResetRepo) GetByEmail(_ context.Context, email string) (*entity.PasswordResetPending, error) {
	if pending, ok := f.pending[email]; ok {
		return pending, nil
	}

	return nil, identerr.ErrVerificationExpired
}

func (f *fakeResetRepo) GetByLinkToken(
	_ context.Context,
	linkToken string,
) (*entity.PasswordResetPending, error) {
	hash := entity.HashSecret(linkToken)
	for _, pending := range f.pending {
		if pending.LinkTokenHash == hash {
			return pending, nil
		}
	}

	return nil, identerr.ErrVerificationExpired
}

func (f *fakeResetRepo) DeleteByEmail(_ context.Context, email string) error {
	delete(f.pending, email)

	return nil
}

func (f *fakeResetRepo) StartSendCooldown(_ context.Context, email string) error {
	if f.cooldowns[email] {
		return identerr.ErrTooManyRequests
	}
	f.cooldowns[email] = true

	return nil
}

func (f *fakeResetRepo) ClearSendCooldown(_ context.Context, email string) error {
	delete(f.cooldowns, email)

	return nil
}

type fakeUserRepo struct {
	users map[string]*entity.User
}

func newFakeUserRepo() *fakeUserRepo {
	return &fakeUserRepo{users: map[string]*entity.User{}}
}

func (f *fakeUserRepo) GetByEmail(_ context.Context, email string) (*entity.User, error) {
	if user, ok := f.users[email]; ok {
		return user, nil
	}

	return nil, identerr.ErrUserNotFound
}

func (f *fakeUserRepo) Create(context.Context, *entity.User) error { return errUnexpectedCall }

func (f *fakeUserRepo) CreateWithPassword(context.Context, *entity.User, string) error {
	return errUnexpectedCall
}

func (f *fakeUserRepo) CreateWithFirstIdentity(
	context.Context,
	*entity.User,
	*entity.UserIdentity,
) error {
	return errUnexpectedCall
}

func (f *fakeUserRepo) GetById(context.Context, valobj.Uid) (*entity.User, error) {
	return nil, errUnexpectedCall
}

func (f *fakeUserRepo) GetPasswordHash(context.Context, valobj.Uid) (string, error) {
	return "", errUnexpectedCall
}

func (f *fakeUserRepo) UpdateProfile(context.Context, *entity.User) error { return errUnexpectedCall }

func (f *fakeUserRepo) UpdateAvatar(context.Context, *entity.User) error { return errUnexpectedCall }

func (f *fakeUserRepo) UpdatePasswordHash(context.Context, valobj.Uid, string) error {
	return errUnexpectedCall
}

type fakePasswordSetter struct {
	calls []valobj.Uid
	plain []string
	err   error
}

func (f *fakePasswordSetter) ResetPassword(
	_ context.Context,
	userId valobj.Uid,
	newPassword string,
) error {
	f.calls = append(f.calls, userId)
	f.plain = append(f.plain, newPassword)

	return f.err
}

// ── helpers ───────────────────────────────────────────────────────────────

func newResetFixture(email string) (*PasswordResetService, *fakeResetRepo, *fakeUserRepo, *fakePasswordSetter) {
	repo := newFakeResetRepo()
	users := newFakeUserRepo()
	if email != "" {
		users.users[email] = &entity.User{Id: valobj.NewUid(), Email: email}
	}
	setter := &fakePasswordSetter{}

	return NewPasswordResetService(repo, users, setter), repo, users, setter
}

// ── 账号是否存在不外泄 ────────────────────────────────────────────────────

// 账号存不存在，响应和限流表现都得一样，不然这个接口就能拿来查邮箱有没有注册
func TestPasswordResetStartDoesNotRevealAccountExistence(t *testing.T) {
	const known = "known@example.com"
	const unknown = "unknown@example.com"

	svc, repo, _, _ := newResetFixture(known)
	ctx := t.Context()

	knownDispatch, knownErr := svc.Start(ctx, known)
	unknownDispatch, unknownErr := svc.Start(ctx, unknown)

	if knownErr != nil || unknownErr != nil {
		t.Fatalf("first request must succeed for both, known=%v unknown=%v", knownErr, unknownErr)
	}
	if !knownDispatch.Sent {
		t.Fatal("an existing account must trigger the email")
	}
	if unknownDispatch.Sent {
		t.Fatal("an unknown account must stay silent")
	}
	if _, ok := repo.pending[unknown]; ok {
		t.Fatal("an unknown account must not write a pending")
	}
	if _, ok := repo.pending[known]; !ok {
		t.Fatal("an existing account must get a pending")
	}
	if repo.saved != 1 {
		t.Fatalf("want exactly 1 saved pending, got %d", repo.saved)
	}

	// 再请求一次两边都该被限流，不能只有存在的账号被限
	_, knownErr = svc.Start(ctx, known)
	_, unknownErr = svc.Start(ctx, unknown)

	if !pkgerrors.Is(knownErr, identerr.ErrTooManyRequests) {
		t.Fatalf("second request for an existing account = %v, want ErrTooManyRequests", knownErr)
	}
	if !pkgerrors.Is(unknownErr, identerr.ErrTooManyRequests) {
		t.Fatalf("second request for an unknown account = %v, want ErrTooManyRequests", unknownErr)
	}
}

// 未知账号也要占住发送间隔，不然光看限流状态就知道邮箱有没有注册
func TestPasswordResetStartClaimsCooldownForUnknownEmail(t *testing.T) {
	svc, repo, _, _ := newResetFixture("")
	ctx := t.Context()

	if _, err := svc.Start(ctx, "unknown@example.com"); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	if !repo.cooldowns["unknown@example.com"] {
		t.Fatal("cooldown must be claimed even when the account does not exist")
	}
}

func TestPasswordResetStartRejectsBadEmail(t *testing.T) {
	svc, _, _, _ := newResetFixture("")

	if _, err := svc.Start(t.Context(), "not-an-email"); !pkgerrors.Is(err, identerr.ErrEmailInvalid) {
		t.Fatalf("Start = %v, want ErrEmailInvalid", err)
	}
}

// ── 链接轮换 ──────────────────────────────────────────────────────────────

// 重新发起要换掉链接 token，旧链接必须立刻失效
func TestPasswordResetStartRotatesCredentials(t *testing.T) {
	const email = "user@example.com"
	svc, repo, _, _ := newResetFixture(email)
	ctx := t.Context()

	first, err := svc.Start(ctx, email)
	if err != nil {
		t.Fatalf("first Start failed: %v", err)
	}
	if first.LinkToken == "" {
		t.Fatal("the token must be returned to the caller")
	}

	// 清掉冷却，模拟用户过一会儿再点一次发送
	if err := repo.ClearSendCooldown(ctx, email); err != nil {
		t.Fatalf("clear cooldown failed: %v", err)
	}

	second, err := svc.Start(ctx, email)
	if err != nil {
		t.Fatalf("second Start failed: %v", err)
	}

	if first.LinkToken == second.LinkToken {
		t.Fatal("re-issuing must rotate the token")
	}
	if _, err := repo.GetByLinkToken(ctx, first.LinkToken); !pkgerrors.Is(err, identerr.ErrVerificationExpired) {
		t.Fatalf("the previous link must stop working, got %v", err)
	}
	if _, err := repo.GetByLinkToken(ctx, second.LinkToken); err != nil {
		t.Fatalf("the latest link must work, got %v", err)
	}
}

// ── 确认 ──────────────────────────────────────────────────────────────────

func TestPasswordResetConfirmLinkSetsPasswordAndConsumesCredential(t *testing.T) {
	const email = "user@example.com"
	const newPassword = "s3cret-pass"
	svc, repo, users, setter := newResetFixture(email)
	ctx := t.Context()

	dispatch, err := svc.Start(ctx, email)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	userId, err := svc.ConfirmLink(ctx, dispatch.LinkToken, newPassword)
	if err != nil {
		t.Fatalf("ConfirmLink failed: %v", err)
	}

	if userId != users.users[email].Id {
		t.Fatal("ConfirmLink must return the account it reset")
	}
	if len(setter.calls) != 1 || setter.calls[0] != userId || setter.plain[0] != newPassword {
		t.Fatalf("password setter calls = %v %v", setter.calls, setter.plain)
	}
	if _, err := repo.GetByLinkToken(ctx, dispatch.LinkToken); !pkgerrors.Is(err, identerr.ErrVerificationExpired) {
		t.Fatal("the credential must be consumed after a successful reset")
	}
}

func TestPasswordResetConfirmLinkRejectsUnknownToken(t *testing.T) {
	const email = "user@example.com"
	svc, _, _, setter := newResetFixture(email)

	_, err := svc.ConfirmLink(t.Context(), "unknown-token", "s3cret-pass")
	if !pkgerrors.Is(err, identerr.ErrVerificationExpired) {
		t.Fatalf("ConfirmLink = %v, want ErrVerificationExpired", err)
	}
	if len(setter.calls) != 0 {
		t.Fatal("a bad token must never reach the password writer")
	}
}

// 密码不合法时不消费凭证，用户可以用同一个链接重试
func TestPasswordResetConfirmLinkKeepsCredentialWhenPasswordRejected(t *testing.T) {
	const email = "user@example.com"
	svc, repo, _, setter := newResetFixture(email)
	ctx := t.Context()
	setter.err = identerr.ErrWeakPassword

	dispatch, err := svc.Start(ctx, email)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	_, err = svc.ConfirmLink(ctx, dispatch.LinkToken, "weak")
	if !pkgerrors.Is(err, identerr.ErrWeakPassword) {
		t.Fatalf("ConfirmLink = %v, want ErrWeakPassword", err)
	}
	if _, err := repo.GetByLinkToken(ctx, dispatch.LinkToken); err != nil {
		t.Fatalf("the credential must survive a rejected password, got %v", err)
	}
}

// 账号在发起之后被删除：凭证不再指向任何账号，按凭证失效处理
func TestPasswordResetConfirmLinkTreatsDeletedAccountAsExpired(t *testing.T) {
	const email = "user@example.com"
	svc, _, users, setter := newResetFixture(email)
	ctx := t.Context()

	dispatch, err := svc.Start(ctx, email)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	delete(users.users, email)

	if _, err := svc.ConfirmLink(ctx, dispatch.LinkToken, "s3cret-pass"); !pkgerrors.Is(err, identerr.ErrVerificationExpired) {
		t.Fatalf("ConfirmLink = %v, want ErrVerificationExpired", err)
	}
	if len(setter.calls) != 0 {
		t.Fatal("a deleted account must not reach the password writer")
	}
}
