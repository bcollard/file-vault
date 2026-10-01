package server

import (
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// rateLimiter holds a per-IP token-bucket. Cloud Run autoscales, so each
// instance keeps its own counter — this protects against single-client abuse,
// not distributed floods. Cloud Armor sits in front of /d/* for that.
type rateLimiter struct {
	mu       sync.Mutex
	visitors map[string]*visitor
	rate     rate.Limit
	burst    int
	idle     time.Duration
}

type visitor struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

func newRateLimiter(r rate.Limit, burst int, idle time.Duration) *rateLimiter {
	rl := &rateLimiter{
		visitors: make(map[string]*visitor),
		rate:     r,
		burst:    burst,
		idle:     idle,
	}
	go rl.sweep()
	return rl
}

func (rl *rateLimiter) get(ip string) *rate.Limiter {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	v, ok := rl.visitors[ip]
	if !ok {
		l := rate.NewLimiter(rl.rate, rl.burst)
		rl.visitors[ip] = &visitor{limiter: l, lastSeen: time.Now()}
		return l
	}
	v.lastSeen = time.Now()
	return v.limiter
}

func (rl *rateLimiter) sweep() {
	t := time.NewTicker(rl.idle)
	defer t.Stop()
	for now := range t.C {
		rl.mu.Lock()
		for ip, v := range rl.visitors {
			if now.Sub(v.lastSeen) > rl.idle {
				delete(rl.visitors, ip)
			}
		}
		rl.mu.Unlock()
	}
}

func (rl *rateLimiter) middleware(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !rl.get(clientIP(r)).Allow() {
			w.Header().Set("Retry-After", "60")
			http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
			return
		}
		h(w, r)
	}
}
