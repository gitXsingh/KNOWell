package ratelimit

import (
	"net/http"
	"sync"

	"golang.org/x/time/rate"
)

type IPLimiter struct {
	mu       sync.Mutex
	limiters map[string]*rate.Limiter
	rate     rate.Limit
	burst    int
}

func New(requestsPerSecond, burst int) *IPLimiter {
	return &IPLimiter{
		limiters: make(map[string]*rate.Limiter),
		rate:     rate.Limit(requestsPerSecond),
		burst:    burst,
	}
}

func (l *IPLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := r.RemoteAddr
		l.mu.Lock()
		limiter, ok := l.limiters[ip]
		if !ok {
			limiter = rate.NewLimiter(l.rate, l.burst)
			l.limiters[ip] = limiter
		}
		l.mu.Unlock()

		if !limiter.Allow() {
			http.Error(w, `{"error":{"code":"rate_limited","message":"Too many requests"}}`, http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}
