package sources

import (
	"context"
	"sync"
	"time"
)

// RateLimiter provides per-source rate limiting.
type RateLimiter struct {
	mu       sync.Mutex
	buckets  map[SourceType]*tokenBucket
	defaults map[SourceType]RateLimit
}

// RateLimit defines rate limit parameters.
type RateLimit struct {
	RequestsPerMinute int
	BurstSize         int
}

// DefaultRateLimits returns the default rate limits per source.
func DefaultRateLimits() map[SourceType]RateLimit {
	return map[SourceType]RateLimit{
		SourceTypeLocal:     {RequestsPerMinute: 0, BurstSize: 0},     // No limit for local
		SourceTypeSkillsSH:  {RequestsPerMinute: 60, BurstSize: 10},   // 60/min
		SourceTypeGitHub:    {RequestsPerMinute: 10, BurstSize: 5},    // 10/min (unauthenticated)
		SourceTypeGitLab:    {RequestsPerMinute: 10, BurstSize: 5},    // 10/min
		SourceTypeBitbucket: {RequestsPerMinute: 10, BurstSize: 5},    // 10/min
		SourceTypeCodeberg:  {RequestsPerMinute: 20, BurstSize: 5},    // 20/min
	}
}

// tokenBucket implements a token bucket rate limiter.
type tokenBucket struct {
	tokens     float64
	maxTokens  float64
	refillRate float64 // tokens per second
	lastRefill time.Time
	mu         sync.Mutex
}

// newTokenBucket creates a new token bucket.
func newTokenBucket(limit RateLimit) *tokenBucket {
	if limit.RequestsPerMinute == 0 {
		return nil // No rate limiting
	}

	maxTokens := float64(limit.BurstSize)
	if maxTokens == 0 {
		maxTokens = float64(limit.RequestsPerMinute) / 6 // Default burst to 10 seconds worth
	}

	return &tokenBucket{
		tokens:     maxTokens,
		maxTokens:  maxTokens,
		refillRate: float64(limit.RequestsPerMinute) / 60.0,
		lastRefill: time.Now(),
	}
}

// take attempts to take a token, blocking if necessary.
func (b *tokenBucket) take(ctx context.Context) error {
	if b == nil {
		return nil // No rate limiting
	}

	for {
		b.mu.Lock()
		// Refill tokens based on time elapsed
		now := time.Now()
		elapsed := now.Sub(b.lastRefill).Seconds()
		b.tokens += elapsed * b.refillRate
		if b.tokens > b.maxTokens {
			b.tokens = b.maxTokens
		}
		b.lastRefill = now

		if b.tokens >= 1 {
			b.tokens--
			b.mu.Unlock()
			return nil
		}

		// Calculate wait time
		waitTime := time.Duration((1 - b.tokens) / b.refillRate * float64(time.Second))
		b.mu.Unlock()

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(waitTime):
			// Try again after waiting
		}
	}
}

// NewRateLimiter creates a new rate limiter with default limits.
func NewRateLimiter() *RateLimiter {
	return &RateLimiter{
		buckets:  make(map[SourceType]*tokenBucket),
		defaults: DefaultRateLimits(),
	}
}

// NewRateLimiterWithLimits creates a rate limiter with custom limits.
func NewRateLimiterWithLimits(limits map[SourceType]RateLimit) *RateLimiter {
	return &RateLimiter{
		buckets:  make(map[SourceType]*tokenBucket),
		defaults: limits,
	}
}

// Wait blocks until a request is allowed for the given source.
func (r *RateLimiter) Wait(ctx context.Context, source SourceType) error {
	bucket := r.getBucket(source)
	return bucket.take(ctx)
}

// getBucket returns or creates a token bucket for the source.
func (r *RateLimiter) getBucket(source SourceType) *tokenBucket {
	r.mu.Lock()
	defer r.mu.Unlock()

	if bucket, ok := r.buckets[source]; ok {
		return bucket
	}

	limit, ok := r.defaults[source]
	if !ok {
		limit = RateLimit{RequestsPerMinute: 10, BurstSize: 5} // Default fallback
	}

	bucket := newTokenBucket(limit)
	r.buckets[source] = bucket
	return bucket
}

// SetLimit updates the rate limit for a source.
func (r *RateLimiter) SetLimit(source SourceType, limit RateLimit) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.defaults[source] = limit
	delete(r.buckets, source) // Will be recreated with new limit
}
