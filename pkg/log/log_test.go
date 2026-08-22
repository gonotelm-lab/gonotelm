package log

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"regexp"
	"strings"
	"testing"
	"time"

	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
	"github.com/gonotelm-lab/gonotelm/pkg/ulid"
)

func TestContextJSONHandler_AddsUserIDFromContext(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(newContextJSONHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	uid := ulid.MustParseString("01hf7yat00vtpvxvyaztxbw001")
	ctx := pkgcontext.WithUserId(context.Background(), uid)
	logger.InfoContext(ctx, "hello")

	var got map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal log json failed: %v", err)
	}

	if got[pkgcontext.AttrKeyUserID] != uid.String() {
		t.Fatalf("user_id mismatch, got=%v", got[pkgcontext.AttrKeyUserID])
	}
}

func TestReplaceAttr_SourceFileBaseNameAndTimeMillisecond(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(newContextJSONHandler(&buf, &slog.HandlerOptions{
		Level:       slog.LevelInfo,
		AddSource:   true,
		ReplaceAttr: replaceAttr,
	}))

	logger.Info("hello")

	var got map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal log json failed: %v", err)
	}

	sourceRaw, ok := got[slog.SourceKey]
	if !ok {
		t.Fatalf("source is missing in log output")
	}

	source, ok := sourceRaw.(map[string]any)
	if !ok {
		t.Fatalf("source type mismatch, got=%T", sourceRaw)
	}

	file, ok := source["file"].(string)
	if !ok || strings.TrimSpace(file) == "" {
		t.Fatalf("source.file is missing, got=%v", source["file"])
	}

	if strings.Contains(file, "/") || strings.Contains(file, "\\") {
		t.Fatalf("source.file should be base name, got=%q", file)
	}

	timeRaw, ok := got[slog.TimeKey].(string)
	if !ok || timeRaw == "" {
		t.Fatalf("time is missing, got=%v", got[slog.TimeKey])
	}
	if _, err := time.Parse(time.RFC3339Nano, timeRaw); err != nil {
		t.Fatalf("time parse failed: %v, raw=%q", err, timeRaw)
	}
	// RFC3339Nano with millisecond precision: fractional seconds at most 3 digits.
	if matched, _ := regexp.MatchString(`\.\d{4,}`, timeRaw); matched {
		t.Fatalf("time should keep at most millisecond precision, got=%q", timeRaw)
	}
}

func TestHertzComponentLogger_AddsComponentField(t *testing.T) {
	var buf bytes.Buffer
	level := &slog.LevelVar{}
	level.Set(slog.LevelInfo)

	logger := newHertzSlogger(level, &buf)
	logger.Infof("hello %s", "hertz")

	var got map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal log json failed: %v", err)
	}

	component, ok := got[AttrKeyComponent].(string)
	if !ok || component == "" {
		t.Fatalf("component field is missing, got=%v", got[AttrKeyComponent])
	}

	if component != ComponentHertz {
		t.Fatalf("component mismatch, got=%q want=%q", component, ComponentHertz)
	}
}
