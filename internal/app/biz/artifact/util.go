package artifact

import (
	"time"

	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
)

func taskRunId() string {
	return uuid.NewV4().String()
}

func getUnixMilli() int64 {
	return time.Now().UnixMilli()
}
