package main

import (
	"math"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type rateLimitEntry struct {
	count   int
	resetAt time.Time
}

func RateLimit(limit int, window time.Duration) gin.HandlerFunc {
	if limit < 1 || window <= 0 {
		panic("rate limit and window must be positive")
	}

	var mutex sync.Mutex
	clients := make(map[string]rateLimitEntry)

	return func(c *gin.Context) {
		now := time.Now()
		clientIP := c.ClientIP()

		mutex.Lock()
		for ip, entry := range clients {
			if !now.Before(entry.resetAt) {
				delete(clients, ip)
			}
		}

		entry := clients[clientIP]
		if !now.Before(entry.resetAt) {
			entry = rateLimitEntry{resetAt: now.Add(window)}
		}
		allowed := entry.count < limit
		if allowed {
			entry.count++
		}
		clients[clientIP] = entry
		mutex.Unlock()

		remaining := limit - entry.count
		if remaining < 0 {
			remaining = 0
		}
		c.Header("X-RateLimit-Limit", strconv.Itoa(limit))
		c.Header("X-RateLimit-Remaining", strconv.Itoa(remaining))

		if !allowed {
			retryAfter := int(math.Ceil(time.Until(entry.resetAt).Seconds()))
			if retryAfter < 1 {
				retryAfter = 1
			}
			c.Header("Retry-After", strconv.Itoa(retryAfter))
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "rate limit exceeded"})
			return
		}
		c.Next()
	}
}
