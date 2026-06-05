package slices

func FromSingle[T any](v T) []T {
	return []T{v}
}
