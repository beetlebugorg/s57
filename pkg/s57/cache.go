package s57

import (
	"container/list"
	"fmt"
	"sync"
	"time"
)

// ChartCache manages loaded charts with LRU eviction policy.
//
// The cache stores fully-parsed charts in memory and evicts least-recently-used
// charts when memory limits are exceeded. This enables lazy loading of charts
// on demand while keeping frequently accessed charts readily available.
//
// Memory estimation is approximate, based on feature count and typical feature sizes.
//
// Example:
//
//	cache := s57.NewChartCache(1024 * 1024 * 1024) // 1GB limit
//
//	// Get chart (loads from disk if not cached)
//	chart, err := cache.Get("US5MA22M", func() (*Chart, error) {
//	    return parser.Parse("/path/to/US5MA22M.000")
//	})
type ChartCache struct {
	maxMemory  int64            // Maximum memory in bytes
	usedMemory int64            // Current memory usage estimate
	charts     map[string]*cacheEntry
	lru        *list.List       // LRU list (most recent at front)
	mu         sync.RWMutex
}

// cacheEntry tracks a cached chart and its metadata
type cacheEntry struct {
	name         string
	chart        *Chart
	memorySize   int64
	element      *list.Element // Position in LRU list
	lastAccessed time.Time
	accessCount  int
}

// NewChartCache creates a new cache with the specified memory limit in bytes.
//
// The memory limit is enforced approximately - actual memory usage may temporarily
// exceed the limit during chart loading. Set to 0 for unlimited cache size.
//
// Example:
//
//	cache := s57.NewChartCache(512 * 1024 * 1024) // 512MB
func NewChartCache(maxMemoryBytes int64) *ChartCache {
	return &ChartCache{
		maxMemory: maxMemoryBytes,
		charts:    make(map[string]*cacheEntry),
		lru:       list.New(),
	}
}

// Get retrieves a chart from cache or loads it using the provided loader function.
//
// If the chart is cached, it's returned immediately and moved to the front of the
// LRU list. If not cached, the loader function is called to load the chart, which
// is then cached for future access.
//
// The loader function should parse the chart from disk. It's only called on cache miss.
//
// If adding the chart would exceed memory limits, least-recently-used charts are
// evicted until sufficient space is available.
//
// Example:
//
//	chart, err := cache.Get("US5MA22M", func() (*Chart, error) {
//	    return parser.Parse("/path/to/US5MA22M.000")
//	})
func (c *ChartCache) Get(name string, loader func() (*Chart, error)) (*Chart, error) {
	// Fast path: check cache with read lock
	c.mu.RLock()
	if entry, ok := c.charts[name]; ok {
		c.mu.RUnlock()

		// Update access metadata with write lock
		c.mu.Lock()
		entry.lastAccessed = time.Now()
		entry.accessCount++
		c.lru.MoveToFront(entry.element)
		c.mu.Unlock()

		return entry.chart, nil
	}
	c.mu.RUnlock()

	// Cache miss - load chart
	chart, err := loader()
	if err != nil {
		return nil, fmt.Errorf("load chart: %w", err)
	}

	// Add to cache
	if err := c.Add(name, chart); err != nil {
		// Cache add failed, but we still have the chart
		// Return it without caching
		return chart, nil
	}

	return chart, nil
}

// Add adds a chart to the cache.
//
// If the cache is at capacity, least-recently-used charts are evicted to make room.
// Returns error if the chart cannot be cached (e.g., chart is larger than max memory).
func (c *ChartCache) Add(name string, chart *Chart) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check if already cached
	if entry, ok := c.charts[name]; ok {
		// Update and move to front
		entry.chart = chart
		entry.lastAccessed = time.Now()
		entry.accessCount++
		c.lru.MoveToFront(entry.element)
		return nil
	}

	// Estimate memory usage
	memSize := estimateChartMemory(chart)

	// If chart is larger than max memory, don't cache it
	if c.maxMemory > 0 && memSize > c.maxMemory {
		return fmt.Errorf("chart too large for cache (%d bytes > %d bytes max)",
			memSize, c.maxMemory)
	}

	// Evict until we have space
	if c.maxMemory > 0 {
		for c.usedMemory+memSize > c.maxMemory && c.lru.Len() > 0 {
			c.evictLRU()
		}
	}

	// Add to cache
	entry := &cacheEntry{
		name:         name,
		chart:        chart,
		memorySize:   memSize,
		lastAccessed: time.Now(),
		accessCount:  1,
	}
	entry.element = c.lru.PushFront(entry)
	c.charts[name] = entry
	c.usedMemory += memSize

	return nil
}

// evictLRU removes the least recently used chart from cache.
// Must be called with c.mu locked.
func (c *ChartCache) evictLRU() {
	if c.lru.Len() == 0 {
		return
	}

	// Remove from back of LRU list
	elem := c.lru.Back()
	if elem == nil {
		return
	}

	entry := elem.Value.(*cacheEntry)
	c.lru.Remove(elem)
	delete(c.charts, entry.name)
	c.usedMemory -= entry.memorySize
}

// Remove explicitly removes a chart from the cache.
func (c *ChartCache) Remove(name string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if entry, ok := c.charts[name]; ok {
		c.lru.Remove(entry.element)
		delete(c.charts, name)
		c.usedMemory -= entry.memorySize
	}
}

// Clear removes all charts from the cache.
func (c *ChartCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.charts = make(map[string]*cacheEntry)
	c.lru.Init()
	c.usedMemory = 0
}

// Stats returns cache statistics.
func (c *ChartCache) Stats() CacheStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	totalAccess := 0
	for _, entry := range c.charts {
		totalAccess += entry.accessCount
	}

	return CacheStats{
		ChartCount:   len(c.charts),
		UsedMemory:   c.usedMemory,
		MaxMemory:    c.maxMemory,
		TotalAccess:  totalAccess,
	}
}

// CacheStats holds cache performance metrics.
type CacheStats struct {
	ChartCount  int   // Number of charts currently cached
	UsedMemory  int64 // Estimated memory usage in bytes
	MaxMemory   int64 // Maximum memory limit in bytes
	TotalAccess int   // Total number of accesses across all cached charts
}

// estimateChartMemory estimates memory usage for a chart.
//
// This is approximate and based on:
//   - Base overhead: ~1KB per chart
//   - Feature count: ~1KB per feature (average)
//   - Geometry coordinates: 16 bytes per coordinate pair
//
// Actual memory usage varies significantly based on feature complexity,
// attribute count, and string data.
func estimateChartMemory(chart *Chart) int64 {
	if chart == nil {
		return 0
	}

	// Base overhead
	size := int64(1024)

	// Feature overhead
	features := chart.Features()
	featureCount := len(features)
	size += int64(featureCount) * 1024

	// Coordinate overhead
	for _, feature := range features {
		geom := feature.Geometry()
		coordCount := len(geom.Coordinates)
		size += int64(coordCount) * 16
	}

	return size
}
