package entity

import "time"

type Spec struct {
	// TTL 沙箱最长存活时间
	TTL time.Duration

	// 可注入的环境变量
	Env map[string]string
}
