package videooverview

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
)

const (
	videoStoryboardDirName  = "videostoryboard"
	videoStoryboardFileName = "storyboard.md"
)

// storyboardFilePath 返回分镜草稿的固定路径，工具只能读写这一个文件：
//
//	/tmp/{notebookId}/videostoryboard/{artifactId}/storyboard.md
func storyboardFilePath(notebookId, artifactId valobj.Id) string {
	return path.Join(
		"/tmp",
		notebookId.String(),
		videoStoryboardDirName,
		artifactId.String(),
		videoStoryboardFileName,
	)
}

// writeStoryboardFile 覆盖写入整份分镜，返回写入字节数。
func writeStoryboardFile(notebookId, artifactId valobj.Id, content string) (int, error) {
	target := storyboardFilePath(notebookId, artifactId)
	dir := filepath.Dir(target)

	if err := os.MkdirAll(dir, 0o750); err != nil {
		return 0, fmt.Errorf("create storyboard dir failed, dir=%s: %w", dir, err)
	}

	// 先写同目录临时文件再 rename，避免中途失败留下半份稿。
	tmp, err := os.CreateTemp(dir, videoStoryboardFileName+".tmp-*")
	if err != nil {
		return 0, fmt.Errorf("create temp storyboard file failed: %w", err)
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()

	if _, err := tmp.WriteString(content); err != nil {
		_ = tmp.Close()
		return 0, fmt.Errorf("write storyboard file failed: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return 0, fmt.Errorf("close storyboard file failed: %w", err)
	}
	if err := os.Chmod(tmpName, 0o640); err != nil {
		return 0, fmt.Errorf("chmod storyboard file failed: %w", err)
	}
	if err := os.Rename(tmpName, target); err != nil {
		return 0, fmt.Errorf("replace storyboard file failed: %w", err)
	}
	return len(content), nil
}

// editStoryboardFile 在草稿里做精确字符串替换，返回替换次数。
// 默认要求 oldString 唯一命中，避免一次替换误伤多镜；replaceAll 时替换全部。
func editStoryboardFile(
	notebookId, artifactId valobj.Id,
	oldString, newString string,
	replaceAll bool,
) (int, error) {
	if oldString == "" {
		return 0, fmt.Errorf("old_string is required")
	}

	content, err := readStoryboardFile(notebookId, artifactId)
	if err != nil {
		return 0, err
	}
	if content == "" {
		return 0, fmt.Errorf("storyboard file is empty, use WriteStoryboard first")
	}

	matched := strings.Count(content, oldString)
	switch {
	case matched == 0:
		return 0, fmt.Errorf("old_string not found in storyboard file")
	case matched > 1 && !replaceAll:
		return 0, fmt.Errorf(
			"old_string matches %d places, add more surrounding context or set replace_all", matched)
	}

	replaced := 1
	limit := 1
	if replaceAll {
		replaced = matched
		limit = -1
	}

	if _, err := writeStoryboardFile(notebookId, artifactId, strings.Replace(content, oldString, newString, limit)); err != nil {
		return 0, err
	}
	return replaced, nil
}

// readStoryboardFile 读取草稿；文件不存在时返回空串。
func readStoryboardFile(notebookId, artifactId valobj.Id) (string, error) {
	raw, err := os.ReadFile(storyboardFilePath(notebookId, artifactId))
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("read storyboard file failed: %w", err)
	}
	return string(raw), nil
}

// statStoryboardFile 返回草稿文件的大小与是否存在。
func statStoryboardFile(notebookId, artifactId valobj.Id) (int64, bool, error) {
	info, err := os.Stat(storyboardFilePath(notebookId, artifactId))
	if err != nil {
		if os.IsNotExist(err) {
			return 0, false, nil
		}
		return 0, false, fmt.Errorf("stat storyboard file failed: %w", err)
	}
	return info.Size(), true, nil
}

// maxStoryboardLineChars 单行最大返回字符数，与 ReadFile 保持一致。
const maxStoryboardLineChars = 2000

// formatStoryboardLines 按行窗口渲染草稿，格式与 ReadFile 相同：LINE_NUMBER|LINE_CONTENT。
func formatStoryboardLines(content string, offset, limit int) string {
	lines := strings.Split(content, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	start := max(1, offset) - 1
	if start >= len(lines) {
		return ""
	}
	end := len(lines)
	if limit > 0 && start+limit < end {
		end = start + limit
	}

	var b strings.Builder
	b.Grow(512)
	for i := start; i < end; i++ {
		line := lines[i]
		if len(line) > maxStoryboardLineChars {
			line = line[:maxStoryboardLineChars] + "... [line truncated]"
		}
		fmt.Fprintf(&b, "%d|%s\n", i+1, line)
	}
	return b.String()
}

func removeStoryboardFile(notebookId, artifactId valobj.Id) error {
	if err := os.Remove(storyboardFilePath(notebookId, artifactId)); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove storyboard file failed: %w", err)
	}
	return nil
}
