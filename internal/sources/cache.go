package sources

import (
	"strconv"
	"sync"
	"time"
)

// CacheEntry holds a cached value with expiration.
type CacheEntry struct {
	Value     interface{}
	ExpiresAt time.Time
}

// Cache provides in-memory caching with TTL.
type Cache struct {
	mu      sync.RWMutex
	entries map[string]CacheEntry
	ttl     time.Duration
}

// NewCache creates a new cache with the specified TTL.
func NewCache(ttl time.Duration) *Cache {
	c := &Cache{
		entries: make(map[string]CacheEntry),
		ttl:     ttl,
	}
	// Start cleanup goroutine
	go c.cleanup()
	return c
}

// Get retrieves a value from the cache.
func (c *Cache) Get(key string) (interface{}, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.entries[key]
	if !ok {
		return nil, false
	}

	if time.Now().After(entry.ExpiresAt) {
		return nil, false
	}

	return entry.Value, true
}

// Set stores a value in the cache.
func (c *Cache) Set(key string, value interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries[key] = CacheEntry{
		Value:     value,
		ExpiresAt: time.Now().Add(c.ttl),
	}
}

// SetWithTTL stores a value with a custom TTL.
func (c *Cache) SetWithTTL(key string, value interface{}, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries[key] = CacheEntry{
		Value:     value,
		ExpiresAt: time.Now().Add(ttl),
	}
}

// Delete removes a value from the cache.
func (c *Cache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.entries, key)
}

// Clear removes all entries from the cache.
func (c *Cache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries = make(map[string]CacheEntry)
}

// cleanup periodically removes expired entries.
func (c *Cache) cleanup() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		c.removeExpired()
	}
}

// removeExpired removes all expired entries.
func (c *Cache) removeExpired() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	for key, entry := range c.entries {
		if now.After(entry.ExpiresAt) {
			delete(c.entries, key)
		}
	}
}

// SearchCache provides specialized caching for search results.
type SearchCache struct {
	*Cache
}

// NewSearchCache creates a cache for search results (10 minute TTL).
func NewSearchCache() *SearchCache {
	return &SearchCache{
		Cache: NewCache(10 * time.Minute),
	}
}

// GetSearchResult retrieves cached search results.
func (c *SearchCache) GetSearchResult(source SourceType, query string, page int) (*SearchResult, bool) {
	key := c.searchKey(source, query, page)
	if val, ok := c.Get(key); ok {
		if result, ok := val.(*SearchResult); ok {
			return result, true
		}
	}
	return nil, false
}

// SetSearchResult caches search results.
func (c *SearchCache) SetSearchResult(source SourceType, query string, page int, result *SearchResult) {
	key := c.searchKey(source, query, page)
	c.Set(key, result)
}

// searchKey generates a cache key for search results.
func (c *SearchCache) searchKey(source SourceType, query string, page int) string {
	return string(source) + ":" + query + ":" + strconv.Itoa(page)
}

// SkillCache provides specialized caching for individual skills.
type SkillCache struct {
	*Cache
}

// NewSkillCache creates a cache for individual skills (10 minute TTL).
func NewSkillCache() *SkillCache {
	return &SkillCache{
		Cache: NewCache(10 * time.Minute),
	}
}

// GetSkill retrieves a cached skill.
func (c *SkillCache) GetSkill(source SourceType, id string) (*ExternalSkill, bool) {
	key := string(source) + ":" + id
	if val, ok := c.Get(key); ok {
		if skill, ok := val.(*ExternalSkill); ok {
			return skill, true
		}
	}
	return nil, false
}

// SetSkill caches a skill.
func (c *SkillCache) SetSkill(source SourceType, id string, skill *ExternalSkill) {
	key := string(source) + ":" + id
	c.Set(key, skill)
}
