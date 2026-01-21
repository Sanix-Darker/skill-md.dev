package sources

import (
	"sync"
	"testing"
	"time"
)

func TestCache_BasicOperations(t *testing.T) {
	cache := NewCache(time.Minute)

	t.Run("set and get", func(t *testing.T) {
		cache.Set("key1", "value1")

		val, ok := cache.Get("key1")
		if !ok {
			t.Error("expected key to exist")
		}
		if val != "value1" {
			t.Errorf("expected 'value1', got %v", val)
		}
	})

	t.Run("get non-existent key", func(t *testing.T) {
		val, ok := cache.Get("nonexistent")
		if ok {
			t.Error("expected key not to exist")
		}
		if val != nil {
			t.Errorf("expected nil, got %v", val)
		}
	})

	t.Run("delete key", func(t *testing.T) {
		cache.Set("toDelete", "value")
		cache.Delete("toDelete")

		_, ok := cache.Get("toDelete")
		if ok {
			t.Error("expected key to be deleted")
		}
	})

	t.Run("clear all", func(t *testing.T) {
		cache.Set("key1", "value1")
		cache.Set("key2", "value2")
		cache.Clear()

		_, ok1 := cache.Get("key1")
		_, ok2 := cache.Get("key2")
		if ok1 || ok2 {
			t.Error("expected all keys to be cleared")
		}
	})
}

func TestCache_Expiration(t *testing.T) {
	// Create cache with very short TTL
	cache := NewCache(10 * time.Millisecond)

	cache.Set("expiring", "value")

	// Value should exist immediately
	_, ok := cache.Get("expiring")
	if !ok {
		t.Error("expected key to exist immediately")
	}

	// Wait for expiration
	time.Sleep(20 * time.Millisecond)

	// Value should be expired
	_, ok = cache.Get("expiring")
	if ok {
		t.Error("expected key to be expired")
	}
}

func TestCache_SetWithTTL(t *testing.T) {
	cache := NewCache(time.Hour) // Long default TTL

	// Set with short custom TTL
	cache.SetWithTTL("custom", "value", 10*time.Millisecond)

	_, ok := cache.Get("custom")
	if !ok {
		t.Error("expected key to exist immediately")
	}

	time.Sleep(20 * time.Millisecond)

	_, ok = cache.Get("custom")
	if ok {
		t.Error("expected key with custom TTL to expire")
	}
}

func TestCache_ConcurrentAccess(t *testing.T) {
	cache := NewCache(time.Minute)
	var wg sync.WaitGroup

	// Concurrent writes
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			cache.Set("concurrent", i)
		}(i)
	}

	// Concurrent reads
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cache.Get("concurrent")
		}()
	}

	wg.Wait()

	// Should not panic and value should exist
	_, ok := cache.Get("concurrent")
	if !ok {
		t.Error("expected concurrent key to exist")
	}
}

func TestSearchCache_GetSearchResult(t *testing.T) {
	cache := NewSearchCache()

	t.Run("cache miss", func(t *testing.T) {
		result, ok := cache.GetSearchResult(SourceTypeGitHub, "query", 1)
		if ok {
			t.Error("expected cache miss")
		}
		if result != nil {
			t.Error("expected nil result on cache miss")
		}
	})

	t.Run("cache hit", func(t *testing.T) {
		expected := &SearchResult{
			Skills: []*ExternalSkill{
				{ID: "1", Name: "Test Skill"},
			},
			Total:  1,
			Page:   1,
			Source: SourceTypeGitHub,
		}

		cache.SetSearchResult(SourceTypeGitHub, "query", 1, expected)

		result, ok := cache.GetSearchResult(SourceTypeGitHub, "query", 1)
		if !ok {
			t.Error("expected cache hit")
		}
		if result == nil {
			t.Fatal("expected non-nil result")
		}
		if len(result.Skills) != 1 {
			t.Errorf("expected 1 skill, got %d", len(result.Skills))
		}
	})

	t.Run("different query", func(t *testing.T) {
		_, ok := cache.GetSearchResult(SourceTypeGitHub, "different", 1)
		if ok {
			t.Error("expected cache miss for different query")
		}
	})

	t.Run("different page", func(t *testing.T) {
		_, ok := cache.GetSearchResult(SourceTypeGitHub, "query", 2)
		if ok {
			t.Error("expected cache miss for different page")
		}
	})

	t.Run("different source", func(t *testing.T) {
		_, ok := cache.GetSearchResult(SourceTypeGitLab, "query", 1)
		if ok {
			t.Error("expected cache miss for different source")
		}
	})
}

func TestSearchCache_KeyCollision(t *testing.T) {
	cache := NewSearchCache()

	// Test potential key collision with colon in query
	// Current implementation: source:query:page
	// If query contains ":", might cause collision

	result1 := &SearchResult{
		Skills: []*ExternalSkill{{ID: "1"}},
		Total:  1,
		Source: SourceTypeGitHub,
	}

	result2 := &SearchResult{
		Skills: []*ExternalSkill{{ID: "2"}},
		Total:  1,
		Source: SourceTypeGitHub,
	}

	// Set result for query with colon
	cache.SetSearchResult(SourceTypeGitHub, "user:test", 1, result1)

	// Set result for different query that might collide
	cache.SetSearchResult(SourceTypeGitHub, "user", 1, result2)

	// Verify no collision
	r1, ok1 := cache.GetSearchResult(SourceTypeGitHub, "user:test", 1)
	r2, ok2 := cache.GetSearchResult(SourceTypeGitHub, "user", 1)

	if !ok1 || !ok2 {
		t.Error("expected both cache entries to exist")
	}

	if r1.Skills[0].ID == r2.Skills[0].ID {
		t.Error("cache key collision detected - different queries returned same result")
	}
}

func TestSkillCache_GetSkill(t *testing.T) {
	cache := NewSkillCache()

	t.Run("cache miss", func(t *testing.T) {
		result, ok := cache.GetSkill(SourceTypeGitHub, "id1")
		if ok {
			t.Error("expected cache miss")
		}
		if result != nil {
			t.Error("expected nil result on cache miss")
		}
	})

	t.Run("cache hit", func(t *testing.T) {
		expected := &ExternalSkill{
			ID:     "id1",
			Name:   "Test Skill",
			Source: SourceTypeGitHub,
		}

		cache.SetSkill(SourceTypeGitHub, "id1", expected)

		result, ok := cache.GetSkill(SourceTypeGitHub, "id1")
		if !ok {
			t.Error("expected cache hit")
		}
		if result == nil {
			t.Fatal("expected non-nil result")
		}
		if result.Name != "Test Skill" {
			t.Errorf("expected 'Test Skill', got %q", result.Name)
		}
	})

	t.Run("different source same id", func(t *testing.T) {
		_, ok := cache.GetSkill(SourceTypeGitLab, "id1")
		if ok {
			t.Error("expected cache miss for different source")
		}
	})
}

func TestCache_removeExpired(t *testing.T) {
	cache := NewCache(10 * time.Millisecond)

	cache.Set("key1", "value1")
	cache.Set("key2", "value2")

	// Wait for expiration
	time.Sleep(20 * time.Millisecond)

	// Manually trigger cleanup
	cache.removeExpired()

	// Both should be removed
	cache.mu.RLock()
	count := len(cache.entries)
	cache.mu.RUnlock()

	if count != 0 {
		t.Errorf("expected 0 entries after cleanup, got %d", count)
	}
}

func TestCache_InterfaceTypes(t *testing.T) {
	cache := NewCache(time.Minute)

	t.Run("string value", func(t *testing.T) {
		cache.Set("string", "test")
		val, _ := cache.Get("string")
		if v, ok := val.(string); !ok || v != "test" {
			t.Error("expected string value")
		}
	})

	t.Run("int value", func(t *testing.T) {
		cache.Set("int", 42)
		val, _ := cache.Get("int")
		if v, ok := val.(int); !ok || v != 42 {
			t.Error("expected int value")
		}
	})

	t.Run("slice value", func(t *testing.T) {
		cache.Set("slice", []string{"a", "b"})
		val, _ := cache.Get("slice")
		if v, ok := val.([]string); !ok || len(v) != 2 {
			t.Error("expected slice value")
		}
	})

	t.Run("struct value", func(t *testing.T) {
		cache.Set("struct", &ExternalSkill{ID: "1"})
		val, _ := cache.Get("struct")
		if v, ok := val.(*ExternalSkill); !ok || v.ID != "1" {
			t.Error("expected struct value")
		}
	})
}

func TestSearchCache_searchKey(t *testing.T) {
	cache := NewSearchCache()

	tests := []struct {
		source   SourceType
		query    string
		page     int
		expected string
	}{
		{SourceTypeGitHub, "test", 1, "github:test:1"},
		{SourceTypeGitLab, "api", 2, "gitlab:api:2"},
		{SourceTypeLocal, "skill", 10, "local:skill:10"},
		{SourceTypeCodeberg, "", 1, "codeberg::1"},
		{SourceTypeSkillsSH, "query with spaces", 1, "skills.sh:query with spaces:1"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			key := cache.searchKey(tt.source, tt.query, tt.page)
			if key != tt.expected {
				t.Errorf("expected key %q, got %q", tt.expected, key)
			}
		})
	}
}
