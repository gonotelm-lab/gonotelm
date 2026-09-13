package videooverview

import (
	"bytes"
	"context"
	"fmt"
	"path"
	"strings"

	"github.com/gabriel-vasile/mimetype"
	"github.com/gonotelm-lab/gonotelm/internal/domain/sandbox/entity"
)

const mimeTypeMP4 = "video/mp4"

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func checkMP4ArtifactValid(ctx context.Context, sandbox entity.Sandbox, filePath string) error {
	fileContent, err := sandbox.ReadFile(ctx, filePath, entity.WithReadMaxBytes(1024))
	if err != nil {
		return fmt.Errorf("can not read mp4 file: %s, err: %w", filePath, err)
	}

	mimeType, err := mimetype.DetectReader(bytes.NewBuffer(fileContent))
	if err != nil {
		return fmt.Errorf("can not detect mime type of file: %s, err: %w", filePath, err)
	}

	splits := strings.Split(mimeType.String(), ";")
	if len(splits) < 1 {
		return fmt.Errorf("can not detect mime type of file: %s, len(splits) less than 1", filePath)
	}

	detected := strings.TrimSpace(splits[0])
	if detected != mimeTypeMP4 {
		return fmt.Errorf("file %s is not valid mp4, but %s", filePath, detected)
	}
	return nil
}

func audioFileName(segmentIndex, lineIndex int) string {
	return fmt.Sprintf("audio_%d-%d.wav", segmentIndex, lineIndex)
}

func audioSandboxPath(workspaceDir string, segmentIndex, lineIndex int) string {
	return path.Join(workspaceDir, "audio", audioFileName(segmentIndex, lineIndex))
}

func renderAudioManifestMarkdown(workspaceDir string, meta *audioCheckpointMeta) string {
	parts := meta.sortedAudioParts()
	if len(parts) == 0 {
		return "(no narration audio)"
	}

	var b strings.Builder
	b.WriteString("Files are already under Workspace `audio/`. Durations are authoritative.\n\n")
	for _, p := range parts {
		id := fmt.Sprintf("audio_%d-%d", p.SegmentIndex, p.LineIndex)
		rel := path.Join("audio", audioFileName(p.SegmentIndex, p.LineIndex))
		abs := audioSandboxPath(workspaceDir, p.SegmentIndex, p.LineIndex)
		fmt.Fprintf(&b, "- **audio_id**: `%s`\n", id)
		fmt.Fprintf(&b, "  - file: `%s`\n", rel)
		fmt.Fprintf(&b, "  - absolute: `%s`\n", abs)
		fmt.Fprintf(&b, "  - duration_ms: %d\n", p.DurationMs)
		fmt.Fprintf(&b, "  - duration_s: %.3f\n", float64(p.DurationMs)/1000.0)
		fmt.Fprintf(&b, "  - text: %s\n\n", strings.TrimSpace(p.Text))
	}
	return strings.TrimSpace(b.String())
}
