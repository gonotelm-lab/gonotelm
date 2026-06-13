package deptest

import (
	"math/rand"
	"sync"
	"testing"
	"time"

	"golang.org/x/sync/semaphore"
)

func TestSemaphore(t *testing.T) {
	sem := semaphore.NewWeighted(5)

	wg := sync.WaitGroup{}
	for idx := range 20 {
		wg.Go(func() {
			err := sem.Acquire(t.Context(), 1)
			if err != nil {
				t.Logf("acquire semaphore failed, idx=%d", idx)
				return
			}

			// random sleep
			t.Logf("acquired semaphore, idx=%d", idx)
			time.Sleep(time.Duration(rand.Intn(5000)) * time.Millisecond)
			sem.Release(1)
			t.Logf("release semaphore, idx=%d", idx)
		})
	}

	wg.Wait()
}
