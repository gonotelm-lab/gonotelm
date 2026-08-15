package entity

import "time"

type Command struct {
	Cwd     string
	Command string
	Env     map[string]string
	Timeout time.Duration
}

type Execution struct {
	// ExitCode 进程退出码；0 表示成功
	ExitCode int
	Stdout   []byte
	Stderr   []byte
	Duration time.Duration
}

func (r Execution) Success() bool { return r.ExitCode == 0 }
