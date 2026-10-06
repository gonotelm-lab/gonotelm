package schema

type ArtifactStylePreview struct {
	Id               int64  `gorm:"column:id;primaryKey;autoIncrement"`
	StoreKey         string `gorm:"column:store_key"`
	Identifier       string `gorm:"column:identifier"`
	Md5sum           []byte `gorm:"column:md5sum"`
	OriginalFilename string `gorm:"column:original_filename"`
	CreatedAt        int64  `gorm:"column:created_at"`
	UpdatedAt        int64  `gorm:"column:updated_at"`
}

func (ArtifactStylePreview) TableName() string { return "artifact_style_previews" }
