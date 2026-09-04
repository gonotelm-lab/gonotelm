package kafka

import (
	"fmt"
	"log/slog"

	"github.com/gonotelm-lab/gonotelm/pkg/log"
)

func kafkaLogger(msg string, args ...any) {
	slog.Info(
		formatKafkaLogMsg(msg, args...),
		slog.String(log.AttrKeyComponent, log.ComponentKafkaGo),
	)
}

func kafkaErrorLogger(msg string, args ...any) {
	slog.Error(
		formatKafkaLogMsg(msg, args...),
		slog.String(log.AttrKeyComponent, log.ComponentKafkaGo),
	)
}

func formatKafkaLogMsg(msg string, args ...any) string {
	if len(args) == 0 {
		return msg
	}

	return fmt.Sprintf(msg, args...)
}
