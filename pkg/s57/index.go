package s57

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/dhconnelly/rtreego"
)

// ChartIndex provides fast spatial queries over a collection of charts.
//
// The index stores lightweight metadata for each chart (bounds, scale, edition)
// and supports efficient spatial filtering using an R-tree spatial index.
// This allows loading only charts that intersect a region of interest,
// dramatically reducing load time for regional rendering.
//
// Spatial queries are O(log N) with the R-tree, compared to O(N) with linear scan.
//
// Example:
//
//	// Build index from a directory
//	idx, err := s57.BuildIndexFromDir("/tmp/noaa_encs/ENC_ROOT", parser)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Query for charts in a region
//	floridaBounds := s57.Bounds{
//	    MinLon: -87.0, MaxLon: -80.0,
//	    MinLat: 24.0, MaxLat: 31.0,
//	}
//	charts := idx.Query(floridaBounds, s57.QueryOptions{})
//
//	fmt.Printf("Found %d charts covering Florida\n", len(charts))
type ChartIndex struct {
	charts []ChartEntry
	rtree  *rtreego.Rtree // Spatial index for fast queries
}

// ChartEntry contains indexed metadata for a single chart.
type ChartEntry struct {
	Path             string    // Absolute path to .000 file
	Name             string    // Dataset name
	GeoBounds        Bounds    // Geographic coverage
	CompilationScale int       // Scale denominator (e.g., 50000 for 1:50000)
	Edition          int       // Edition number
	UpdateNumber     int       // Update number
	UsageBand        UsageBand // Intended usage band
}

// Bounds method for rtreego.Spatial interface.
// Converts geographic bounds to R-tree rectangle.
func (e ChartEntry) Bounds() rtreego.Rect {
	// Create point at southwest corner
	point := rtreego.Point{e.GeoBounds.MinLon, e.GeoBounds.MinLat}

	// Create lengths (width, height)
	lengths := []float64{
		e.GeoBounds.MaxLon - e.GeoBounds.MinLon,
		e.GeoBounds.MaxLat - e.GeoBounds.MinLat,
	}

	rect, _ := rtreego.NewRect(point, lengths)
	return rect
}

// QueryOptions controls spatial query behavior.
type QueryOptions struct {
	// MinScale filters charts by minimum scale (larger scale, smaller denominator).
	// Only charts at this scale or larger are returned.
	// Example: MinScale=20000 includes 1:20000 and 1:10000, excludes 1:50000.
	MinScale int

	// MaxScale filters charts by maximum scale (smaller scale, larger denominator).
	// Only charts at this scale or smaller are returned.
	// Example: MaxScale=100000 includes 1:100000 and 1:250000, excludes 1:50000.
	MaxScale int

	// UsageBands filters by usage band.
	// If non-empty, only charts matching these bands are returned.
	// Example: []UsageBand{UsageBandApproach, UsageBandHarbour}
	UsageBands []UsageBand
}

// BuildIndexFromDir builds a chart index by scanning a directory tree.
//
// The function recursively searches for .000 files (base cells) and loads
// metadata from each chart. This is done in parallel for performance.
//
// The directory structure is typically:
//
//	ENC_ROOT/
//	  CHART1/
//	    CHART1.000
//	  CHART2/
//	    CHART2.000
//
// Progress can be monitored via LoadOptions.Progress callback.
//
// Example:
//
//	parser := s57.NewParser()
//	idx, err := s57.BuildIndexFromDir("/tmp/noaa_encs/ENC_ROOT", parser,
//	    s57.LoadOptions{
//	        Parallel:   true,
//	        SkipErrors: true,
//	        Progress: func(loaded, total int) {
//	            fmt.Printf("\rIndexing: %d/%d", loaded, total)
//	        },
//	    })
func BuildIndexFromDir(root string, parser Parser, opts LoadOptions) (*ChartIndex, error) {
	// Find all .000 files
	var paths []string
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

	if len(paths) == 0 {
		return nil, fmt.Errorf("no charts found in %s", root)
	}

	// Load charts in parallel
	cellSet, errs := LoadCellsParallel(paths, parser, opts)
	if len(cellSet.Cells) == 0 {
		return nil, fmt.Errorf("no charts could be loaded (%d errors)", len(errs))
	}

	// Build index from loaded cells
	return BuildIndex(cellSet), nil
}

// BuildIndex creates an index from a loaded CellSet.
//
// This is useful when you've already loaded charts and want to create
// an index for spatial queries. The function builds both a linear array
// and an R-tree spatial index for efficient queries.
func BuildIndex(cellSet *CellSet) *ChartIndex {
	entries := make([]ChartEntry, len(cellSet.Cells))

	// Create R-tree (2D, min=25 children, max=50 children)
	rtree := rtreego.NewTree(2, 25, 50)

	for i, cell := range cellSet.Cells {
		entries[i] = ChartEntry{
			Path:             "", // Path not available from Cell (TODO: add to Cell struct)
			Name:             cell.Chart.DatasetName(),
			GeoBounds:        cell.Chart.Bounds(),
			CompilationScale: cell.CompilationScale,
			Edition:          cell.Edition(),
			UpdateNumber:     cell.UpdateNumber(),
			UsageBand:        cell.Chart.UsageBand(),
		}

		// Insert into R-tree for spatial indexing
		rtree.Insert(entries[i])
	}

	return &ChartIndex{
		charts: entries,
		rtree:  rtree,
	}
}

// Query returns charts intersecting the given bounds, sorted by priority.
//
// Uses R-tree spatial index for efficient O(log N) queries instead of O(N) linear scan.
//
// Priority ordering (per S-52 Section 10.3.5):
//  1. Scale: Larger scale (smaller denominator) has priority
//  2. Edition: Higher edition number has priority
//  3. Update: Higher update number has priority
//
// QueryOptions can filter by scale range and usage bands.
//
// Example:
//
//	// Find approach and harbour charts in San Francisco Bay
//	charts := idx.Query(
//	    s57.Bounds{MinLon: -122.5, MaxLon: -122.0, MinLat: 37.5, MaxLat: 38.0},
//	    s57.QueryOptions{
//	        MinScale: 10000,  // 1:10000 or larger
//	        MaxScale: 100000, // 1:100000 or smaller
//	        UsageBands: []s57.UsageBand{
//	            s57.UsageBandApproach,
//	            s57.UsageBandHarbour,
//	        },
//	    },
//	)
func (idx *ChartIndex) Query(bounds Bounds, opts QueryOptions) []ChartEntry {
	var result []ChartEntry

	// Use R-tree for spatial query if available
	if idx.rtree != nil {
		// Convert bounds to rtreego.Rect
		point := rtreego.Point{bounds.MinLon, bounds.MinLat}
		lengths := []float64{
			bounds.MaxLon - bounds.MinLon,
			bounds.MaxLat - bounds.MinLat,
		}
		queryRect, _ := rtreego.NewRect(point, lengths)

		// Query R-tree for intersecting charts
		spatials := idx.rtree.SearchIntersect(queryRect)

		// Extract ChartEntry from Spatial interface
		for _, spatial := range spatials {
			entry := spatial.(ChartEntry)

			// Apply scale filters
			if opts.MinScale > 0 && entry.CompilationScale > opts.MinScale {
				continue // Chart scale too small (denominator too large)
			}
			if opts.MaxScale > 0 && entry.CompilationScale < opts.MaxScale {
				continue // Chart scale too large (denominator too small)
			}

			// Apply usage band filter
			if len(opts.UsageBands) > 0 {
				match := false
				for _, band := range opts.UsageBands {
					if entry.UsageBand == band {
						match = true
						break
					}
				}
				if !match {
					continue
				}
			}

			result = append(result, entry)
		}
	} else {
		// Fallback to linear scan if no R-tree (shouldn't happen)
		for _, entry := range idx.charts {
			// Check spatial intersection
			if !bounds.Intersects(entry.GeoBounds) {
				continue
			}

			// Apply scale filters
			if opts.MinScale > 0 && entry.CompilationScale > opts.MinScale {
				continue // Chart scale too small (denominator too large)
			}
			if opts.MaxScale > 0 && entry.CompilationScale < opts.MaxScale {
				continue // Chart scale too large (denominator too small)
			}

			// Apply usage band filter
			if len(opts.UsageBands) > 0 {
				match := false
				for _, band := range opts.UsageBands {
					if entry.UsageBand == band {
						match = true
						break
					}
				}
				if !match {
					continue
				}
			}

			result = append(result, entry)
		}
	}

	// Sort by priority
	sort.Slice(result, func(i, j int) bool {
		// Priority 1: Scale (smaller denominator = larger scale = higher priority)
		if result[i].CompilationScale != result[j].CompilationScale {
			return result[i].CompilationScale < result[j].CompilationScale
		}

		// Priority 2: Edition (higher = newer)
		if result[i].Edition != result[j].Edition {
			return result[i].Edition > result[j].Edition
		}

		// Priority 3: Update (higher = newer)
		return result[i].UpdateNumber > result[j].UpdateNumber
	})

	return result
}

// Count returns the total number of charts in the index.
func (idx *ChartIndex) Count() int {
	return len(idx.charts)
}

// Bounds returns the union of all chart bounds in the index.
func (idx *ChartIndex) Bounds() Bounds {
	if len(idx.charts) == 0 {
		return Bounds{}
	}

	bounds := idx.charts[0].GeoBounds
	for i := 1; i < len(idx.charts); i++ {
		bounds = bounds.Union(idx.charts[i].GeoBounds)
	}

	return bounds
}

// All returns all chart entries in the index.
func (idx *ChartIndex) All() []ChartEntry {
	return idx.charts
}
