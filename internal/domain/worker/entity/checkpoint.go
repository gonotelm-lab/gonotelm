package entity

import (
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
)

type Checkpoint struct {
	ArtifactId valobj.Id
	Field1     []byte
	Field2     []byte
	Field3     []byte
	Field4     []byte
	Field5     []byte
	Field6     []byte
	Field7     []byte
	Field8     []byte
	UpdateTime valobj.Time
	CreateTime valobj.Time
}

func NewCheckpoint(artifactId valobj.Id) *Checkpoint {
	return &Checkpoint{
		ArtifactId: artifactId,
		UpdateTime: valobj.NewTime(),
		CreateTime: valobj.NewTime(),
	}
}

func (c *Checkpoint) UpdateField1(field1 []byte) {
	c.Field1 = field1
	c.UpdateTime = valobj.NewTime()
}

func (c *Checkpoint) UpdateField2(field2 []byte) {
	c.Field2 = field2
	c.UpdateTime = valobj.NewTime()
}

func (c *Checkpoint) UpdateField3(field3 []byte) {
	c.Field3 = field3
	c.UpdateTime = valobj.NewTime()
}

func (c *Checkpoint) UpdateField4(field4 []byte) {
	c.Field4 = field4
	c.UpdateTime = valobj.NewTime()
}

func (c *Checkpoint) UpdateField5(field5 []byte) {
	c.Field5 = field5
	c.UpdateTime = valobj.NewTime()
}

func (c *Checkpoint) UpdateField6(field6 []byte) {
	c.Field6 = field6
	c.UpdateTime = valobj.NewTime()
}

func (c *Checkpoint) UpdateField7(field7 []byte) {
	c.Field7 = field7
	c.UpdateTime = valobj.NewTime()
}

func (c *Checkpoint) UpdateField8(field8 []byte) {
	c.Field8 = field8
	c.UpdateTime = valobj.NewTime()
}
