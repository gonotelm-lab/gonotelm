package schema

import "github.com/gonotelm-lab/gonotelm/pkg/uuid"

type ArtifactTask struct {
	Id         uuid.UUID `gorm:"column:id"`          // 任务id
	NotebookId uuid.UUID `gorm:"column:notebook_id"` // 笔记本id
	Kind       string    `gorm:"column:kind"`        // 任务类型
	Status     string    `gorm:"column:status"`      // 任务状态
	Result     []byte    `gorm:"column:result"`      // 任务结果
	ResultKind string    `gorm:"column:result_kind"` // 任务结果类型
	Title      string    `gorm:"column:title"`       // 生成的任务标题
	UserId     string    `gorm:"column:user_id"`     // 用户id
	RunId      string    `gorm:"column:run_id"`      // 消费者id
	LockNo     int32     `gorm:"column:lock_no"`     // 乐观锁版本号
	Payload    []byte    `gorm:"column:payload"`     // 任务输入
	CreatedAt  int64     `gorm:"column:created_at"`  // 任务创建时间
	UpdatedAt  int64     `gorm:"column:updated_at"`  // 任务更新时间
	ExpiredAt  int64     `gorm:"column:expired_at"`  // 任务过期时间
}

func (ArtifactTask) TableName() string {
	return "artifact_tasks"
}

type ArtifactTaskClaimParams struct {
	NewStatus string
	UpdatedAt int64
	RunId     string
	Mode      uint8 // 0-skip lock mode, 1-version lock mode
}

type ArtifactTaskUpdateStatusParams struct {
	NewStatus string // set status = :new_status
	UpdatedAt int64  // set updated_at = :updated_at
}

type ArtifactTaskUpdateResultParams struct {
	NewStatus  string // set status = :new_status
	Title      string // set title = :title
	Result     []byte // set result = :result
	ResultKind string // set result_kind = :result_kind
	UpdatedAt  int64  // set updated_at = :updated_at
}
