package entity

import (
	"context"
	"io"
)

// 通用化沙箱抽象
type Sandbox interface {
	Id() string

	// Run 同步执行一条命令，返回完整结果。
	Run(ctx context.Context, cmd Command) (Execution, error)

	// WriteFile 写入文件
	WriteFile(ctx context.Context, path string, data io.Reader) error

	// ReadFile 读取文件全部内容
	ReadFile(ctx context.Context, path string) ([]byte, error)
}
