package middleware

import (
	"net"
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type rateLimitEntry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

type IPRateLimiter struct {
	mu                sync.Mutex
	clients           map[string]*rateLimitEntry
	requestsPerSecond rate.Limit
	burst             int
	lastCleanup       time.Time
}

func NewIPRateLimiter(requestsPerSecond float64, burst int) *IPRateLimiter {
	return &IPRateLimiter{
		clients:           make(map[string]*rateLimitEntry),
		requestsPerSecond: rate.Limit(requestsPerSecond),
		burst:             burst,
	}
}

func (l *IPRateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !l.allow(clientIP(r.RemoteAddr)) {
			w.Header().Set("Retry-After", "1")
			writeError(w, http.StatusTooManyRequests, "rate limit exceeded")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (l *IPRateLimiter) allow(client string) bool {
	now := time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()
	if now.Sub(l.lastCleanup) >= time.Minute {
		for ip, entry := range l.clients {
			if now.Sub(entry.lastSeen) >= 10*time.Minute {
				delete(l.clients, ip)
			}
		}
		l.lastCleanup = now
	}
	entry, ok := l.clients[client]
	if !ok {
		entry = &rateLimitEntry{limiter: rate.NewLimiter(l.requestsPerSecond, l.burst)}
		l.clients[client] = entry
	}
	entry.lastSeen = now
	return entry.limiter.AllowN(now, 1)
}

func clientIP(remoteAddr string) string {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err == nil {
		return host
	}
	return remoteAddr
}
