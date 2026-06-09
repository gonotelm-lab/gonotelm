package chat

import (
	"strings"

	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
)

type MessageV2 struct {
	Id        Id
	ChatId    Id
	UserId    string
	Role      MessageRole
	SeqNo     int64
	Fragments []*Fragment
}

type FragmentType string

func (f FragmentType) IsTool() bool {
	return strings.HasPrefix(string(f), string(FragmentTypeToolPrefix))
}

func (f FragmentType) String() string {
	return string(f)
}

const (
	FragmentTypeReasoning FragmentType = "REASONING" // thinking
	FragmentTypeAnswer    FragmentType = "ANSWER"

	// 所有工具类型的fragment都需要这个前缀
	FragmentTypeToolPrefix string = "TOOL_"

	FragmentTypeToolRetrieve FragmentType = "TOOL_RETRIEVE"
)

type FragmentStatus string

const (
	FragmentStatusWIP  FragmentStatus = "WIP" // Work In Progress
	FragmentStatusDone FragmentStatus = "DONE"
)

type Fragment struct {
	Id     int64          `json:"id"` // 本地自增id
	Type   FragmentType   `json:"type"`
	Status FragmentStatus `json:"status"`

	// REASONING / ANSWER
	Content *FragmentContent `json:"content"`

	// 工具输出输出
	ToolInput  *ToolInput  `json:"tool_input,omitempty"`
	ToolResult *ToolResult `json:"tool_result,omitempty"`
}

type ContentType string

const (
	ContentTypeText  ContentType = "TEXT"
	ContentTypeImage ContentType = "IMAGE"
)

// 支持多模态
type FragmentContent struct {
	Type ContentType `json:"type"`

	// 按照Type赋值下列某个字段之一
	Text  *ContentText  `json:"text,omitempty"`
	Image *ContentImage `json:"image,omitempty"`
}

type ContentText struct {
	Data string `json:"data"`
}

type ContentImage struct {
	StoreKey string `json:"store_key,omitempty"`
	Format   string `json:"format"` // image/png, image/jpeg ...

	DataUrl string `json:"data_url,omitempty"`
}

// 工具是输入参数
type ToolInput struct {
	// 对应TOOL_RETRIEVE
	Retrieve *ToolRetrieveInput `json:"retrieve,omitempty"`
}

// 工具的结果
type ToolResult struct {
	// 对应TOOL_RETRIEVE
	Retrieve *ToolRetrieveResult `json:"retrieve,omitempty"`
}

type ToolRetrieveInput struct {
	Sids  []uuid.UUID `json:"sids"`  // source ids
	Query string      `json:"query"` // target query text
}

type ToolRetrieveResult struct{}
