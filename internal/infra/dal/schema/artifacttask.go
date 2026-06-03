package schema

type ArtifactTask struct {
	Id         string `gorm:"column:id"`          // 任务id
	NotebookId string `gorm:"column:notebook_id"` // 笔记本id
	Kind       string `gorm:"column:kind"`        // 任务类型
	Status     string `gorm:"column:status"`      // 任务状态
	Result     []byte `gorm:"column:result"`      // 任务结果
	ResultKind string `gorm:"column:result_kind"` // 任务结果类型
	UserId     string `gorm:"column:user_id"`     // 用户id
	RunId      string `gorm:"column:run_id"`      // 消费者id
	LockNo     int32  `gorm:"column:lock_no"`     // 乐观锁版本号
	Payload    []byte `gorm:"column:payload"`     // 任务输入
	CreatedAt  int64  `gorm:"column:created_at"`  // 任务创建时间
	UpdatedAt  int64  `gorm:"column:updated_at"`  // 任务更新时间
	ExpiredAt  int64  `gorm:"column:expired_at"`  // 任务过期时间
}

func (ArtifactTask) TableName() string {
	return "artifact_tasks"
}

type ArtifactTaskClaimParams struct {
	NewStatus string
	UpdatedAt int64
	RunId     string
}

type ArtifactTaskUpdateStatusParams struct {
	NewStatus string // set status = :new_status
	UpdatedAt int64  // set updated_at = :updated_at
}

type ArtifactTaskUpdateResultParams struct {
	NewStatus  string // set status = :new_status
	Result     []byte // set result = :result
	ResultKind string // set result_kind = :result_kind
	UpdatedAt  int64  // set updated_at = :updated_at
}
