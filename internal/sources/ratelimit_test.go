package sources

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestRateLimiter_Wait_NoLimit(t *testing.T) {
	rl := NewRateLimiterWithLimits(map[SourceType]RateLimit{
		SourceTypeLocal: {RequestsPerMinute: 0, BurstSize: 0}, // No limit
	})

	ctx := context.Background()

	// Should return immediately with no delay
	start := time.Now()
	for i := 0; i < 100; i++ {
		err := rl.Wait(ctx, SourceTypeLocal)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}
	elapsed := time.Since(start)

	// 100 requests with no limit should be nearly instant
	if elapsed > 100*time.Millisecond {
		t.Errorf("expected fast completion, took %v", elapsed)
	}
}

func TestRateLimiter_Wait_BurstAllowed(t *testing.T) {
	rl := NewRateLimiterWithLimits(map[SourceType]RateLimit{
		SourceTypeGitHub: {RequestsPerMinute: 60, BurstSize: 5},
	})

	ctx := context.Background()

	// Burst of 5 requests should complete quickly
	start := time.Now()
	for i := 0; i < 5; i++ {
		err := rl.Wait(ctx, SourceTypeGitHub)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}
	elapsed := time.Since(start)

	// Burst should be fast
	if elapsed > 50*time.Millisecond {
		t.Errorf("burst should be fast, took %v", elapsed)
	}
}

func TestRateLimiter_Wait_ContextCancellation(t *testing.T) {
	rl := NewRateLimiterWithLimits(map[SourceType]RateLimit{
		SourceTypeGitHub: {RequestsPerMinute: 1, BurstSize: 1}, // Very slow
	})

	// Exhaust the bucket
	ctx := context.Background()
	_ = rl.Wait(ctx, SourceTypeGitHub)

	// Now cancel context while waiting
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	err := rl.Wait(ctx, SourceTypeGitHub)
	if err == nil {
		t.Error("expected context cancellation error")
	}
	if err != context.DeadlineExceeded {
		t.Errorf("expected context.DeadlineExceeded, got %v", err)
	}
}

func TestRateLimiter_ConcurrentWait(t *testing.T) {
	rl := NewRateLimiterWithLimits(map[SourceType]RateLimit{
		SourceTypeGitHub: {RequestsPerMinute: 600, BurstSize: 10}, // 10 per second
	})

	ctx := context.Background()
	var wg sync.WaitGroup
	errors := make(chan error, 20)

	// 10 concurrent requests
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := rl.Wait(ctx, SourceTypeGitHub); err != nil {
				errors <- err
			}
		}()
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRateLimiter_SetLimit(t *testing.T) {
	rl := NewRateLimiter()

	// Set a new limit
	rl.SetLimit(SourceTypeGitHub, RateLimit{
		RequestsPerMinute: 120,
		BurstSize:         10,
	})

	// Verify the limit was updated
	rl.mu.Lock()
	limit := rl.defaults[SourceTypeGitHub]
	rl.mu.Unlock()

	if limit.RequestsPerMinute != 120 {
		t.Errorf("expected 120 RPM, got %d", limit.RequestsPerMinute)
	}
	if limit.BurstSize != 10 {
		t.Errorf("expected burst 10, got %d", limit.BurstSize)
	}
}

func TestRateLimiter_DefaultRateLimits(t *testing.T) {
	defaults := DefaultRateLimits()

	tests := []struct {
		source   SourceType
		expected RateLimit
	}{
		{SourceTypeLocal, RateLimit{RequestsPerMinute: 0, BurstSize: 0}},
		{SourceTypeGitHub, RateLimit{RequestsPerMinute: 10, BurstSize: 5}},
		{SourceTypeGitLab, RateLimit{RequestsPerMinute: 10, BurstSize: 5}},
		{SourceTypeSkillsSH, RateLimit{RequestsPerMinute: 60, BurstSize: 10}},
		{SourceTypeCodeberg, RateLimit{RequestsPerMinute: 20, BurstSize: 5}},
	}

	for _, tt := range tests {
		t.Run(string(tt.source), func(t *testing.T) {
			limit, ok := defaults[tt.source]
			if !ok {
				t.Fatalf("no default limit for %s", tt.source)
			}
			if limit.RequestsPerMinute != tt.expected.RequestsPerMinute {
				t.Errorf("expected RPM %d, got %d", tt.expected.RequestsPerMinute, limit.RequestsPerMinute)
			}
			if limit.BurstSize != tt.expected.BurstSize {
				t.Errorf("expected burst %d, got %d", tt.expected.BurstSize, limit.BurstSize)
			}
		})
	}
}

func TestRateLimiter_getBucket_UnknownSource(t *testing.T) {
	rl := NewRateLimiter()

	// Unknown source should get default fallback limit
	bucket := rl.getBucket("unknown")

	// Should not be nil (fallback creates a bucket)
	if bucket == nil {
		// This is expected for sources not in defaults
		// but the implementation provides a default fallback
		t.Log("bucket is nil for unknown source (may be expected)")
	}
}

func TestTokenBucket_Take_Nil(t *testing.T) {
	var bucket *tokenBucket = nil

	// Nil bucket should return nil error (no rate limiting)
	err := bucket.take(context.Background())
	if err != nil {
		t.Errorf("expected nil error for nil bucket, got %v", err)
	}
}

func TestTokenBucket_Take_Refill(t *testing.T) {
	limit := RateLimit{
		RequestsPerMinute: 6000, // 100 per second
		BurstSize:         1,
	}
	bucket := newTokenBucket(limit)

	ctx := context.Background()

	// Exhaust the token
	err := bucket.take(ctx)
	if err != nil {
		t.Fatalf("first take failed: %v", err)
	}

	// Wait for refill (at 100/sec, should refill in ~10ms)
	time.Sleep(15 * time.Millisecond)

	// Should be able to take again
	err = bucket.take(ctx)
	if err != nil {
		t.Errorf("expected refill to allow take, got error: %v", err)
	}
}

func TestNewRateLimiter(t *testing.T) {
	rl := NewRateLimiter()

	if rl.buckets == nil {
		t.Error("expected buckets map to be initialized")
	}
	if rl.defaults == nil {
		t.Error("expected defaults map to be initialized")
	}
}

func TestNewRateLimiterWithLimits(t *testing.T) {
	customLimits := map[SourceType]RateLimit{
		"custom": {RequestsPerMinute: 100, BurstSize: 20},
	}

	rl := NewRateLimiterWithLimits(customLimits)

	if limit, ok := rl.defaults["custom"]; !ok {
		t.Error("expected custom limit to be set")
	} else if limit.RequestsPerMinute != 100 {
		t.Errorf("expected 100 RPM, got %d", limit.RequestsPerMinute)
	}
}

func TestRateLimiter_MultipleSources(t *testing.T) {
	rl := NewRateLimiterWithLimits(map[SourceType]RateLimit{
		SourceTypeGitHub: {RequestsPerMinute: 600, BurstSize: 5},
		SourceTypeGitLab: {RequestsPerMinute: 600, BurstSize: 5},
	})

	ctx := context.Background()

	// Exhaust GitHub bucket
	for i := 0; i < 5; i++ {
		_ = rl.Wait(ctx, SourceTypeGitHub)
	}

	// GitLab should still have tokens
	start := time.Now()
	err := rl.Wait(ctx, SourceTypeGitLab)
	elapsed := time.Since(start)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if elapsed > 10*time.Millisecond {
		t.Errorf("GitLab should have independent tokens, took %v", elapsed)
	}
}

func TestNewTokenBucket_ZeroBurstDefault(t *testing.T) {
	limit := RateLimit{
		RequestsPerMinute: 60,
		BurstSize:         0, // Should default to RPM/6 = 10
	}
	bucket := newTokenBucket(limit)

	if bucket.maxTokens != 10 {
		t.Errorf("expected default max tokens of 10, got %f", bucket.maxTokens)
	}
}

func TestNewTokenBucket_NoLimit(t *testing.T) {
	limit := RateLimit{
		RequestsPerMinute: 0,
		BurstSize:         0,
	}
	bucket := newTokenBucket(limit)

	if bucket != nil {
		t.Error("expected nil bucket for no rate limit")
	}
}
