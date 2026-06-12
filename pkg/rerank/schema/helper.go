package schema

import "fmt"

// NormalizeTopN 统一处理 topN 语义：
//   - topN == 0 → 使用全部文档
//   - topN < 0  → 报错
//   - topN > docLen → 取 docLen
func NormalizeTopN(topN int, docLen int) (int, error) {
	if docLen <= 0 {
		return 0, fmt.Errorf("documents must not be empty")
	}
	if topN < 0 {
		return 0, fmt.Errorf("top_n must not be negative")
	}
	if topN == 0 {
		return docLen, nil
	}
	if topN > docLen {
		return docLen, nil
	}
	return topN, nil
}
