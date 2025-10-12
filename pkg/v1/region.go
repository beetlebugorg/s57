package s57

import (
	"fmt"
	"os"
	"path/filepath"
)

// Region specifies a geographic area and loading parameters for regional chart loading.
//
// This is the primary high-level API for loading charts covering a specific area.
// It automatically handles chart discovery, spatial filtering, and parallel loading.
type Region struct {
	// Bounds defines the geographic area to load.
	// Charts that intersect this area will be loaded.
	Bounds Bounds

	// MinScale filters by minimum scale (larger scale, smaller denominator).
	// Optional. If 0, no minimum scale filter is applied.
	// Example: MinScale=20000 includes 1:20000 and 1:10000, excludes 1:50000.
	MinScale int

	// MaxScale filters by maximum scale (smaller scale, larger denominator).
	// Optional. If 0, no maximum scale filter is applied.
	// Example: MaxScale=100000 includes 1:100000 and 1:250000, excludes 1:50000.
	MaxScale int

	// UsageBands filters by usage band.
	// Optional. If empty, all usage bands are included.
	// Example: []UsageBand{UsageBandApproach, UsageBandHarbour}
	UsageBands []UsageBand

	// Progress is an optional callback for tracking loading progress.
	// Called periodically during chart loading.
	Progress func(loaded, total int)

	// ErrorLog is an optional writer for detailed error logging.
	ErrorLog *os.File
}

// LoadRegion is the recommended high-level API for loading charts covering a region.
//
// This function automatically:
//  1. Discovers all charts in the directory tree
//  2. Builds a spatial index
//  3. Queries for charts intersecting the region
//  4. Filters by scale and usage band (if specified)
//  5. Loads matching charts in parallel
//
// This is the simplest way to load charts for a specific geographic area.
//
// Example - Load harbour charts for San Francisco Bay:
//
//	parser := s57.NewParser()
//	cells, err := s57.LoadRegion(
//	    "/tmp/noaa_encs/ENC_ROOT",
//	    parser,
//	    s57.Region{
//	        Bounds: s57.Bounds{
//	            MinLon: -122.5, MaxLon: -122.0,
//	            MinLat: 37.5, MaxLat: 38.0,
//	        },
//	        MinScale: 10000,
//	        MaxScale: 100000,
//	        UsageBands: []s57.UsageBand{
//	            s57.UsageBandApproach,
//	            s57.UsageBandHarbour,
//	        },
//	        Progress: func(loaded, total int) {
//	            fmt.Printf("\rLoading: %d/%d", loaded, total)
//	        },
//	    },
//	)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Printf("\nLoaded %d charts\n", len(cells.Cells))
//
// Example - Load all charts for Florida:
//
//	cells, err := s57.LoadRegion(
//	    "/tmp/noaa_encs/ENC_ROOT",
//	    parser,
//	    s57.Region{
//	        Bounds: s57.Bounds{
//	            MinLon: -87.0, MaxLon: -80.0,
//	            MinLat: 24.0, MaxLat: 31.0,
//	        },
//	    },
//	)
func LoadRegion(root string, parser Parser, region Region) (*CellSet, error) {
	// Build index from directory
	// Use default parallel loading for index building
	indexOpts := DefaultLoadOptions()
	if region.Progress != nil {
		indexOpts.Progress = region.Progress
	}
	if region.ErrorLog != nil {
		indexOpts.ErrorLog = region.ErrorLog
	}

	idx, err := BuildIndexFromDir(root, parser, indexOpts)
	if err != nil {
		return nil, fmt.Errorf("build index: %w", err)
	}

	// Query for charts in region
	queryOpts := QueryOptions{
		MinScale:   region.MinScale,
		MaxScale:   region.MaxScale,
		UsageBands: region.UsageBands,
	}

	charts := idx.Query(region.Bounds, queryOpts)
	if len(charts) == 0 {
		return &CellSet{Cells: []*Cell{}}, nil
	}

	// Extract paths for loading
	// Note: ChartEntry.Path may be empty if index was built from CellSet
	// In that case, we need to reload from the original files
	// For now, we'll just return an error if paths aren't available
	paths := make([]string, len(charts))
	for i, chart := range charts {
		if chart.Path == "" {
			return nil, fmt.Errorf("chart path not available in index (chart: %s)", chart.Name)
		}
		paths[i] = chart.Path
	}

	// Load filtered charts in parallel
	loadOpts := DefaultLoadOptions()
	if region.Progress != nil {
		loadOpts.Progress = region.Progress
	}
	if region.ErrorLog != nil {
		loadOpts.ErrorLog = region.ErrorLog
	}

	cellSet, errs := LoadCellsParallel(paths, parser, loadOpts)
	if len(errs) > 0 && len(cellSet.Cells) == 0 {
		// All charts failed to load
		return nil, fmt.Errorf("failed to load any charts (%d errors)", len(errs))
	}

	// Return cellSet even if there were some errors (they were logged via ErrorLog)
	return cellSet, nil
}

// LoadRegionWithIndex is similar to LoadRegion but uses a pre-built index.
//
// This is more efficient when loading multiple regions from the same dataset,
// as the index only needs to be built once.
//
// Example:
//
//	// Build index once
//	parser := s57.NewParser()
//	idx, err := s57.BuildIndexFromDir("/tmp/noaa_encs/ENC_ROOT", parser,
//	    s57.DefaultLoadOptions())
//
//	// Load multiple regions using the same index
//	sfBay := s57.Region{
//	    Bounds: s57.Bounds{MinLon: -122.5, MaxLon: -122.0, MinLat: 37.5, MaxLat: 38.0},
//	}
//	sfCells, _ := s57.LoadRegionWithIndex(idx, parser, sfBay)
//
//	laBay := s57.Region{
//	    Bounds: s57.Bounds{MinLon: -118.5, MaxLon: -118.0, MinLat: 33.5, MaxLat: 34.0},
//	}
//	laCells, _ := s57.LoadRegionWithIndex(idx, parser, laBay)
func LoadRegionWithIndex(idx *ChartIndex, parser Parser, region Region) (*CellSet, error) {
	// Query for charts in region
	queryOpts := QueryOptions{
		MinScale:   region.MinScale,
		MaxScale:   region.MaxScale,
		UsageBands: region.UsageBands,
	}

	charts := idx.Query(region.Bounds, queryOpts)
	if len(charts) == 0 {
		return &CellSet{Cells: []*Cell{}}, nil
	}

	// Extract paths for loading
	paths := make([]string, len(charts))
	for i, chart := range charts {
		if chart.Path == "" {
			return nil, fmt.Errorf("chart path not available in index (chart: %s)", chart.Name)
		}
		paths[i] = chart.Path
	}

	// Load filtered charts in parallel
	loadOpts := DefaultLoadOptions()
	if region.Progress != nil {
		loadOpts.Progress = region.Progress
	}
	if region.ErrorLog != nil {
		loadOpts.ErrorLog = region.ErrorLog
	}

	cellSet, errs := LoadCellsParallel(paths, parser, loadOpts)
	if len(errs) > 0 && len(cellSet.Cells) == 0 {
		// All charts failed to load
		return nil, fmt.Errorf("failed to load any charts (%d errors)", len(errs))
	}

	// Return cellSet even if there were some errors (they were logged via ErrorLog)
	return cellSet, nil
}

// DiscoverCharts finds all .000 files (base cells) in a directory tree.
//
// This is a lower-level function for chart discovery. Most users should
// use LoadRegion() instead, which handles discovery automatically.
//
// Example:
//
//	paths, err := s57.DiscoverCharts("/tmp/noaa_encs/ENC_ROOT")
//	fmt.Printf("Found %d charts\n", len(paths))
func DiscoverCharts(root string) ([]string, error) {
	var paths []string

	// Walk directory tree
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && filepath.Ext(path) == ".000" {
			paths = append(paths, path)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk directory: %w", err)
	}

	return paths, nil
}
