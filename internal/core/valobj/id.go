package valobj

import "github.com/gonotelm-lab/gonotelm/pkg/uuid"

// General purpose unique id
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

// IdsToStrings 将 Id 列表转换为字符串列表
func IdsToStrings(ids []Id) []string {
	if len(ids) == 0 {
		return nil
	}

	result := make([]string, 0, len(ids))
	for _, id := range ids {
		result = append(result, id.String())
	}

	return result
}

// StringsToIds 将字符串列表转换为 Id 列表
func StringsToIds(strs []string) ([]Id, error) {
	if len(strs) == 0 {
		return nil, nil
	}

	result := make([]Id, 0, len(strs))
	for _, s := range strs {
		id, err := NewIdFromString(s)
		if err != nil {
			return nil, err
		}
		result = append(result, id)
	}

	return result, nil
}
