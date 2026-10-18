package flow

import "time"

// Config 是 flow 客户端的配置，对应 etc/*.toml.tpl 的 [flow] 段。
//
// task 与 schedule 两个客户端共用一份：各自只取自己用得到的字段，这样 bootstrap
// 里映射一次就能同时喂给它们。
type Config struct {
	// Addr 是 flow-server 的 gRPC 地址。
	Addr string
	// Namespace 是本项目占用的 namespace，namespace 不存在时 flow-server 会拒绝提交。
	Namespace string
	// APIKey 是该 namespace 的 api_key：flow-server 打开 FLOW_API_AUTH_ENABLED 后必填，
	// 关闭时带上也无害（SDK 只在非空时才发凭据头）。
	APIKey string
	// MaxRetry 是提交任务的最大重试次数（不含首次尝试），<=0 时用服务端默认值。
	// 只有 task 客户端用。
	MaxRetry int
	// DialTimeout 暂未使用：v2 用 grpc.NewClient 惰性建连，没有「拨号超时」这个语义。
	DialTimeout time.Duration
}
