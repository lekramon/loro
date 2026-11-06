// Package jobs shares bounded workers across durable domain-owned job kinds.
package jobs

import (
	"context"
	"sync"
	"time"
)

type Step func(context.Context) (bool, error)

// Run rotates kinds fairly; an idle scheduled job never occupies a worker.
func Run(ctx context.Context, count int, steps ...Step) {
	if len(steps) == 0 {
		return
	}
	var wg sync.WaitGroup
	for worker := 0; worker < count; worker++ {
		wg.Add(1)
		go func(next int) {
			defer wg.Done()
			tick := time.NewTicker(500 * time.Millisecond)
			defer tick.Stop()
			for {
				worked := false
				for range steps {
					if ctx.Err() != nil {
						return
					}
					step := steps[next]
					next = (next + 1) % len(steps)
					worked, _ = step(ctx)
					if worked {
						break
					}
				}
				if !worked {
					select {
					case <-ctx.Done():
						return
					case <-tick.C:
					}
				}
			}
		}(worker % len(steps))
	}
	wg.Wait()
}
