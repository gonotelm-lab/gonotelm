package log

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"reflect"
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

	function, ok := source["function"].(string)
	if !ok || strings.TrimSpace(function) == "" {
		t.Fatalf("source.function is missing, got=%v", source["function"])
	}
	if strings.HasPrefix(function, modulePath) {
		t.Fatalf("source.function should drop the module prefix %q, got=%q", modulePath, function)
	}
	if !strings.HasPrefix(function, "pkg/log.TestReplaceAttr_") {
		t.Fatalf("source.function should keep the package path after trimming, got=%q", function)
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

func TestModulePath_ResolvedFromRuntimeBuildInfo(t *testing.T) {
	if modulePath == "" {
		t.Fatal("modulePath is empty: runtime build info is unavailable in this binary")
	}

	if got := reflect.TypeOf(hertzSlogger{}).PkgPath(); got != modulePath+"/pkg/log" {
		t.Fatalf("modulePath=%q does not match the package path %q", modulePath, got)
	}
}

func TestTrimModulePrefix(t *testing.T) {
	cases := []struct {
		name     string
		function string
		want     string
	}{
		{
			name:     "own module package function",
			function: modulePath + "/internal/application/notelm/source/eventhandle.(*OnNotebookEventHandler).Handle",
			want:     "internal/application/notelm/source/eventhandle.(*OnNotebookEventHandler).Handle",
		},
		{
			name:     "own module package level function",
			function: modulePath + "/pkg/log.Info",
			want:     "pkg/log.Info",
		},
		{
			name:     "third party function untouched",
			function: "github.com/cloudwego/hertz/pkg/common/hlog.(*logger).Info",
			want:     "github.com/cloudwego/hertz/pkg/common/hlog.(*logger).Info",
		},
		{
			name:     "lookalike module prefix untouched",
			function: "github.com/gonotelm-lab/gonotelm-lab/pkg/log.Info",
			want:     "github.com/gonotelm-lab/gonotelm-lab/pkg/log.Info",
		},
		{
			name:     "empty function untouched",
			function: "",
			want:     "",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := trimModulePrefix(c.function); got != c.want {
				t.Fatalf("trimModulePrefix(%q)=%q, want=%q", c.function, got, c.want)
			}
		})
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

	source, ok := got[slog.SourceKey].(map[string]any)
	if !ok {
		t.Fatalf("source is missing in hertz log output")
	}
	function, _ := source["function"].(string)
	if !strings.HasPrefix(function, "pkg/log.(*hertzSlogger)") {
		t.Fatalf("hertz source.function should drop the module prefix, got=%q", function)
	}
}
