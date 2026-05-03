package slices

func AsSet[T comparable](slice []T) map[T]struct{} {
	seen := make(map[T]struct{})
	for _, v := range slice {
		seen[v] = struct{}{}
	}
	return seen
}

func AsSetF[T any, K comparable](slice []T, fn func(T) K) map[K]struct{} {
	seen := make(map[K]struct{})
	for _, v := range slice {
		seen[fn(v)] = struct{}{}
	}

	return seen
}

func AsMap[T comparable](slice []T) map[T]T {
	out := make(map[T]T)
	for _, v := range slice {
		out[v] = v
	}
	return out
}

func AsMapF[T any, K comparable](slice []T, fn func(T) K) map[K]T {
	out := make(map[K]T)
	for _, v := range slice {
		out[fn(v)] = v
	}

	return out
}
