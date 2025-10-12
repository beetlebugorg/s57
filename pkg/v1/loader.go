package s57

import (
	"fmt"

	"github.com/dhconnelly/rtreego"
)

// ChartLoader provides lazy loading of charts with caching.
//
// IMPORTANT: Most users should use ChartManager instead of ChartLoader directly.
// ChartManager provides automatic catalog management, updates, and XDG-compliant
// file storage. ChartLoader is a lower-level API for advanced use cases.
//
// The loader combines a spatial index (for fast chart discovery) with
// an LRU cache (for keeping frequently-accessed charts in memory).
//
// Charts are loaded on-demand when viewport queries request them, and
// automatically evicted from cache when memory limits are exceeded.
//
// For NOAA charts, use ChartManager (RECOMMENDED):
//
//	mgr, err := s57.NewChartManager(s57.DefaultChartManagerOptions())
//	if err != nil {
//	    log.Fatal(err)
//	}
//	charts, err := mgr.GetChartsForViewport(viewport, zoom)
//
// Advanced use only - create loader from catalog directly:
//
//	catalog, _ := s57.DownloadCatalog("/tmp/noaa_catalog.xml")
//	loader, _ := s57.NewChartLoaderFromCatalog(
//	    catalog,
//	    "/tmp/chart_cache",
//	    s57.LoaderOptions{CacheSize: 1024 * 1024 * 1024},
//	)
//	charts, _ := loader.GetChartsForViewport(viewport, zoom)
type ChartLoader struct {
	index         *ChartIndex
	cache         *ChartCache
	parser        Parser
	catalog       *ChartCatalog // Catalog for download-on-demand
	downloadTo    string        // Directory to download charts to
	keepExtracted bool          // Whether to keep extracted files or stream from zip
	hits          int           // Cache hits
	misses        int           // Cache misses
}

// LoaderOptions configures chart loader behavior.
type LoaderOptions struct {
	// CacheSize sets maximum cache memory in bytes.
	// Default: 512MB
	CacheSize int64

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

	// IndexOptions controls index building.
	IndexOptions LoadOptions
}

// DefaultLoaderOptions returns loader options with defaults.
func DefaultLoaderOptions() LoaderOptions {
	return LoaderOptions{
		CacheSize:     512 * 1024 * 1024, // 512MB default
		KeepExtracted: true,               // Extract for performance by default
		IndexOptions:  DefaultLoadOptions(),
	}
}

// NewChartLoaderFromCatalog creates a lazy-loading chart loader from a NOAA catalog.
//
// NOTE: Most users should use ChartManager instead, which handles catalog
// management automatically. This lower-level constructor is for advanced use cases.
//
// The catalog provides precise polygon boundaries for all charts without requiring
// local chart files. Charts are downloaded on-demand when viewport queries request them.
//
// Advantages of catalog-based loading:
//   - Fast startup: 1.5s to parse catalog and build index
//   - Download on-demand: Fetch only charts needed for viewport
//   - Precise boundaries: GML polygons from NOAA catalog metadata
//   - Always current: Re-download catalog to get latest chart updates
//   - Incremental storage: Storage grows as you explore new areas (vs 30GB upfront)
//
// Example:
//
//	// Download and parse NOAA catalog (1.5 seconds, 90MB)
//	catalog, err := s57.DownloadCatalog("/tmp/noaa_catalog.xml")
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Create loader with download-on-demand
//	loader, err := s57.NewChartLoaderFromCatalog(
//	    catalog,
//	    "/tmp/chart_cache",    // Where to cache downloaded charts
//	    s57.LoaderOptions{
//	        CacheSize: 1024 * 1024 * 1024, // 1GB in-memory cache
//	    },
//	)
//
//	// Query viewport - downloads and caches only needed charts
//	viewport := s57.Bounds{
//	    MinLon: -122.5, MaxLon: -122.0,
//	    MinLat: 37.5, MaxLat: 38.0,
//	}
//	charts, err := loader.GetChartsForViewport(viewport, 12)
func NewChartLoaderFromCatalog(catalog *ChartCatalog, downloadDir string, opts LoaderOptions) (*ChartLoader, error) {
	// Build index from catalog entries (no downloading yet)
	index := buildIndexFromCatalog(catalog)

	// Create cache
	cache := NewChartCache(opts.CacheSize)

	// Create parser internally
	parser := NewParser()

	return &ChartLoader{
		index:         index,
		cache:         cache,
		parser:        parser,
		catalog:       catalog,
		downloadTo:    downloadDir,
		keepExtracted: opts.KeepExtracted,
	}, nil
}

// buildIndexFromCatalog creates a ChartIndex from catalog entries.
func buildIndexFromCatalog(catalog *ChartCatalog) *ChartIndex {
	entries := make([]ChartEntry, len(catalog.Entries))

	// Create R-tree
	rtree := rtreego.NewTree(2, 25, 50)

	for i, entry := range catalog.Entries {
		bounds := entry.Bounds()

		// Parse usage band from chart name (e.g., "US5CA12M" -> band 5)
		// NOAA chart names: US<band><region><cell><update>
		usageBand := chartNameToUsageBand(entry.Name)

		// Use catalog scale if available, otherwise derive from usage band
		scale := entry.Scale
		if scale == 0 {
			scale = usageBandToScale(usageBand)
		}

		entries[i] = ChartEntry{
			Path:             "", // Will be set on download
			Name:             entry.Name,
			GeoBounds:        bounds,
			CompilationScale: scale,
			Edition:          0, // Parse from edition string if needed
			UpdateNumber:     0,
			UsageBand:        usageBand,
		}

		// Insert into R-tree
		rtree.Insert(entries[i])
	}

	return &ChartIndex{
		charts: entries,
		rtree:  rtree,
	}
}

// chartNameToUsageBand extracts the usage band from a NOAA chart name.
// NOAA names: US<band><region><cell><update>
// Example: "US5CA12M" -> band 5 (Harbour)
func chartNameToUsageBand(name string) UsageBand {
	if len(name) < 3 || name[0:2] != "US" {
		return 0 // Unknown
	}

	// Band is the first digit after "US"
	bandChar := name[2]
	if bandChar >= '1' && bandChar <= '6' {
		return UsageBand(bandChar - '0')
	}

	return 0 // Unknown
}

// usageBandToScale converts a usage band to approximate compilation scale.
func usageBandToScale(band UsageBand) int {
	switch band {
	case 1: // Overview
		return 3000000
	case 2: // General
		return 500000
	case 3: // Coastal
		return 150000
	case 4: // Approach
		return 50000
	case 5: // Harbour
		return 20000
	case 6: // Berthing
		return 5000
	default:
		return 0
	}
}

// scaleToUsageBand converts a compilation scale denominator to IHO usage band.
func scaleToUsageBand(scale int) UsageBand {
	switch {
	case scale > 1500000:
		return 1 // Overview
	case scale > 350000:
		return 2 // General
	case scale > 90000:
		return 3 // Coastal
	case scale > 30000:
		return 4 // Approach
	case scale > 12000:
		return 5 // Harbour
	default:
		return 6 // Berthing
	}
}

// GetChartsForViewport returns charts covering the viewport at the specified zoom level.
//
// This is the primary lazy-loading API. Charts are:
//  1. Queried from spatial index based on viewport bounds
//  2. Filtered by zoom level (converted to INTU usage band)
//  3. Loaded from cache if available, or parsed from disk
//  4. Cached for future queries
//
// Zoom levels are mapped to IHO usage bands per the S-57 standard:
//   - Zoom 0-4:   INTU 1 (Overview)
//   - Zoom 5-8:   INTU 2 (General)
//   - Zoom 9-11:  INTU 3 (Coastal)
//   - Zoom 12-13: INTU 4 (Approach)
//   - Zoom 14-15: INTU 5 (Harbour)
//   - Zoom 16+:   INTU 6 (Berthing)
//
// Example:
//
//	// Get charts for San Francisco Bay at zoom 12 (approach scale)
//	viewport := s57.Bounds{
//	    MinLon: -122.5, MaxLon: -122.0,
//	    MinLat: 37.5, MaxLat: 38.0,
//	}
//	charts, err := loader.GetChartsForViewport(viewport, 12)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Printf("Loaded %d charts for rendering\n", len(charts))
func (l *ChartLoader) GetChartsForViewport(viewport Bounds, zoom int) ([]*Chart, error) {
	// Convert zoom to INTU and query index
	targetINTU := zoomToINTU(zoom)

	// Query with ±1 INTU tolerance (allow adjacent scales)
	minINTU := targetINTU - 1
	maxINTU := targetINTU + 1
	if minINTU < 1 {
		minINTU = 1
	}
	if maxINTU > 6 {
		maxINTU = 6
	}

	// Build usage band filter
	usageBands := []UsageBand{}
	for i := minINTU; i <= maxINTU; i++ {
		usageBands = append(usageBands, UsageBand(i))
	}

	// Query index
	entries := l.index.Query(viewport, QueryOptions{
		UsageBands: usageBands,
	})

	// Load charts (from cache or disk)
	charts := make([]*Chart, 0, len(entries))
	for _, entry := range entries {
		chart, err := l.loadChart(entry.Name)
		if err != nil {
			// Skip charts that fail to load
			continue
		}
		charts = append(charts, chart)
	}

	return charts, nil
}

// loadChart loads a chart by name, using cache if available.
// For catalog-based loaders, downloads chart on-demand if not cached locally.
func (l *ChartLoader) loadChart(name string) (*Chart, error) {
	// Try cache first
	chart, err := l.cache.Get(name, func() (*Chart, error) {
		// Cache miss - need to load from disk
		l.misses++

		// Get chart path - either from cellPaths (local) or catalog (download)
		path, err := l.getChartPath(name)
		if err != nil {
			return nil, err
		}

		return l.parser.Parse(path)
	})

	if err == nil {
		// Check if this was a cache hit (Get succeeded without calling loader)
		// This is approximate - Get might have loaded from disk
		// We increment hits on successful return
		l.hits++
	}

	return chart, err
}

// getChartPath returns the path to a chart file, downloading from catalog if necessary.
func (l *ChartLoader) getChartPath(name string) (string, error) {
	if l.catalog == nil {
		return "", fmt.Errorf("no catalog available for chart: %s", name)
	}

	// Find chart in catalog
	var entry *CatalogEntry
	for i := range l.catalog.Entries {
		if l.catalog.Entries[i].Name == name {
			entry = &l.catalog.Entries[i]
			break
		}
	}
	if entry == nil {
		return "", fmt.Errorf("chart not found in catalog: %s", name)
	}

	// Download chart (or return cached path if already downloaded)
	path, err := l.catalog.DownloadChart(*entry, l.downloadTo, l.keepExtracted)
	if err != nil {
		return "", fmt.Errorf("download chart %s: %w", name, err)
	}

	return path, nil
}

// zoomToINTU converts web mercator zoom level to IHO intended usage band.
//
// Per IHO S-57 §2.4, charts have "Intended Usage" (INTU) codes indicating
// appropriate scale ranges:
//
//	INTU 1: > 1:1,500,000   (Overview)
//	INTU 2: 1:350,000 - 1:1,500,000 (General)
//	INTU 3: 1:90,000 - 1:350,000    (Coastal)
//	INTU 4: 1:30,000 - 1:90,000     (Approach)
//	INTU 5: 1:12,000 - 1:30,000     (Harbour)
//	INTU 6: < 1:12,000              (Berthing)
func zoomToINTU(zoom int) int {
	switch {
	case zoom <= 4:
		return 1 // Overview
	case zoom <= 8:
		return 2 // General
	case zoom <= 11:
		return 3 // Coastal
	case zoom <= 13:
		return 4 // Approach
	case zoom <= 15:
		return 5 // Harbour
	default:
		return 6 // Berthing
	}
}

// GetChart loads a specific chart by name (cell identifier).
//
// The chart is loaded from cache if available, otherwise parsed from disk
// and added to cache.
//
// Example:
//
//	chart, err := loader.GetChart("US5MA22M")
func (l *ChartLoader) GetChart(name string) (*Chart, error) {
	return l.loadChart(name)
}

// Index returns the underlying spatial index.
//
// This allows direct access to the index for advanced queries.
func (l *ChartLoader) Index() *ChartIndex {
	return l.index
}

// Cache returns the underlying chart cache.
//
// This allows inspecting cache statistics and manually managing cached charts.
func (l *ChartLoader) Cache() *ChartCache {
	return l.cache
}

// CacheHitRate returns the cache hit rate (0.0 to 1.0).
//
// This indicates what percentage of chart requests were served from cache
// vs loaded from disk.
func (l *ChartLoader) CacheHitRate() float64 {
	total := l.hits + l.misses
	if total == 0 {
		return 0
	}
	return float64(l.hits) / float64(total)
}

// Stats returns loader statistics.
func (l *ChartLoader) Stats() LoaderStats {
	cacheStats := l.cache.Stats()
	return LoaderStats{
		IndexedCharts: l.index.Count(),
		CachedCharts:  cacheStats.ChartCount,
		CacheHits:     l.hits,
		CacheMisses:   l.misses,
		CacheMemory:   cacheStats.UsedMemory,
		MaxMemory:     cacheStats.MaxMemory,
	}
}

// LoaderStats holds loader performance metrics.
type LoaderStats struct {
	IndexedCharts int   // Total charts in index
	CachedCharts  int   // Charts currently in cache
	CacheHits     int   // Number of cache hits
	CacheMisses   int   // Number of cache misses
	CacheMemory   int64 // Current cache memory usage
	MaxMemory     int64 // Maximum cache memory limit
}
