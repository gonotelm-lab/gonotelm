package videooverview

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
	"sync"
	"unicode/utf8"
)

const maxMissingAudioIDsInStatus = 20

var shotAudioIDLinePattern = regexp.MustCompile(`(?m)^-\s*audio_ids?:\s*(.+)$`)

// storyboardDoc 按镜头序号存放草稿。Append/Edit 带 index，可并发写入，落盘时按 index 升序拼接。
type storyboardDoc struct {
	mu       sync.Mutex
	shots    map[int]string // index -> shot markdown
	expected []string       // 口播轨全部 audio_id，按播放顺序
}

type storyboardShotInput struct {
	Index   int
	Content string
}

type storyboardEditOp struct {
	Index   int
	Content string
}

func newStoryboardDoc() *storyboardDoc {
	return &storyboardDoc{shots: make(map[int]string)}
}

func (d *storyboardDoc) SetExpectedAudioIDs(ids []string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.expected = append([]string(nil), ids...)
}

func (d *storyboardDoc) Markdown() string {
	d.mu.Lock()
	defer d.mu.Unlock()
	if len(d.shots) == 0 {
		return ""
	}
	indexes := sortedIndexes(d.shots)
	parts := make([]string, 0, len(indexes))
	for _, idx := range indexes {
		parts = append(parts, d.shots[idx])
	}
	return strings.Join(parts, "\n\n") + "\n"
}

func (d *storyboardDoc) ShotCount() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return len(d.shots)
}

// Len 返回已写入总字节数（日志用）。
func (d *storyboardDoc) Len() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	n := 0
	for _, s := range d.shots {
		n += len(s)
	}
	return n
}

// coverageReport 返回缺失的 audio_id 与 shot index 空洞。
func (d *storyboardDoc) coverageReport() (missingAudio []string, indexGaps []int) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.coverageReportLocked()
}

func (d *storyboardDoc) coverageReportLocked() (missingAudio []string, indexGaps []int) {
	covered := make(map[string]struct{})
	for _, content := range d.shots {
		for _, id := range extractShotAudioIDs(content) {
			if id == "" || id == "none" {
				continue
			}
			covered[id] = struct{}{}
		}
	}
	for _, id := range d.expected {
		if _, ok := covered[id]; !ok {
			missingAudio = append(missingAudio, id)
		}
	}
	indexes := sortedIndexes(d.shots)
	if len(indexes) == 0 {
		return missingAudio, nil
	}
	maxIdx := indexes[len(indexes)-1]
	have := make(map[int]struct{}, len(indexes))
	for _, idx := range indexes {
		have[idx] = struct{}{}
	}
	for i := 1; i <= maxIdx; i++ {
		if _, ok := have[i]; !ok {
			indexGaps = append(indexGaps, i)
		}
	}
	return missingAudio, indexGaps
}

func (d *storyboardDoc) coverageStatusLocked() string {
	if len(d.expected) == 0 {
		return fmt.Sprintf("total %d shots", len(d.shots))
	}
	missing, gaps := d.coverageReportLocked()
	covered := len(d.expected) - len(missing)
	var b strings.Builder
	fmt.Fprintf(&b, "audio coverage %d/%d", covered, len(d.expected))
	if len(missing) > 0 {
		show := missing
		if len(show) > maxMissingAudioIDsInStatus {
			show = show[:maxMissingAudioIDsInStatus]
		}
		fmt.Fprintf(&b, "; missing audio_ids %v", show)
		if len(missing) > maxMissingAudioIDsInStatus {
			fmt.Fprintf(&b, " (+%d more)", len(missing)-maxMissingAudioIDsInStatus)
		}
	}
	if len(gaps) > 0 {
		show := gaps
		if len(show) > maxMissingAudioIDsInStatus {
			show = show[:maxMissingAudioIDsInStatus]
		}
		fmt.Fprintf(&b, "; shot index gaps %v", show)
		if len(gaps) > maxMissingAudioIDsInStatus {
			fmt.Fprintf(&b, " (+%d more)", len(gaps)-maxMissingAudioIDsInStatus)
		}
	}
	fmt.Fprintf(&b, "; total %d shots", len(d.shots))
	return b.String()
}

// checkContinuity 供模型自检：shot index 是否 1..N 连续、口播是否全覆盖、audio 顺序是否合理。
func (d *storyboardDoc) checkContinuity() string {
	d.mu.Lock()
	defer d.mu.Unlock()

	indexes := sortedIndexes(d.shots)
	var b strings.Builder

	if len(indexes) == 0 {
		b.WriteString("FAIL: storyboard is empty\n")
		b.WriteString("action: AppendStoryBoardShot starting from index 1\n")
		return b.String()
	}

	missingAudio, indexGaps := d.coverageReportLocked()
	dupAudio := d.duplicateAudioIDsLocked(indexes)
	orderIssues := d.audioOrderIssuesLocked(indexes)

	ok := len(indexGaps) == 0 &&
		(len(d.expected) == 0 || len(missingAudio) == 0) &&
		len(dupAudio) == 0 &&
		len(orderIssues) == 0

	if ok {
		b.WriteString("PASS: shot indexes contiguous and audio coverage OK\n")
	} else {
		b.WriteString("FAIL: continuity/coverage problems found\n")
	}

	fmt.Fprintf(&b, "shots: %d (indexes %d..%d)\n", len(indexes), indexes[0], indexes[len(indexes)-1])
	if len(d.expected) > 0 {
		fmt.Fprintf(&b, "audio coverage: %d/%d\n", len(d.expected)-len(missingAudio), len(d.expected))
	}

	if len(indexGaps) == 0 {
		b.WriteString("shot index continuity: OK (1..max filled)\n")
	} else {
		show := indexGaps
		if len(show) > maxMissingAudioIDsInStatus {
			show = show[:maxMissingAudioIDsInStatus]
		}
		fmt.Fprintf(&b, "shot index gaps (%d): %v", len(indexGaps), show)
		if len(indexGaps) > maxMissingAudioIDsInStatus {
			fmt.Fprintf(&b, " (+%d more)", len(indexGaps)-maxMissingAudioIDsInStatus)
		}
		b.WriteByte('\n')
		b.WriteString("action: AppendStoryBoardShot with these missing indexes (keep 1..N contiguous)\n")
	}

	if len(d.expected) > 0 {
		if len(missingAudio) == 0 {
			b.WriteString("audio_id coverage: OK\n")
		} else {
			show := missingAudio
			if len(show) > maxMissingAudioIDsInStatus {
				show = show[:maxMissingAudioIDsInStatus]
			}
			fmt.Fprintf(&b, "missing audio_ids (%d): %v", len(missingAudio), show)
			if len(missingAudio) > maxMissingAudioIDsInStatus {
				fmt.Fprintf(&b, " (+%d more)", len(missingAudio)-maxMissingAudioIDsInStatus)
			}
			b.WriteByte('\n')
			b.WriteString("action: Append or Edit shots to cover every missing audio_id (may merge consecutive clips into one shot)\n")
		}
	}

	if len(dupAudio) > 0 {
		show := dupAudio
		if len(show) > maxMissingAudioIDsInStatus {
			show = show[:maxMissingAudioIDsInStatus]
		}
		fmt.Fprintf(&b, "duplicate audio_ids (%d): %v\n", len(dupAudio), show)
		b.WriteString("action: each audio_id should appear in exactly one shot; Edit to remove duplicates\n")
	}

	if len(orderIssues) > 0 {
		show := orderIssues
		if len(show) > 8 {
			show = show[:8]
		}
		fmt.Fprintf(&b, "audio order issues (%d):\n", len(orderIssues))
		for _, issue := range show {
			fmt.Fprintf(&b, "- %s\n", issue)
		}
		if len(orderIssues) > 8 {
			fmt.Fprintf(&b, "- (+%d more)\n", len(orderIssues)-8)
		}
		b.WriteString("action: within/across shots, audio_id must follow narration track order\n")
	}

	return strings.TrimRight(b.String(), "\n") + "\n"
}

func (d *storyboardDoc) duplicateAudioIDsLocked(indexes []int) []string {
	seen := make(map[string]int)
	var dups []string
	dupSet := make(map[string]struct{})
	for _, idx := range indexes {
		for _, id := range extractShotAudioIDs(d.shots[idx]) {
			if id == "" || id == "none" {
				continue
			}
			seen[id]++
			if seen[id] == 2 {
				if _, ok := dupSet[id]; !ok {
					dupSet[id] = struct{}{}
					dups = append(dups, id)
				}
			}
		}
	}
	slices.Sort(dups)
	return dups
}

func (d *storyboardDoc) audioOrderIssuesLocked(indexes []int) []string {
	if len(d.expected) == 0 {
		return nil
	}
	pos := make(map[string]int, len(d.expected))
	for i, id := range d.expected {
		pos[id] = i
	}

	var issues []string
	prevGlobal := -1
	for _, idx := range indexes {
		ids := extractShotAudioIDs(d.shots[idx])
		var spoken []string
		for _, id := range ids {
			if id == "" || id == "none" {
				continue
			}
			spoken = append(spoken, id)
		}
		if len(spoken) == 0 {
			continue
		}
		prevLocal := -1
		for _, id := range spoken {
			p, ok := pos[id]
			if !ok {
				issues = append(issues, fmt.Sprintf("shot %d: unknown audio_id %s (not in narration track)", idx, id))
				continue
			}
			if prevLocal >= 0 && p != prevLocal+1 {
				issues = append(issues, fmt.Sprintf(
					"shot %d: audio_ids not consecutive in track (%s then %s)",
					idx, d.expected[prevLocal], id,
				))
			}
			if prevGlobal >= 0 && p < prevGlobal {
				issues = append(issues, fmt.Sprintf(
					"shot %d: audio_id %s goes backwards vs previous shot order",
					idx, id,
				))
			}
			prevLocal = p
			if p > prevGlobal {
				prevGlobal = p
			}
		}
	}
	return issues
}

// append 按 index 写入一个或多个 shot；同 index 已存在则失败（改用 Edit）。可并发调用。
func (d *storyboardDoc) append(shots []storyboardShotInput) (string, error) {
	if len(shots) == 0 {
		return "", fmt.Errorf("shots is required (one or more)")
	}

	prepared := make([]storyboardShotInput, 0, len(shots))
	seen := make(map[int]struct{}, len(shots))
	for i, raw := range shots {
		if raw.Index < 1 {
			return "", fmt.Errorf("shots[%d].index must be >= 1", i)
		}
		if _, dup := seen[raw.Index]; dup {
			return "", fmt.Errorf("duplicate index %d in shots", raw.Index)
		}
		seen[raw.Index] = struct{}{}

		content, err := normalizeShotContent(raw.Content)
		if err != nil {
			return "", fmt.Errorf("shots[%d] (index=%d): %w", i, raw.Index, err)
		}
		prepared = append(prepared, storyboardShotInput{Index: raw.Index, Content: content})
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	if d.shots == nil {
		d.shots = make(map[int]string)
	}
	for _, s := range prepared {
		if _, exists := d.shots[s.Index]; exists {
			return "", fmt.Errorf("index %d already exists; use EditStoryBoardShot to replace", s.Index)
		}
	}
	indexes := make([]int, 0, len(prepared))
	for _, s := range prepared {
		d.shots[s.Index] = s.Content
		indexes = append(indexes, s.Index)
	}
	slices.Sort(indexes)

	status := d.coverageStatusLocked()
	if len(indexes) == 1 {
		return fmt.Sprintf(
			"OK appended shot index %d (%d runes); %s",
			indexes[0],
			utf8.RuneCountInString(prepared[0].Content),
			status,
		), nil
	}
	return fmt.Sprintf(
		"OK appended shot indexes %v (%d shots); %s",
		indexes, len(indexes), status,
	), nil
}

// edit 按镜头序号批量替换/删除；空 content 表示删除。
func (d *storyboardDoc) edit(ops []storyboardEditOp) (string, error) {
	if len(ops) == 0 {
		return "", fmt.Errorf("edits is required (one or more)")
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	if d.shots == nil {
		d.shots = make(map[int]string)
	}

	pending := make(map[int]string, len(ops))
	for i, op := range ops {
		if op.Index < 1 {
			return "", fmt.Errorf("edits[%d].index must be >= 1", i)
		}
		if _, exists := d.shots[op.Index]; !exists {
			return "", fmt.Errorf("edits[%d].index %d not found", i, op.Index)
		}
		if _, dup := pending[op.Index]; dup {
			return "", fmt.Errorf("duplicate index %d in edits", op.Index)
		}
		content := strings.TrimSpace(op.Content)
		if content == "" {
			pending[op.Index] = "" // delete
			continue
		}
		normalized, err := normalizeShotContent(op.Content)
		if err != nil {
			return "", fmt.Errorf("edits[%d].content: %w", i, err)
		}
		pending[op.Index] = normalized
	}

	replaced, deleted := 0, 0
	for idx, content := range pending {
		if content == "" {
			delete(d.shots, idx)
			deleted++
			continue
		}
		d.shots[idx] = content
		replaced++
	}

	return fmt.Sprintf(
		"OK edited: replaced %d, deleted %d; %s",
		replaced, deleted, d.coverageStatusLocked(),
	), nil
}

// read 按镜头序号读取。indexes 非空时按给定序号（升序输出）；否则对已排序序号做 offset/limit。
func (d *storyboardDoc) read(offset, limit int, indexes []int) (string, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if len(d.shots) == 0 {
		return "(empty storyboard)", nil
	}

	sorted := sortedIndexes(d.shots)
	total := len(sorted)
	var selected []int

	if len(indexes) > 0 {
		seen := make(map[int]struct{}, len(indexes))
		for _, idx := range indexes {
			if _, ok := d.shots[idx]; !ok {
				return "", fmt.Errorf("index %d not found; have %v", idx, sorted)
			}
			if _, ok := seen[idx]; ok {
				continue
			}
			seen[idx] = struct{}{}
			selected = append(selected, idx)
		}
		slices.Sort(selected)
	} else {
		start := max(1, offset) - 1
		if start >= total {
			return "", nil
		}
		end := total
		if limit > 0 && start+limit < end {
			end = start + limit
		}
		selected = sorted[start:end]
	}

	var out strings.Builder
	out.Grow(256)
	for i, idx := range selected {
		fmt.Fprintf(&out, "### Shot index=%d (%d/%d)\n%s\n\n",
			idx, i+1, total, d.shots[idx])
	}
	return strings.TrimRight(out.String(), "\n") + "\n", nil
}

func normalizeShotContent(raw string) (string, error) {
	s := strings.ReplaceAll(raw, "\r\n", "\n")
	s = strings.TrimSpace(s)
	if s == "" {
		return "", fmt.Errorf("content is required")
	}
	return s, nil
}

func extractShotAudioIDs(content string) []string {
	matches := shotAudioIDLinePattern.FindAllStringSubmatch(content, -1)
	if len(matches) == 0 {
		return nil
	}
	out := make([]string, 0, len(matches))
	for _, m := range matches {
		if len(m) < 2 {
			continue
		}
		raw := strings.TrimSpace(m[1])
		raw = strings.Trim(raw, "`")
		raw = strings.TrimSpace(strings.Trim(raw, "[]"))
		for _, part := range strings.Split(raw, ",") {
			id := strings.TrimSpace(part)
			id = strings.Trim(id, "`\"'")
			if id != "" {
				out = append(out, id)
			}
		}
	}
	return out
}

func sortedIndexes(shots map[int]string) []int {
	out := make([]int, 0, len(shots))
	for idx := range shots {
		out = append(out, idx)
	}
	slices.Sort(out)
	return out
}

func expectedAudioIDsFromMeta(meta *audioCheckpointMeta) []string {
	if meta == nil {
		return nil
	}
	parts := meta.sortedAudioParts()
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		out = append(out, fmt.Sprintf("audio_%d-%d", p.SegmentIndex, p.LineIndex))
	}
	return out
}
