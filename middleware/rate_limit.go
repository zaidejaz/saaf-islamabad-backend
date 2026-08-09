package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// PasswordResetRateLimit limits password-reset attempts per authenticated actor.
// Default: 5 attempts per 15 minutes.
func PasswordResetRateLimit(maxAttempts int, window time.Duration) gin.HandlerFunc {
	if maxAttempts <= 0 {
		maxAttempts = 5
	}
	if window <= 0 {
		window = 15 * time.Minute
	}

	type bucket struct {
		times []time.Time
	}
	var (
		mu   sync.Mutex
		byID = make(map[uuid.UUID]*bucket)
	)

	return func(c *gin.Context) {
		raw, exists := c.Get("user_id")
		if !exists {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		actorID, ok := raw.(uuid.UUID)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		now := time.Now()
		cutoff := now.Add(-window)

		mu.Lock()
		b := byID[actorID]
		if b == nil {
			b = &bucket{}
			byID[actorID] = b
		}
		kept := b.times[:0]
		for _, t := range b.times {
			if t.After(cutoff) {
				kept = append(kept, t)
			}
		}
		b.times = kept
		if len(b.times) >= maxAttempts {
			mu.Unlock()
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"success": false,
				"message": "too many password reset attempts; try again later",
			})
			return
		}
		b.times = append(b.times, now)
		mu.Unlock()

		c.Next()
	}
}
