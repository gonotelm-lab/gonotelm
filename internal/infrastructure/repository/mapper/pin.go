package mapper

import (
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	pinentity "github.com/gonotelm-lab/gonotelm/internal/domain/pin/entity"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/database/schema"
)

func UserNotebookPinToSchema(pin *pinentity.UserNotebookPin) *schema.UserNotebookPin {
	return &schema.UserNotebookPin{
		UserId:     pin.UserId,
		NotebookId: pin.NotebookId,
		PinnedAt:   pin.PinnedAt.Value(),
	}
}

func UserNotebookPinFromSchema(pin *schema.UserNotebookPin) *pinentity.UserNotebookPin {
	return &pinentity.UserNotebookPin{
		UserId:     pin.UserId,
		NotebookId: valobj.Id(pin.NotebookId),
		PinnedAt:   valobj.NewTimeFrom(pin.PinnedAt),
	}
}

func UserNotebookPinsFromSchema(pins []*schema.UserNotebookPin) []*pinentity.UserNotebookPin {
	results := make([]*pinentity.UserNotebookPin, 0, len(pins))
	for i := range pins {
		results = append(results, UserNotebookPinFromSchema(pins[i]))
	}
	return results
}
