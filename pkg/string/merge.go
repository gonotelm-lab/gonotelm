package string

import (
	"strings"

	"github.com/gonotelm-lab/gonotelm/pkg/token"
)

// 将多个chunk合并 合并后的每个chunk的token大概为approxChunkLength
func MergeChunks(chunks []string, approxChunkLength int) []string {
	if len(chunks) == 0 {
		return nil
	}

	if approxChunkLength <= 0 {
		return chunks
	}

	merged := make([]string, 0, len(chunks))
	start := 0
	currentToken := token.EstimateToken(chunks[0])
	currentBytes := len(chunks[0])

	for i := 1; i < len(chunks); i++ {
		chunk := chunks[i]
		chunkToken := token.EstimateToken(chunk)

		if currentToken+chunkToken <= approxChunkLength {
			currentToken += chunkToken
			currentBytes += len(chunk)
			continue
		}

		merged = appendMergedChunkGroup(merged, chunks[start:i], currentBytes)

		start = i
		currentToken = chunkToken
		currentBytes = len(chunk)
	}

	merged = appendMergedChunkGroup(merged, chunks[start:], currentBytes)
	return merged
}

func appendMergedChunkGroup(dst []string, group []string, groupBytes int) []string {
	switch len(group) {
	case 0:
		return dst
	case 1:
		return append(dst, group[0])
	}

	var builder strings.Builder
	builder.Grow(groupBytes)
	for _, chunk := range group {
		builder.WriteString(chunk)
	}

	return append(dst, builder.String())
}
