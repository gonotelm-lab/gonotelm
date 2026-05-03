package log

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"

	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
)

func TestContextJSONHandler_AddsUserIDFromContext(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(newContextJSONHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	ctx := pkgcontext.WithUserId(context.Background(), "u-123")
	logger.InfoContext(ctx, "hello")

	var got map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal log json failed: %v", err)
	}

	if got[AttrKeyUserID] != "u-123" {
		t.Fatalf("user_id mismatch, got=%v", got[AttrKeyUserID])
	}
}

func TestReplaceSourceFileWithBaseName(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(newContextJSONHandler(&buf, &slog.HandlerOptions{
		Level:       slog.LevelInfo,
		AddSource:   true,
		ReplaceAttr: replaceSourceFileWithBaseName,
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
