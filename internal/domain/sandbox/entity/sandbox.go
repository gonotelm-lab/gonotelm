package entity

import (
	"context"
	"io"
	"path"
	"time"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
)

type sandBoxOption struct {
	Bytes int64 // 最多读取的字节数
}

type SandboxOption func(o *sandBoxOption)

func WithReadMaxBytes(bytes int64) SandboxOption {
	return func(o *sandBoxOption) {
		o.Bytes = bytes
	}
}

func GetSandboxOption(opts ...SandboxOption) sandBoxOption {
	o := sandBoxOption{}
	for _, opt := range opts {
		opt(&o)
	}

	return o
}

// 通用化沙箱抽象
type Sandbox interface {
	Id() string

	// Run 同步执行一条命令，返回完整结果。
	Run(ctx context.Context, cmd Command) (Execution, error)

	// WriteFile 写入文件
	WriteFile(ctx context.Context, path string, data io.Reader) error

	// ReadFile 读取文件全部内容
	ReadFile(ctx context.Context, path string, opts ...SandboxOption) ([]byte, error)

	// ReadFile2 读取全部内容 但是返回值不同
	ReadFile2(ctx context.Context, path string, opts ...SandboxOption) (io.ReadCloser, error)

	// 列出目录情况
	ListDir(ctx context.Context, path string) ([]ListDirItem, error)

	// 编辑文件
	EditFile(ctx context.Context, path, old, new string) error

	// 每个沙箱的metadata描述 包含Runtime等
	Description() SandboxDescription
}

type ListDirItem struct {
	Path       string
	Type       string // dir or regular file
	Size       int64
	ModifiedAt time.Time
	CreatedAt  time.Time
}

// 描述沙箱的元信息
type SandboxDescription struct {
	Id      string
	Key     SandboxKey
	Runtime string // os runtime
}

type SandboxKey struct {
	UserId     valobj.Uid
	NotebookId valobj.Id
}

func (k *SandboxKey) WorkspaceDir() string {
	return path.Join("/tmp", k.UserId.String(), k.NotebookId.String())
}

func (k *SandboxKey) String() string {
	if k == nil {
		return ""
	}
	return "uid:" + k.UserId.String() + ", notebookId:" + k.NotebookId.String()
}

type Spec struct {
	TTL time.Duration
	Env map[string]string
}
