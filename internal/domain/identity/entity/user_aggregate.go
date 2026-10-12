package entity

import (
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	identerr "github.com/gonotelm-lab/gonotelm/internal/domain/identity/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

type UserAggregate struct {
	user        *User
	identities  []*UserIdentity
	hasPassword bool
}

func NewUserAggregate(user *User, identities []*UserIdentity, hasPassword bool) *UserAggregate {
	return &UserAggregate{
		user:        user,
		identities:  identities,
		hasPassword: hasPassword,
	}
}

func (a *UserAggregate) User() *User { return a.user }

func (a *UserAggregate) Id() valobj.Uid { return a.user.Id }

func (a *UserAggregate) HasPassword() bool { return a.hasPassword }

func (a *UserAggregate) IsBanned() bool { return a.user.IsBanned() }

func (a *UserAggregate) IsEmailVerified() bool { return a.user.IsEmailVerified() }

func (a *UserAggregate) Identities() []*UserIdentity {
	return append([]*UserIdentity(nil), a.identities...)
}

// LoginMethodCount 密码算一种登录方式，每条三方身份算一种。
func (a *UserAggregate) LoginMethodCount() int {
	n := len(a.identities)
	if a.hasPassword {
		n++
	}

	return n
}

func (a *UserAggregate) FindIdentity(provider ProviderType) *UserIdentity {
	for _, identity := range a.identities {
		if identity.Provider == provider {
			return identity
		}
	}

	return nil
}

func (a *UserAggregate) HasIdentity(provider ProviderType) bool {
	return a.FindIdentity(provider) != nil
}

func (a *UserAggregate) HasIdentityOfAnotherSubject(provider ProviderType, subject string) bool {
	for _, identity := range a.identities {
		if identity.Provider == provider && identity.Subject != subject {
			return true
		}
	}

	return false
}

// BindIdentity 追加一条身份；每个 provider 每账号至多一个身份。
func (a *UserAggregate) BindIdentity(identity *UserIdentity) error {
	if a.HasIdentity(identity.Provider) {
		return identerr.ErrProviderAlreadyBound
	}

	a.identities = append(a.identities, identity)

	return nil
}

// UnbindIdentity 摘除一条身份。错误优先级：ErrLastLoginMethod > ErrIdentityNotFound > provider mismatch。
func (a *UserAggregate) UnbindIdentity(id valobj.Id, provider ProviderType) (*UserIdentity, error) {
	if a.LoginMethodCount() <= 1 {
		return nil, identerr.ErrLastLoginMethod
	}

	target := a.findIdentityById(id)
	if target == nil {
		return nil, identerr.ErrIdentityNotFound
	}
	if target.Provider != provider {
		return nil, errors.ErrParams.Msgf("provider mismatch: binding is %s", target.Provider)
	}

	a.removeIdentity(target)

	return target, nil
}

// TouchIdentityLogin 更新该身份的最近登录时间，返回被更新的身份（账号下没有该身份时返回 nil）。
func (a *UserAggregate) TouchIdentityLogin(id valobj.Id, at valobj.Time) *UserIdentity {
	identity := a.findIdentityById(id)
	if identity == nil {
		return nil
	}

	identity.TouchLogin(at)

	return identity
}

func (a *UserAggregate) findIdentityById(id valobj.Id) *UserIdentity {
	for _, identity := range a.identities {
		if identity.Id == id {
			return identity
		}
	}

	return nil
}

func (a *UserAggregate) removeIdentity(target *UserIdentity) {
	kept := make([]*UserIdentity, 0, len(a.identities))
	for _, identity := range a.identities {
		if identity.Id != target.Id {
			kept = append(kept, identity)
		}
	}

	a.identities = kept
}
