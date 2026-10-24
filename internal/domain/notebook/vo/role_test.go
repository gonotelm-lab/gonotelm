package vo

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRoleCapabilities(t *testing.T) {
	tests := []struct {
		role              Role
		canView           bool
		canManageContent  bool
		canManageNotebook bool
		canManageSharing  bool
		isMember          bool
	}{
		{RoleOwner, true, true, true, true, false},
		{RoleCollaborator, true, true, false, false, true},
		{RoleViewer, true, false, false, false, true},
		{RoleNone, false, false, false, false, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.role), func(t *testing.T) {
			assert.Equal(t, tt.canView, tt.role.CanView())
			assert.Equal(t, tt.canManageContent, tt.role.CanManageContent())
			assert.Equal(t, tt.canManageNotebook, tt.role.CanManageNotebook())
			assert.Equal(t, tt.canManageSharing, tt.role.CanManageSharing())
			assert.Equal(t, tt.isMember, tt.role.IsMemberRole())
		})
	}
}

func TestNormalizeRole(t *testing.T) {
	assert.Equal(t, RoleViewer, NormalizeRole(""))
	assert.Equal(t, RoleCollaborator, NormalizeRole(RoleCollaborator))
	assert.Equal(t, RoleViewer, NormalizeRole(Role("invalid")))
}
