package s57

import (
	"testing"
)

func TestCacheBasic(t *testing.T) {
	cache := NewChartCache(1024 * 1024) // 1MB

	// Test empty cache
	stats := cache.Stats()
	if stats.ChartCount != 0 {
		t.Errorf("Expected empty cache, got %d charts", stats.ChartCount)
	}

	// Test cache miss and load
	loadCount := 0
	chart, err := cache.Get("test", func() (*Chart, error) {
		loadCount++
		return &Chart{datasetName: "test"}, nil
	})
	if err != nil {
		t.Fatalf("Failed to load chart: %v", err)
	}
	if chart.DatasetName() != "test" {
		t.Errorf("Expected dataset name 'test', got '%s'", chart.DatasetName())
	}
	if loadCount != 1 {
		t.Errorf("Expected loader called once, got %d times", loadCount)
	}

	// Test cache hit
	chart2, err := cache.Get("test", func() (*Chart, error) {
		loadCount++
		return &Chart{datasetName: "test2"}, nil
	})
	if err != nil {
		t.Fatalf("Failed to get cached chart: %v", err)
	}
	if chart2.DatasetName() != "test" {
		t.Errorf("Expected cached dataset name 'test', got '%s'", chart2.DatasetName())
	}
	if loadCount != 1 {
		t.Errorf("Expected loader not called for cache hit, called %d times", loadCount)
	}
}

func TestCacheEviction(t *testing.T) {
	// Create small cache (10KB)
	cache := NewChartCache(10 * 1024)

	// Add multiple charts until eviction occurs
	for i := 0; i < 10; i++ {
		name := string(rune('A' + i))
		_, err := cache.Get(name, func() (*Chart, error) {
			// Create chart with some features to use memory
			features := make([]Feature, 5)
			return &Chart{
				datasetName: name,
				features:    features,
			}, nil
		})
		if err != nil {
			t.Fatalf("Failed to add chart %s: %v", name, err)
		}
	}

	stats := cache.Stats()
	if stats.ChartCount >= 10 {
		t.Errorf("Expected eviction, but cache has %d charts", stats.ChartCount)
	}
	if stats.UsedMemory > cache.maxMemory {
		t.Errorf("Cache exceeded max memory: %d > %d", stats.UsedMemory, cache.maxMemory)
	}
}

func TestCacheClear(t *testing.T) {
	cache := NewChartCache(1024 * 1024)

	// Add some charts
	for i := 0; i < 5; i++ {
		name := string(rune('A' + i))
		_, err := cache.Get(name, func() (*Chart, error) {
			return &Chart{datasetName: name}, nil
		})
		if err != nil {
			t.Fatalf("Failed to add chart: %v", err)
		}
	}

	if cache.Stats().ChartCount != 5 {
		t.Errorf("Expected 5 charts, got %d", cache.Stats().ChartCount)
	}

	// Clear cache
	cache.Clear()

	if cache.Stats().ChartCount != 0 {
		t.Errorf("Expected empty cache after clear, got %d charts", cache.Stats().ChartCount)
	}
	if cache.Stats().UsedMemory != 0 {
		t.Errorf("Expected zero memory after clear, got %d bytes", cache.Stats().UsedMemory)
	}
}

func TestCacheRemove(t *testing.T) {
	cache := NewChartCache(1024 * 1024)

	// Add chart
	_, err := cache.Get("test", func() (*Chart, error) {
		return &Chart{datasetName: "test"}, nil
	})
	if err != nil {
		t.Fatalf("Failed to add chart: %v", err)
	}

	if cache.Stats().ChartCount != 1 {
		t.Errorf("Expected 1 chart, got %d", cache.Stats().ChartCount)
	}

	// Remove chart
	cache.Remove("test")

	if cache.Stats().ChartCount != 0 {
		t.Errorf("Expected 0 charts after remove, got %d", cache.Stats().ChartCount)
	}

	// Try to get removed chart (should reload)
	loadCount := 0
	_, err = cache.Get("test", func() (*Chart, error) {
		loadCount++
		return &Chart{datasetName: "test"}, nil
	})
	if err != nil {
		t.Fatalf("Failed to reload chart: %v", err)
	}
	if loadCount != 1 {
		t.Errorf("Expected loader called after remove, called %d times", loadCount)
	}
}
