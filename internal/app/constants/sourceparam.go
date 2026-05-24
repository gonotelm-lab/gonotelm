package constants

const (
	// 一篇来源最大允许token数量
	MaxSourceTextContentToken = 1_000_000 // 1000k

	// 文本来源最大允许字符数
	MaxSourceTextContentLength = 50_000 // 50k

	MaxSourceCountPerNotebook = 50

	// 文件来源最大允许大小 100MB
	MaxSourceFileSizeBytes = 100 * 1024 * 1024
)
