package schema

// 沙箱的元信息描述
type SandboxDescription struct {
	Id      string     `json:"id"      msgpack:"id"`
	Key     SandboxKey `json:"key"     msgpack:"key"`
	Runtime string     `json:"runtime" msgpack:"runtime"`
}

type SandboxKey struct {
	UserId     string `json:"user_id"     msgpack:"user_id"`
	NotebookId string `json:"notebook_id" msgpack:"notebook_id"`
}
