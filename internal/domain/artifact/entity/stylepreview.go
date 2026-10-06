package entity

import "github.com/gonotelm-lab/gonotelm/internal/core/valobj"

type StylePreviewIdentifier string

func (i StylePreviewIdentifier) String() string {
	return string(i)
}

const (
	StylePreviewSlidesDefault     StylePreviewIdentifier = "slidesstyle.default"
	StylePreviewSlidesCute        StylePreviewIdentifier = "slidesstyle.cute"
	StylePreviewSlidesEducational StylePreviewIdentifier = "slidesstyle.educational"
)

type StylePreview struct {
	StoreKey         valobj.StoreKey
	Identifier       StylePreviewIdentifier
	Md5sum           []byte
	OriginalFilename string
	CreateTime       valobj.Time
	UpdateTime       valobj.Time
}

func NewStylePreview(identifier StylePreviewIdentifier, storeKey valobj.StoreKey) *StylePreview {
	now := valobj.NewTime()
	return &StylePreview{
		StoreKey:   storeKey,
		Identifier: identifier,
		CreateTime: now,
		UpdateTime: now,
	}
}

func (p *StylePreview) UpdateAsset(storeKey valobj.StoreKey, md5sum []byte, originalFilename string) {
	p.StoreKey = storeKey
	p.Md5sum = md5sum
	p.OriginalFilename = originalFilename
	p.UpdateTime = valobj.NewTime()
}
