package slides

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"github.com/gonotelm-lab/gonotelm/internal/domain/sandbox/entity"
	sourceentitiy "github.com/gonotelm-lab/gonotelm/internal/domain/source/entity"

	"github.com/gabriel-vasile/mimetype"
)

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// 检查agent生成的pptx的产物是否和格式正确的pptx文件
func (g *Generator) checkPPTXArtifactValid(ctx context.Context, sandbox entity.Sandbox, path string) error {
	fileContent, err := sandbox.ReadFile(ctx, path, entity.WithReadMaxBytes(4096)) // 读4KB
	if err != nil {
		return fmt.Errorf("can not read pptx file: %s, err: %w", path, err)
	}

	mimeType, err := mimetype.DetectReader(bytes.NewBuffer(fileContent))
	if err != nil {
		return fmt.Errorf("can not detect mime type of file: %s, err: %w", path, err)
	}

	splits := strings.Split(mimeType.String(), ";")
	if len(splits) < 1 {
		return fmt.Errorf("can not detect mime type of file: %s, len(splits) less than 1", path)
	}

	detected := strings.TrimSpace(splits[0])
	if detected != sourceentitiy.MimeTypePPTX {
		return fmt.Errorf("file %s is not valid pptx, but %s", path, detected)
	}

	return nil
}
