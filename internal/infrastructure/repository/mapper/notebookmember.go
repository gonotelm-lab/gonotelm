package mapper

import (
	"github.com/gonotelm-lab/gonotelm/internal/core/entity"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	notebookentity "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/entity"
	"github.com/gonotelm-lab/gonotelm/internal/domain/notebook/vo"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/database/schema"
)

func NotebookMemberToSchema(member *notebookentity.NotebookMember) *schema.NotebookMember {
	return &schema.NotebookMember{
		Id:         member.Id,
		NotebookId: member.NotebookId,
		UserId:     member.UserId,
		Role:       string(member.Role),
		CreatedAt:  member.CreateTime.Value(),
		UpdatedAt:  member.UpdateTime.Value(),
	}
}

func NotebookMemberFromSchema(member *schema.NotebookMember) *notebookentity.NotebookMember {
	return &notebookentity.NotebookMember{
		Base: entity.Base{
			Id:         valobj.Id(member.Id),
			CreateTime: valobj.NewTimeFrom(member.CreatedAt),
			UpdateTime: valobj.NewTimeFrom(member.UpdatedAt),
		},
		NotebookId: valobj.Id(member.NotebookId),
		UserId:     member.UserId,
		Role:       vo.Role(member.Role),
	}
}

func NotebookMembersFromSchema(members []*schema.NotebookMember) []*notebookentity.NotebookMember {
	results := make([]*notebookentity.NotebookMember, 0, len(members))
	for i := range members {
		results = append(results, NotebookMemberFromSchema(members[i]))
	}
	return results
}
