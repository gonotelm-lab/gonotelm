package idgen

import (
	"fmt"
	"sync/atomic"
	"testing"
)

func BenchmarkGet_SingleKey(b *testing.B) {
	key := b.Name()
	Get(key) // warm up

	b.ResetTimer()
	for range b.N {
		Get(key)
	}
}

func BenchmarkGet_NewKeyEachTime(b *testing.B) {
	prefix := b.Name()

	b.ResetTimer()
	for i := range b.N {
		Get(fmt.Sprintf("%s-%d", prefix, i))
	}
}

func BenchmarkGet_RotatingKeys(b *testing.B) {
	const keyCount = 1024
	keys := make([]string, keyCount)
	for i := range keyCount {
		keys[i] = fmt.Sprintf("%s-%d", b.Name(), i)
	}

	b.ResetTimer()
	for i := range b.N {
		Get(keys[i%keyCount])
	}
}

func BenchmarkGet_ParallelSingleKey(b *testing.B) {
	key := b.Name()
	Get(key)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			Get(key)
		}
	})
}

func BenchmarkGet_ParallelRotatingKeys(b *testing.B) {
	const keyCount = 1024
	keys := make([]string, keyCount)
	for i := range keyCount {
		keys[i] = fmt.Sprintf("%s-%d", b.Name(), i)
	}

	var seq atomic.Uint64

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			i := seq.Add(1)
			Get(keys[i%keyCount])
		}
	})
}

func BenchmarkGet_WithLRUEviction(b *testing.B) {
	const cap = 1024
	SetMaxKeys(cap)
	b.Cleanup(func() { SetMaxKeys(defaultMaxKeys) })

	prefix := b.Name()
	for i := range cap {
		Get(fmt.Sprintf("%s-seed-%d", prefix, i))
	}

	b.ResetTimer()
	for i := range b.N {
		Get(fmt.Sprintf("%s-%d", prefix, i))
	}
}
