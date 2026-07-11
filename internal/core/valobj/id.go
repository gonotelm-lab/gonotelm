package valobj

import "github.com/gonotelm-lab/gonotelm/pkg/uuid"

type Id = uuid.UUID

func NewId() Id {
	return uuid.NewV7()
}

func NewUnOrderedId() Id {
	return uuid.NewV4()
}

func NewIdFromString(s string) (Id, error) {
	return uuid.ParseString(s)
}
