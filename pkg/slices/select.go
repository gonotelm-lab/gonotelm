package slices

func Select[T any](slice []T, indices []int) []T {
	selected := make([]T, 0, len(indices))
	for _, index := range indices {
		if index < len(slice) && index >= 0 {
			selected = append(selected, slice[index])
		}
	}
	
	return selected
}
