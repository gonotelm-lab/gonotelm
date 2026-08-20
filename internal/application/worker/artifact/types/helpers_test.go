package types

import (
	"testing"

	einoschema "github.com/cloudwego/eino/schema"
	"github.com/stretchr/testify/require"
)

func TestBuildTipMessage(t *testing.T) {
	t.Parallel()

	t.Run("empty_tip_still_returns_user", func(t *testing.T) {
		t.Parallel()
		msg := BuildTipMessage("")
		require.NotNil(t, msg)
		require.Equal(t, einoschema.User, msg.Role)
		require.NotEmpty(t, msg.Content)
	})

	t.Run("non_empty_tip_wraps_extra_requirement", func(t *testing.T) {
		t.Parallel()
		msg := BuildTipMessage(" keep it short ")
		require.NotNil(t, msg)
		require.Equal(t, einoschema.User, msg.Role)
		require.Contains(t, msg.Content, "keep it short")
		require.Contains(t, msg.Content, "<user_extra_input>")
	})
}
