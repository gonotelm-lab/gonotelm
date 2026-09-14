package opensandbox

import (
	"testing"
	"time"

	"github.com/gonotelm-lab/gonotelm/internal/domain/sandbox/entity"
	"github.com/stretchr/testify/assert"
)

func TestCommandTimeoutMs(t *testing.T) {
	tests := []struct {
		name    string
		timeout time.Duration
		want    int64
	}{
		{name: "zero means execd default", timeout: 0, want: 0},
		{name: "negative means execd default", timeout: -time.Second, want: 0},
		{name: "15min bash default to 900000ms", timeout: 15 * time.Minute, want: 900000},
		{name: "30s to 30000ms", timeout: 30 * time.Second, want: 30000},
		{name: "sub-second keeps ms precision", timeout: 1500 * time.Millisecond, want: 1500},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, commandTimeoutMs(entity.Command{Timeout: tt.timeout}))
		})
	}
}
