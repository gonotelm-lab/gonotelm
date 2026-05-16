package string

import (
	"strings"
	"testing"

	"github.com/gonotelm-lab/gonotelm/pkg/token"
)

var benchmarkMergeChunksSink []string

func BenchmarkMergeChunksByTotalTextLength(b *testing.B) {
	const chunkLen = 8000
	unit := benchmarkChunk(chunkLen)
	approx := token.EstimateToken(unit) * 8
	if approx <= 0 {
		approx = 1
	}

	cases := []struct {
		name     string
		totalLen int
	}{
		{name: "TotalLen1W", totalLen: 10_000},
		{name: "TotalLen10W", totalLen: 100_000},
		{name: "TotalLen50W", totalLen: 500_000},
		{name: "TotalLen100W", totalLen: 1_000_000},
	}

	for _, tc := range cases {
		tc := tc
		chunks := benchmarkChunksByTotalLen(tc.totalLen, chunkLen)
		totalLen := totalStringLength(chunks)
		if totalLen != tc.totalLen {
			b.Fatalf("invalid benchmark data: expect totalLen=%d, got %d", tc.totalLen, totalLen)
		}

		b.Run(tc.name, func(b *testing.B) {
			b.SetBytes(int64(tc.totalLen))
			b.ReportAllocs()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				benchmarkMergeChunksSink = MergeChunks(chunks, approx)
			}
		})
	}
}

func benchmarkChunk(chunkLen int) string {
	const seed = "Rust ownership and borrowing with lifetimes. "
	return strings.Repeat(seed, chunkLen/len(seed)+1)[:chunkLen]
}

func benchmarkChunksByTotalLen(totalLen int, chunkLen int) []string {
	if totalLen <= 0 {
		return nil
	}

	if chunkLen <= 0 {
		chunkLen = totalLen
	}

	full := benchmarkChunk(chunkLen)
	chunkCount := (totalLen + chunkLen - 1) / chunkLen
	chunks := make([]string, 0, chunkCount)

	remain := totalLen
	for remain > 0 {
		if remain >= chunkLen {
			chunks = append(chunks, full)
			remain -= chunkLen
			continue
		}

		chunks = append(chunks, full[:remain])
		remain = 0
	}

	return chunks
}

func totalStringLength(chunks []string) int {
	total := 0
	for _, chunk := range chunks {
		total += len(chunk)
	}
	return total
}
