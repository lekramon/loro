package jobs

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func TestPoolSharesLimitAndDoesNotStarveKinds(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var active, peak, a, b atomic.Int32
	release := make(chan struct{})
	entered := make(chan struct{}, 2)
	step := func(count *atomic.Int32) Step {
		return func(ctx context.Context) (bool, error) {
			n := active.Add(1)
			defer active.Add(-1)
			for {
				old := peak.Load()
				if old >= n || peak.CompareAndSwap(old, n) {
					break
				}
			}
			count.Add(1)
			select {
			case entered <- struct{}{}:
			default:
			}
			select {
			case <-ctx.Done():
			case <-release:
			}
			return true, nil
		}
	}
	done := make(chan struct{})
	go func() { defer close(done); Run(ctx, 2, step(&a), step(&b)) }()
	for range 2 {
		select {
		case <-entered:
		case <-time.After(time.Second):
			t.Fatal("workers did not start")
		}
	}
	if peak.Load() != 2 || a.Load() == 0 || b.Load() == 0 {
		t.Fatal("worker limit/fairness", peak.Load(), a.Load(), b.Load())
	}
	close(release)
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("pool did not join")
	}
	if active.Load() != 0 || peak.Load() > 2 {
		t.Fatal("overlapping/unjoined workers")
	}
}
