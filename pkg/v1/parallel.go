package s57

import (
	"fmt"
	"io"
	"runtime"
	"sync"
)

// LoadOptions controls parallel loading behavior and error handling.
type LoadOptions struct {
	// Parallel enables concurrent chart loading.
	// When true, charts are loaded using multiple worker goroutines.
	Parallel bool

	// Workers specifies the number of parallel loader goroutines.
	// If 0, defaults to runtime.NumCPU().
	// Only used when Parallel is true.
	Workers int

	// SkipErrors causes loading to continue even when individual charts fail.
	// Failed charts are skipped and errors are collected.
	// When false, the first error stops loading and is returned immediately.
	SkipErrors bool

	// Progress is an optional callback for tracking loading progress.
	// Called after each chart is loaded (successfully or with error).
	// Parameters: (loaded, total) where loaded is count of charts processed so far.
	Progress func(loaded, total int)

	// ErrorLog is an optional writer for detailed error reporting.
	// Each loading error is written here with the chart path and error details.
	ErrorLog io.Writer
}

// DefaultLoadOptions returns load options with sensible defaults.
func DefaultLoadOptions() LoadOptions {
	return LoadOptions{
		Parallel:   true,
		Workers:    runtime.NumCPU(),
		SkipErrors: true,
		Progress:   nil,
		ErrorLog:   nil,
	}
}

// LoadCellsParallel loads multiple ENC charts in parallel with progress reporting.
//
// This function uses a worker pool pattern to load charts concurrently, significantly
// reducing total load time for large chart sets. With 8 CPU cores, loading 100 charts
// can be 6-8x faster than serial loading.
//
// The function respects LoadOptions:
//   - Parallel: Enable/disable parallel loading
//   - Workers: Number of concurrent loaders (defaults to NumCPU)
//   - SkipErrors: Continue loading despite individual chart failures
//   - Progress: Optional callback for progress updates
//   - ErrorLog: Optional writer for error details
//
// Example:
//
//	parser := s57.NewParser()
//	paths := []string{"chart1.000", "chart2.000", "chart3.000"}
//
//	cellSet, errs := s57.LoadCellsParallel(paths, parser, s57.LoadOptions{
//	    Parallel:   true,
//	    Workers:    8,
//	    SkipErrors: true,
//	    Progress: func(loaded, total int) {
//	        fmt.Printf("\rLoading: %d/%d (%.0f%%)",
//	            loaded, total, float64(loaded)/float64(total)*100)
//	    },
//	    ErrorLog: os.Stderr,
//	})
//
//	if len(errs) > 0 {
//	    fmt.Printf("\nSkipped %d charts due to errors\n", len(errs))
//	}
//	fmt.Printf("\nSuccessfully loaded %d charts\n", len(cellSet.Cells))
func LoadCellsParallel(paths []string, parser Parser, opts LoadOptions) (*CellSet, []error) {
	// Handle empty input
	if len(paths) == 0 {
		return &CellSet{Cells: []*Cell{}}, nil
	}

	// If parallel loading disabled, fall back to serial
	if !opts.Parallel {
		return loadCellsSerial(paths, parser, opts)
	}

	// Determine worker count
	workers := opts.Workers
	if workers <= 0 {
		workers = runtime.NumCPU()
	}

	// Don't create more workers than charts
	if workers > len(paths) {
		workers = len(paths)
	}

	// Create result channels
	type loadResult struct {
		index int
		cell  *Cell
		err   error
	}

	jobs := make(chan int, len(paths))
	results := make(chan loadResult, len(paths))

	// Start worker pool
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for index := range jobs {
				path := paths[index]
				cell, err := LoadCell(path, parser)
				results <- loadResult{
					index: index,
					cell:  cell,
					err:   err,
				}
			}
		}()
	}

	// Send jobs to workers
	for i := range paths {
		jobs <- i
	}
	close(jobs)

	// Wait for workers to finish in a separate goroutine
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect results
	cellMap := make(map[int]*Cell)
	var errors []error
	loaded := 0

	for result := range results {
		loaded++

		// Call progress callback
		if opts.Progress != nil {
			opts.Progress(loaded, len(paths))
		}

		// Handle errors
		if result.err != nil {
			err := fmt.Errorf("%s: %w", paths[result.index], result.err)

			// Log error if writer provided
			if opts.ErrorLog != nil {
				fmt.Fprintf(opts.ErrorLog, "Error loading chart: %v\n", err)
			}

			if opts.SkipErrors {
				errors = append(errors, err)
				continue
			} else {
				// Stop on first error
				return nil, []error{err}
			}
		}

		// Store successfully loaded cell
		cellMap[result.index] = result.cell
	}

	// Build ordered cell list
	cells := make([]*Cell, 0, len(cellMap))
	for i := 0; i < len(paths); i++ {
		if cell, ok := cellMap[i]; ok {
			cells = append(cells, cell)
		}
	}

	return &CellSet{Cells: cells}, errors
}

// loadCellsSerial loads charts one at a time (fallback when Parallel=false).
func loadCellsSerial(paths []string, parser Parser, opts LoadOptions) (*CellSet, []error) {
	cells := make([]*Cell, 0, len(paths))
	var errors []error

	for i, path := range paths {
		// Call progress callback
		if opts.Progress != nil {
			opts.Progress(i, len(paths))
		}

		cell, err := LoadCell(path, parser)
		if err != nil {
			err := fmt.Errorf("%s: %w", path, err)

			// Log error if writer provided
			if opts.ErrorLog != nil {
				fmt.Fprintf(opts.ErrorLog, "Error loading chart: %v\n", err)
			}

			if opts.SkipErrors {
				errors = append(errors, err)
				continue
			} else {
				return nil, []error{err}
			}
		}

		cells = append(cells, cell)
	}

	// Final progress callback
	if opts.Progress != nil {
		opts.Progress(len(paths), len(paths))
	}

	return &CellSet{Cells: cells}, errors
}
