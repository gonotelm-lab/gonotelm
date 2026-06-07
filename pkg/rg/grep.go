package rg

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/coregx/coregex"
)

// OutputMode 定义 Grep 的输出模式。
type OutputMode string

const (
	// OutputModeContent 输出实际命中内容（或行级内容）。
	OutputModeContent OutputMode = "content"

	// OutputModeCount 仅输出命中总数。
	OutputModeCount OutputMode = "count"
)

// Params 是内存匹配参数集合（不承载文本本身）。
//
// 设计约束：
//   - 文本通过函数参数传入（[]byte 或 string）
//   - 参数结构只包含“匹配规则”和“输出控制”
type Params struct {
	// 匹配的正则
	Pattern string `json:"pattern" jsonschema_description:"The regular expression pattern to search for in file contents"`

	// OutputMode 为空时默认按 OutputModeContent 处理。
	OutputMode OutputMode `json:"output_mode,omitempty" jsonschema:"enum=content,enum=count" jsonschema_description:"Output mode: 'content' show matching lines (supports -A/-B/-C context, -n line numbers, head_limit), 'count' show match counts (support head_limit). Defaults to 'content'"`

	// BeforeContext 表示每个命中前额外输出多少行（等价 -B）。
	BeforeContext int `json:"-B,omitempty" jsonschema_description:"Number of lines to show before each match. Requires output_mode: 'content', ignored otherwise."`

	// AfterContext 表示每个命中后额外输出多少行（等价 -A）。
	AfterContext int `json:"-A,omitempty" jsonjsonschema_descriptionschema:"Number of lines to show after each match. Requires output_mode: 'content', ignored otherwise."`

	// Context 同时设置前后文行数（等价 -C），优先级高于 Before/After。
	Context int `json:"-C,omitempty" jsonschema_description:"Number of lines to show before and after each match. Requires output_mode: 'content', ignored otherwise."`

	// LineNumber 控制是否输出行号前缀。
	LineNumber bool `json:"-n,omitempty" jsonschema_description:"Show line numbers in output. Requires output_mode: 'content', ignored otherwise."`

	// IgnoreCase 控制是否忽略大小写匹配。
	IgnoreCase bool `json:"-i,omitempty" jsonschema_description:"Case insensitive search."`

	// HeadLimit 限制输出条数（或输出行数，取决于输出分支）。
	HeadLimit int `json:"head_limit,omitempty" jsonschema_description:"Limit output to first N lines/entries, equivalent to '| head -N'. Works across all output modes: content (limits output lines), count (limits count entries). When unspecified, shows all results."`

	// Multiline 为 true 时启用 dotall（即 "." 可匹配换行）。
	Multiline bool `json:"multiline,omitempty" jsonschema_description:"Enable multiline mode where . matches newlines and patterns can span lines. Defaults: false"`
}

// Match 表示一次命中的区间和文本内容。
type Match struct {
	// Start 是命中起始偏移（含）。
	Start int
	// End 是命中结束偏移（不含）。
	End int
	// Text 是命中的原始子串。
	Text string
}

// FindAllMatches 在 []byte 文本中执行正则匹配并返回结构化命中列表。
func FindAllMatches(content []byte, params *Params) ([]Match, error) {
	return findAllMatchesInBytes(content, params.Pattern, params)
}

// FindAllMatchesString 在 string 文本中执行正则匹配并返回结构化命中列表。
func FindAllMatchesString(text string, params *Params) ([]Match, error) {
	return findAllMatchesInSource(text, params.Pattern, params)
}

// Grep 在 []byte 文本中执行匹配，并返回字符串形式结果。
func Grep(content []byte, params *Params) (string, error) {
	return grepInBytes(content, params.Pattern, params)
}

// GrepString 在 string 文本中执行匹配，并返回字符串形式结果。
func GrepString(text string, params *Params) (string, error) {
	return grepInSource(text, params.Pattern, params)
}

// findAllMatchesInSource 是结构化匹配的核心实现。
func findAllMatchesInSource(source string, pattern string, params *Params) ([]Match, error) {
	return findAllMatchesWithRegex(
		pattern,
		params,
		func(re *coregex.Regex) [][]int {
			return re.FindAllStringIndex(source, -1)
		},
		func(indices [][]int, headLimit int) []Match {
			return buildMatchesFromIndices(indices, headLimit, func(start int, end int) string {
				return source[start:end]
			})
		},
	)
}

// findAllMatchesInBytes 是 []byte 结构化匹配的核心实现。
func findAllMatchesInBytes(content []byte, pattern string, params *Params) ([]Match, error) {
	return findAllMatchesWithRegex(
		pattern,
		params,
		func(re *coregex.Regex) [][]int {
			return re.FindAllIndex(content, -1)
		},
		func(indices [][]int, headLimit int) []Match {
			return buildMatchesFromIndices(indices, headLimit, func(start int, end int) string {
				return string(content[start:end])
			})
		},
	)
}

// grepInSource 是字符串输出模式的核心实现。
func grepInSource(source string, pattern string, params *Params) (string, error) {
	return grepWithRegex(
		pattern,
		params,
		func(re *coregex.Regex) [][]int {
			return re.FindAllStringIndex(source, -1)
		},
		func(indices [][]int, headLimit int) []Match {
			return buildMatchesFromIndices(indices, headLimit, func(start int, end int) string {
				return source[start:end]
			})
		},
		func(indices [][]int, lineParams *Params) string {
			return renderLineOutput(source, indices, lineParams)
		},
	)
}

// grepInBytes 是 []byte 输出模式的核心实现。
func grepInBytes(content []byte, pattern string, params *Params) (string, error) {
	return grepWithRegex(
		pattern,
		params,
		func(re *coregex.Regex) [][]int {
			return re.FindAllIndex(content, -1)
		},
		func(indices [][]int, headLimit int) []Match {
			return buildMatchesFromIndices(indices, headLimit, func(start int, end int) string {
				return string(content[start:end])
			})
		},
		func(indices [][]int, lineParams *Params) string {
			return renderLineOutput(string(content), indices, lineParams)
		},
	)
}

type (
	findIndicesFunc      func(*coregex.Regex) [][]int
	buildMatchesFunc     func(indices [][]int, headLimit int) []Match
	renderLineOutputFunc func(indices [][]int, params *Params) string
)

func findAllMatchesWithRegex(
	pattern string,
	params *Params,
	findIndices findIndicesFunc,
	buildMatches buildMatchesFunc,
) ([]Match, error) {
	re, err := compilePattern(pattern, params.IgnoreCase, params.Multiline)
	if err != nil {
		return nil, err
	}
	return buildMatches(findIndices(re), params.HeadLimit), nil
}

func grepWithRegex(
	pattern string,
	params *Params,
	findIndices findIndicesFunc,
	buildMatches buildMatchesFunc,
	renderLine renderLineOutputFunc,
) (string, error) {
	re, err := compilePattern(pattern, params.IgnoreCase, params.Multiline)
	if err != nil {
		return "", err
	}

	mode, err := normalizeOutputMode(params.OutputMode)
	if err != nil {
		return "", err
	}

	indices := findIndices(re)
	if mode == OutputModeCount {
		return strconv.Itoa(len(indices)), nil
	}
	if len(indices) == 0 {
		return "", nil
	}

	if requiresLineOutput(params) {
		return renderLine(indices, params), nil
	}

	return joinMatchText(buildMatches(indices, params.HeadLimit)), nil
}

func normalizeOutputMode(mode OutputMode) (OutputMode, error) {
	if mode == "" {
		mode = OutputModeContent
	}
	if mode != OutputModeContent && mode != OutputModeCount {
		return "", fmt.Errorf("unsupported output_mode: %s", mode)
	}
	return mode, nil
}

func requiresLineOutput(params *Params) bool {
	return params.LineNumber || params.BeforeContext > 0 || params.AfterContext > 0 || params.Context > 0
}

func buildMatchesFromIndices(indices [][]int, headLimit int, textAt func(start int, end int) string) []Match {
	if len(indices) == 0 {
		return nil
	}

	matches := make([]Match, 0, len(indices))
	for _, idx := range indices {
		matches = append(matches, Match{
			Start: idx[0],
			End:   idx[1],
			Text:  textAt(idx[0], idx[1]),
		})
		if headLimit > 0 && len(matches) >= headLimit {
			break
		}
	}
	return matches
}

func joinMatchText(matches []Match) string {
	out := make([]string, 0, len(matches))
	for _, m := range matches {
		out = append(out, m.Text)
	}
	return strings.Join(out, "\n")
}

// renderLineOutput 将命中区间转换为“按行展示”的输出文本。
func renderLineOutput(source string, indices [][]int, params *Params) string {
	lines, starts := splitLines(source)
	if len(lines) == 0 {
		return ""
	}

	matchLines := collectMatchLines(indices, starts)
	if len(matchLines) == 0 {
		return ""
	}

	before, after := params.BeforeContext, params.AfterContext
	if params.Context > 0 {
		before, after = params.Context, params.Context
	}
	ranges := mergeRanges(matchLines, len(lines), before, after)

	out := make([]string, 0, len(lines))
	for _, r := range ranges {
		for ln := r[0]; ln <= r[1]; ln++ {
			line := lines[ln-1]
			if params.LineNumber {
				out = append(out, fmt.Sprintf("%d:%s", ln, line))
			} else {
				out = append(out, line)
			}

			if params.HeadLimit > 0 && len(out) >= params.HeadLimit {
				return strings.Join(out, "\n")
			}
		}
	}

	return strings.Join(out, "\n")
}

// collectMatchLines 将命中区间映射为去重且升序的行号列表。
func collectMatchLines(indices [][]int, starts []int) []int {
	seen := make(map[int]struct{}, len(indices)*2)
	lines := make([]int, 0, len(indices)*2)

	for _, idx := range indices {
		startLine := lineByOffset(starts, idx[0])
		endOffset := idx[1]
		if endOffset > idx[0] {
			endOffset--
		}
		endLine := lineByOffset(starts, endOffset)
		if endLine < startLine {
			endLine = startLine
		}

		for ln := startLine; ln <= endLine; ln++ {
			if _, ok := seen[ln]; ok {
				continue
			}
			seen[ln] = struct{}{}
			lines = append(lines, ln)
		}
	}

	sort.Ints(lines)
	return lines
}

// compilePattern 基于参数组装正则前缀并编译 coregex。
func compilePattern(pattern string, ignoreCase bool, multiline bool) (*coregex.Regex, error) {
	if strings.TrimSpace(pattern) == "" {
		return nil, errors.New("pattern is required")
	}

	prefix := ""
	if ignoreCase {
		prefix += "(?i)"
	}
	if multiline {
		prefix += "(?s)"
	}

	re, err := coregex.Compile(prefix + pattern)
	if err != nil {
		return nil, fmt.Errorf("invalid pattern: %w", err)
	}
	return re, nil
}

// splitLines 把原始文本拆成“行内容 + 行起始偏移”。
func splitLines(content string) ([]string, []int) {
	if content == "" {
		return nil, nil
	}

	lines := make([]string, 0, 64)
	starts := make([]int, 0, 64)
	start := 0
	for i := 0; i < len(content); i++ {
		if content[i] != '\n' {
			continue
		}
		lines = append(lines, strings.TrimSuffix(content[start:i], "\r"))
		starts = append(starts, start)
		start = i + 1
	}

	if start < len(content) {
		lines = append(lines, strings.TrimSuffix(content[start:], "\r"))
		starts = append(starts, start)
	}

	return lines, starts
}

// lineByOffset 根据字节偏移定位 1-based 行号。
func lineByOffset(starts []int, offset int) int {
	if len(starts) == 0 {
		return 0
	}
	if offset < 0 {
		offset = 0
	}
	if offset <= starts[0] {
		return 1
	}

	idx := sort.Search(len(starts), func(i int) bool {
		return starts[i] > offset
	})
	if idx == 0 {
		return 1
	}
	if idx >= len(starts) {
		return len(starts)
	}
	return idx
}

// mergeRanges 根据命中行与上下文参数合并输出区间。
func mergeRanges(lines []int, total int, before int, after int) [][2]int {
	if len(lines) == 0 || total <= 0 {
		return nil
	}
	if before < 0 {
		before = 0
	}
	if after < 0 {
		after = 0
	}

	ranges := make([][2]int, 0, len(lines))
	for _, line := range lines {
		start := max(1, line-before)
		end := min(total, line+after)

		if len(ranges) == 0 {
			ranges = append(ranges, [2]int{start, end})
			continue
		}

		last := &ranges[len(ranges)-1]
		if start <= last[1]+1 {
			if end > last[1] {
				last[1] = end
			}
			continue
		}
		ranges = append(ranges, [2]int{start, end})
	}
	return ranges
}
