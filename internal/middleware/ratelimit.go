package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

// ipRateLimiter keeps a token-bucket limiter per client IP address. It is the
// Go equivalent of slowapi's per-IP rate limiting.
type ipRateLimiter struct {
	mu      sync.Mutex
	buckets map[string]*bucket
	limit   rate.Limit
	burst   int
}

type bucket struct {
	limiter *rate.Limiter
	seen    time.Time
}

// RateLimit builds middleware allowing `perMinute` requests per minute per IP,
// with a burst equal to `perMinute`. A value <= 0 disables rate limiting and
// returns a pass-through middleware.
func RateLimit(perMinute int) gin.HandlerFunc {
	if perMinute <= 0 {
		return func(c *gin.Context) { c.Next() }
	}
	rl := &ipRateLimiter{
		buckets: make(map[string]*bucket),
		limit:   rate.Every(time.Minute / time.Duration(perMinute)),
		burst:   perMinute,
	}

	return func(c *gin.Context) {
		if !rl.get(c.ClientIP()).Allow() {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"detail": "Rate limit exceeded. Please try again later.",
			})
			return
		}
		c.Next()
	}
}

func (rl *ipRateLimiter) get(ip string) *rate.Limiter {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	b, ok := rl.buckets[ip]
	if !ok {
		b = &bucket{limiter: rate.NewLimiter(rl.limit, rl.burst)}
		rl.buckets[ip] = b
	}
	b.seen = time.Now()
	rl.evictStale()
	return b.limiter
}

// evictStale drops limiters that have been idle for over an hour to keep the map
// from growing unbounded. Called under lock.
func (rl *ipRateLimiter) evictStale() {
	cutoff := time.Now().Add(-time.Hour)
	for ip, b := range rl.buckets {
		if b.seen.Before(cutoff) {
			delete(rl.buckets, ip)
		}
	}
}
