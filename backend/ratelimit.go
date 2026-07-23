package main

import (
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type PerIPRateLimiter struct {
	limiters sync.Map
	rate     rate.Limit
	burst    int
	ttl      time.Duration
	stopCh   chan struct{}
}

func NewPerIPRateLimiter(r rate.Limit, burst int, ttl time.Duration) *PerIPRateLimiter {
	rl := &PerIPRateLimiter{
		rate:   r,
		burst:  burst,
		ttl:    ttl,
		stopCh: make(chan struct{}),
	}
	go rl.cleanupLoop()
	return rl
}

func (rl *PerIPRateLimiter) Stop() {
	close(rl.stopCh)
}

func (rl *PerIPRateLimiter) GetLimiter(ip string) *rate.Limiter {
	actual, _ := rl.limiters.LoadOrStore(ip, rate.NewLimiter(rl.rate, rl.burst))
	return actual.(*rate.Limiter)
}

func (rl *PerIPRateLimiter) Allow(ip string) bool {
	return rl.GetLimiter(ip).Allow()
}

func (rl *PerIPRateLimiter) cleanupLoop() {
	ticker := time.NewTicker(rl.ttl)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			rl.cleanup()
		case <-rl.stopCh:
			return
		}
	}
}

func (rl *PerIPRateLimiter) cleanup() {
	rl.limiters.Range(func(key, value interface{}) bool {
		limiter := value.(*rate.Limiter)
		if limiter.Tokens() >= float64(rl.burst) {
			rl.limiters.Delete(key)
		}
		return true
	})
}

type GlobalRateLimiter struct {
	limiter *rate.Limiter
}

func NewGlobalRateLimiter(r rate.Limit, burst int) *GlobalRateLimiter {
	return &GlobalRateLimiter{
		limiter: rate.NewLimiter(r, burst),
	}
}

func (gl *GlobalRateLimiter) Allow() bool {
	return gl.limiter.Allow()
}
