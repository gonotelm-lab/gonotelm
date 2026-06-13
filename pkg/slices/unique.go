package slices

func Unique[T comparable](slice []T) []T {
	seen := make(map[T]struct{})
	result := make([]T, 0, len(slice))
	for _, v := range slice {
		if _, ok := seen[v]; !ok {
			seen[v] = struct{}{}
			result = append(result, v)
		}
	}
	return result
}

// 按照自定义的方式去重
func UniqueyFn[T any, K comparable](slice []T, fn func(T) K) []T {
	seen := make(map[K]struct{})
	result := make([]T, 0, len(slice))
	for _, v := range slice {
		key := fn(v)
		if _, ok := seen[key]; !ok {
			seen[key] = struct{}{}
			result = append(result, v)
		}
	}

	return result
}

func UniqueCount[T comparable](slice []T) int {
	seen := make(map[T]struct{})
	for _, v := range slice {
		seen[v] = struct{}{}
	}
	return len(seen)
}
