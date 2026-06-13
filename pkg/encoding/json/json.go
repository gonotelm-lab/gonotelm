package json

import (
	stdjson "encoding/json"
	"fmt"
	"reflect"
	"strings"

	"github.com/bytedance/sonic"
)

type Decoder struct {
	DisallowUnknownFields bool

	// LogOnDirectFailure is called when direct unmarshal against full payload fails
	// and decoder is about to try compatibility fallbacks.
	// This callback is optional.
	LogOnDirectFailure func(err error, data []byte)
}

var defaultDecoder = Decoder{}

// Unmarshal is a compatibility JSON parser.
//
// It first tries direct unmarshal against the full payload.
// If that fails, it attempts to extract a JSON object from
// noisy model output (e.g. prefixed explanation text or ```json blocks).
func Unmarshal(data []byte, v any) error {
	return defaultDecoder.Unmarshal(data, v)
}

// UnmarshalStrict works like Unmarshal, but additionally rejects unknown JSON fields
// when target v is a struct pointer.
func UnmarshalStrict(data []byte, v any) error {
	return Decoder{DisallowUnknownFields: true}.Unmarshal(data, v)
}

func (d Decoder) Unmarshal(data []byte, v any) error {
	directErr := sonic.Unmarshal(data, v)
	if directErr == nil {
		if d.DisallowUnknownFields {
			if err := rejectUnknownFields(data, v); err != nil {
				return err
			}
		}
		return nil
	}

	if d.LogOnDirectFailure != nil {
		d.LogOnDirectFailure(directErr, data)
	}

	content := strings.TrimSpace(string(data))
	candidates := jsonObjectCandidates(content)
	lastErr := directErr
	for _, candidate := range candidates {
		if candidate == "" || candidate == content {
			continue
		}

		candidateBytes := []byte(candidate)
		if err := sonic.Unmarshal(candidateBytes, v); err == nil {
			if d.DisallowUnknownFields {
				if err := rejectUnknownFields(candidateBytes, v); err != nil {
					lastErr = err
					continue
				}
			}
			return nil
		} else {
			lastErr = err
		}
	}

	if lastErr != nil {
		return lastErr
	}

	return directErr
}

func rejectUnknownFields(data []byte, v any) error {
	allowed, enabled := allowedJSONFields(v)
	if !enabled {
		return nil
	}

	var payload map[string]stdjson.RawMessage
	if err := sonic.Unmarshal(data, &payload); err != nil {
		return err
	}

	for key := range payload {
		if _, ok := allowed[key]; !ok {
			return fmt.Errorf("unknown json field: %s", key)
		}
	}

	return nil
}

func allowedJSONFields(v any) (map[string]struct{}, bool) {
	t := reflect.TypeOf(v)
	if t == nil {
		return nil, false
	}

	for t.Kind() == reflect.Ptr {
		t = t.Elem()
		if t == nil {
			return nil, false
		}
	}
	if t.Kind() != reflect.Struct {
		return nil, false
	}

	allowed := make(map[string]struct{})
	collectAllowedFields(t, allowed)
	return allowed, true
}

func collectAllowedFields(t reflect.Type, allowed map[string]struct{}) {
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)

		if field.PkgPath != "" && !field.Anonymous {
			// unexported non-anonymous field
			continue
		}

		tag := field.Tag.Get("json")
		if field.Anonymous && tag == "" {
			ft := field.Type
			for ft.Kind() == reflect.Pointer {
				ft = ft.Elem()
			}
			if ft.Kind() == reflect.Struct {
				collectAllowedFields(ft, allowed)
			}
			continue
		}

		name, skip := parseJSONFieldTag(tag, field.Name)
		if skip {
			continue
		}

		if name != "" {
			allowed[name] = struct{}{}
		}
	}
}

func parseJSONFieldTag(tag string, defaultName string) (name string, skip bool) {
	if tag == "-" {
		return "", true
	}
	if tag == "" {
		return defaultName, false
	}

	parts := strings.Split(tag, ",")
	if len(parts) == 0 {
		return defaultName, false
	}
	if parts[0] == "" {
		return defaultName, false
	}

	return parts[0], false
}

func jsonObjectCandidates(content string) []string {
	candidates := make([]string, 0, 3)
	candidates = append(candidates, strings.TrimSpace(content))

	if blockJSON := extractJSONCodeBlock(content); blockJSON != "" {
		candidates = append(candidates, blockJSON)
	}
	if objectJSON := extractFirstJSONObject(content); objectJSON != "" {
		candidates = append(candidates, objectJSON)
	}

	uniq := make([]string, 0, len(candidates))
	seen := make(map[string]struct{}, len(candidates))
	for _, candidate := range candidates {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		if _, ok := seen[candidate]; ok {
			continue
		}
		seen[candidate] = struct{}{}
		uniq = append(uniq, candidate)
	}

	return uniq
}

func extractJSONCodeBlock(content string) string {
	_, after, ok := strings.Cut(content, "```json")
	if !ok {
		return ""
	}

	rest := after
	before, _, ok0 := strings.Cut(rest, "```")
	if !ok0 {
		return ""
	}

	return strings.TrimSpace(before)
}

func extractFirstJSONObject(content string) string {
	start := strings.Index(content, "{")
	if start < 0 {
		return ""
	}

	inString := false
	escaped := false
	depth := 0
	for idx := start; idx < len(content); idx++ {
		ch := content[idx]
		if inString {
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if ch == '"' {
				inString = false
			}
			continue
		}

		switch ch {
		case '"':
			inString = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return strings.TrimSpace(content[start : idx+1])
			}
		}
	}

	return ""
}
