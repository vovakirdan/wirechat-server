package http

import "time"

type rateLimiter struct {
	limit   int
	counter int
	reset   *time.Ticker
}

func newRateLimiter(limit int) *rateLimiter {
	if limit <= 0 {
		return &rateLimiter{limit: 0}
	}
	return &rateLimiter{
		limit: limit,
		reset: time.NewTicker(time.Minute),
	}
}

func (r *rateLimiter) allow() bool {
	if r == nil || r.limit <= 0 {
		return true
	}
	r.counter++
	return r.counter <= r.limit
}

func (r *rateLimiter) startReset(stop <-chan struct{}) {
	if r == nil || r.reset == nil {
		return
	}
	go func() {
		for {
			select {
			case <-r.reset.C:
				r.counter = 0
			case <-stop:
				r.reset.Stop()
				return
			}
		}
	}()
}
