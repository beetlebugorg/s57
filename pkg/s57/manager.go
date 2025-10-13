package s57

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// ChartManager provides automatic management of NOAA charts with catalog updates
// and on-demand downloading.
//
// This is the primary API for working with NOAA ENC charts. It handles:
//   - Catalog download and caching
//   - Catalog freshness checks and updates
//   - Chart downloading on-demand
//   - Local caching in XDG-compliant directories
//   - Lazy loading with LRU cache
//
// Example:
//
//	// Create manager (uses XDG cache directories automatically)
//	mgr, err := s57.NewNOAAChartManager(s57.NOAAManagerOptions{
//	    CacheSize: 1024 * 1024 * 1024, // 1GB in-memory cache
//	})
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Query viewport - automatically downloads charts if needed
//	viewport := s57.Bounds{
//	    MinLon: -122.5, MaxLon: -122.0,
//	    MinLat: 37.5, MaxLat: 38.0,
//	}
//	charts, err := mgr.GetChartsForViewport(viewport, 12)
type ChartManager struct {
	loader        *ChartLoader
	catalog       *ChartCatalog
	catalogPath   string
	chartCacheDir string
	maxCatalogAge time.Duration
}

// ChartManagerOptions configures chart manager behavior.
type ChartManagerOptions struct {
	// CacheSize sets maximum in-memory cache size in bytes.
	// Default: 512MB
	CacheSize int64

	// CatalogPath overrides the default XDG catalog location.
	// If empty, uses XDG-compliant default (~/.cache/canvas52/ENCProdCat_19115.xml)
	CatalogPath string

	// ChartCacheDir overrides the default XDG chart cache location.
	// If empty, uses XDG-compliant default (~/.cache/canvas52/noaa-s57)
	ChartCacheDir string

	// MaxCatalogAge sets how old the catalog can be before re-downloading.
	// Default: 7 days
	MaxCatalogAge time.Duration

	// ForceUpdate forces catalog re-download even if cached version is recent.
	// Default: false
	ForceUpdate bool

	// KeepExtracted controls whether to extract chart files to disk or stream from zip.
	// If true (default): Extract files to disk for convenient access.
	// If false: Stream charts directly from zip files in memory (saves ~67% disk space).
	//
	// Disk space comparison for 12 charts (NY Harbor):
	//   - KeepExtracted=true:  18MB (extracted files, no zips)
	//   - KeepExtracted=false: 5.8MB (zips only, 67% savings)
	//
	// Note: Streaming mode may be faster since it uses /tmp (often tmpfs/RAM).
	// Default: true (keep extracted files)
	KeepExtracted bool
}

// DefaultChartManagerOptions returns manager options with defaults.
func DefaultChartManagerOptions() ChartManagerOptions {
	return ChartManagerOptions{
		CacheSize:     512 * 1024 * 1024,  // 512MB
		MaxCatalogAge: 7 * 24 * time.Hour, // 7 days
		ForceUpdate:   false,
		KeepExtracted: true, // Extract by default for performance
	}
}

// NewChartManager creates a new chart manager with automatic catalog management.
//
// The manager:
//   1. Checks for cached catalog in XDG cache directory
//   2. Downloads catalog from NOAA if missing or stale
//   3. Creates a chart loader with download-on-demand
//   4. Manages chart cache in XDG cache directory
//
// All file operations use XDG-compliant directories by default, but can be
// overridden via options.
//
// Example:
//
//	mgr, err := s57.NewNOAAChartManager(s57.DefaultNOAAManagerOptions())
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Printf("Manager ready with %d charts indexed\n", mgr.ChartCount())
func NewChartManager(opts ChartManagerOptions) (*ChartManager, error) {
	// Get XDG paths if not overridden
	catalogPath := opts.CatalogPath
	if catalogPath == "" {
		var err error
		catalogPath, err = DefaultCatalogPath()
		if err != nil {
			return nil, fmt.Errorf("get default catalog path: %w", err)
		}
	}

	chartCacheDir := opts.ChartCacheDir
	if chartCacheDir == "" {
		var err error
		chartCacheDir, err = DefaultChartCacheDir()
		if err != nil {
			return nil, fmt.Errorf("get default chart cache dir: %w", err)
		}
	}

	// Ensure directories exist
	if err := os.MkdirAll(filepath.Dir(catalogPath), 0755); err != nil {
		return nil, fmt.Errorf("create catalog directory: %w", err)
	}
	if err := os.MkdirAll(chartCacheDir, 0755); err != nil {
		return nil, fmt.Errorf("create chart cache directory: %w", err)
	}

	// Load or download catalog
	catalog, err := loadOrUpdateCatalog(catalogPath, opts.MaxCatalogAge, opts.ForceUpdate)
	if err != nil {
		return nil, fmt.Errorf("load catalog: %w", err)
	}

	// Create chart loader (parser created internally)
	loader, err := NewChartLoaderFromCatalog(
		catalog,
		chartCacheDir,
		LoaderOptions{
			CacheSize:     opts.CacheSize,
			KeepExtracted: opts.KeepExtracted,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("create loader: %w", err)
	}

	return &ChartManager{
		loader:        loader,
		catalog:       catalog,
		catalogPath:   catalogPath,
		chartCacheDir: chartCacheDir,
		maxCatalogAge: opts.MaxCatalogAge,
	}, nil
}

// loadOrUpdateCatalog loads the catalog from cache or downloads if missing/stale.
func loadOrUpdateCatalog(path string, maxAge time.Duration, forceUpdate bool) (*ChartCatalog, error) {
	// Check if catalog exists
	info, err := os.Stat(path)
	needsUpdate := false

	if os.IsNotExist(err) {
		// Catalog doesn't exist - download it
		needsUpdate = true
	} else if err != nil {
		return nil, fmt.Errorf("stat catalog: %w", err)
	} else if forceUpdate {
		// Force update requested
		needsUpdate = true
	} else if time.Since(info.ModTime()) > maxAge {
		// Catalog is too old
		needsUpdate = true
	}

	if needsUpdate {
		// Download fresh catalog
		catalog, err := DownloadCatalog(path)
		if err != nil {
			return nil, fmt.Errorf("download catalog: %w", err)
		}
		return catalog, nil
	}

	// Load existing catalog
	catalog, err := LoadCatalog(path)
	if err != nil {
		return nil, fmt.Errorf("load catalog: %w", err)
	}

	return catalog, nil
}

// GetChartsForViewport returns charts covering the viewport at the specified zoom level.
//
// Charts are downloaded automatically if not already cached locally. The downloaded
// charts are cached in the XDG cache directory for future use.
//
// Zoom levels are automatically mapped to IHO usage bands.
//
// Example:
//
//	viewport := s57.Bounds{
//	    MinLon: -122.5, MaxLon: -122.0,
//	    MinLat: 37.5, MaxLat: 38.0,
//	}
//	charts, err := mgr.GetChartsForViewport(viewport, 12)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Printf("Loaded %d charts for rendering\n", len(charts))
func (m *ChartManager) GetChartsForViewport(viewport Bounds, zoom int) ([]*Chart, error) {
	return m.loader.GetChartsForViewport(viewport, zoom)
}

// GetChart loads a specific chart by name (cell identifier).
//
// The chart is downloaded automatically if not already cached locally.
//
// Example:
//
//	chart, err := mgr.GetChart("US5MA22M")
func (m *ChartManager) GetChart(name string) (*Chart, error) {
	return m.loader.GetChart(name)
}

// QueryCatalog returns catalog entries for charts intersecting the viewport.
//
// This returns metadata only, without downloading charts. Useful for showing
// available charts or their boundaries before loading.
//
// Example:
//
//	viewport := s57.Bounds{MinLon: -122.5, MaxLon: -122.0, MinLat: 37.5, MaxLat: 38.0}
//	entries := mgr.QueryCatalog(viewport)
//	fmt.Printf("Found %d charts covering this area:\n", len(entries))
//	for _, entry := range entries {
//	    fmt.Printf("  %s (%.1f MB) - Scale 1:%d\n",
//	        entry.Name, entry.SizeMB, entry.Scale)
//	}
func (m *ChartManager) QueryCatalog(viewport Bounds) []CatalogEntry {
	return m.catalog.Query(viewport)
}

// UpdateCatalog re-downloads the NOAA catalog from the official URL.
//
// This is useful for getting the latest chart updates from NOAA. The catalog
// is typically updated weekly.
//
// Example:
//
//	if err := mgr.UpdateCatalog(); err != nil {
//	    log.Printf("Failed to update catalog: %v", err)
//	}
func (m *ChartManager) UpdateCatalog() error {
	catalog, err := DownloadCatalog(m.catalogPath)
	if err != nil {
		return fmt.Errorf("download catalog: %w", err)
	}

	// Update internal catalog reference
	m.catalog = catalog

	// Rebuild loader index with new catalog (parser created internally)
	loader, err := NewChartLoaderFromCatalog(
		catalog,
		m.chartCacheDir,
		LoaderOptions{
			CacheSize: m.loader.cache.maxMemory,
		},
	)
	if err != nil {
		return fmt.Errorf("rebuild loader: %w", err)
	}

	m.loader = loader
	return nil
}

// ChartCount returns the total number of charts in the NOAA catalog.
func (m *ChartManager) ChartCount() int {
	return len(m.catalog.Entries)
}

// Loader returns the underlying chart loader for advanced use.
func (m *ChartManager) Loader() *ChartLoader {
	return m.loader
}

// Stats returns manager statistics including cache performance.
func (m *ChartManager) Stats() ChartManagerStats {
	loaderStats := m.loader.Stats()
	return ChartManagerStats{
		CatalogCharts: len(m.catalog.Entries),
		IndexedCharts: loaderStats.IndexedCharts,
		CachedCharts:  loaderStats.CachedCharts,
		CacheHits:     loaderStats.CacheHits,
		CacheMisses:   loaderStats.CacheMisses,
		CacheMemory:   loaderStats.CacheMemory,
		MaxMemory:     loaderStats.MaxMemory,
		CatalogPath:   m.catalogPath,
		ChartCacheDir: m.chartCacheDir,
	}
}

// ChartManagerStats holds manager statistics.
type ChartManagerStats struct {
	CatalogCharts int    // Total charts in catalog
	IndexedCharts int    // Charts in spatial index
	CachedCharts  int    // Charts in memory cache
	CacheHits     int    // Number of cache hits
	CacheMisses   int    // Number of cache misses
	CacheMemory   int64  // Current cache memory usage
	MaxMemory     int64  // Maximum cache memory limit
	CatalogPath   string // Path to catalog file
	ChartCacheDir string // Path to chart cache directory
}

// CacheHitRate returns the cache hit rate (0.0 to 1.0).
func (s ChartManagerStats) CacheHitRate() float64 {
	total := s.CacheHits + s.CacheMisses
	if total == 0 {
		return 0
	}
	return float64(s.CacheHits) / float64(total)
}
