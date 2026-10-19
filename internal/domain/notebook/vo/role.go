package vo

type Role string

const (
	RoleNone         Role = ""
	RoleOwner        Role = "owner"
	RoleCollaborator Role = "collaborator"
	RoleViewer       Role = "viewer"
)

func (r Role) String() string {
	return string(r)
}

func (r Role) IsValid() bool {
	return r.IsMemberRole() || r == RoleOwner
}

// IsMemberRole 成员表只允许 collaborator / viewer。
func (r Role) IsMemberRole() bool {
	return r == RoleCollaborator || r == RoleViewer
}

func NormalizeRole(r Role) Role {
	if r == "" {
		return RoleViewer
	}
	if !r.IsValid() {
		return RoleViewer
	}

	return r
}

func (r Role) CanView() bool {
	return r == RoleOwner || r == RoleCollaborator || r == RoleViewer
}

func (r Role) CanManageContent() bool {
	return r == RoleOwner || r == RoleCollaborator
}

func (r Role) CanManageNotebook() bool {
	return r == RoleOwner
}

func (r Role) CanManageSharing() bool {
	return r == RoleOwner
}

func NewRole(s string) Role {
	ss := Role(s)

	return NormalizeRole(ss)
}
