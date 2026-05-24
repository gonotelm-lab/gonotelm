package slices

import (
	"golang.org/x/exp/constraints"
)

func CastFloat[T, S constraints.Float](s []T) []S {
	result := make([]S, len(s))
	for i, v := range s {
		result[i] = S(v)
	}
	return result
}
