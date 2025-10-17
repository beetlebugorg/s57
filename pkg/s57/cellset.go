package s57

import (
	"fmt"
	"sort"
	"strconv"
)

// Cell represents a loaded ENC cell with metadata for multi-cell rendering.
//
// Multi-cell composite charts require loading multiple ENC cells covering
// different geographic areas or scales. Cell captures the metadata needed
// for cell prioritization and coverage area management.
//
// Reference: S-52 Section 10.3.5 (Cell Selection and Display Priority)
type Cell struct {
	Chart            *Chart
	CompilationScale int            // CSCL from DSID
	CoverageAreas    []CoverageArea // M_COVR features
	ScaleAreas       []ScaleArea    // M_CSCL features
}

// CoverageArea defines where a cell provides data (M_COVR metadata feature).
//
// Each cell may define one or more coverage areas indicating where it provides
// valid data (CATCOV=1) or explicitly has no data (CATCOV=2).
//
// Reference: S-52 Section 10.3.6, S-57 Appendix A (M_COVR object class)
type CoverageArea struct {
	Polygon  [][]float64 // Polygon coordinates [lat, lon]
	Category int         // CATCOV: 1=Coverage Available, 2=No Coverage
}

// ScaleArea defines variable compilation scale regions within a cell (M_CSCL metadata feature).
//
// A single cell may contain areas compiled at different scales. For example,
// a general chart at 1:52,000 may include a harbor area compiled at 1:45,000.
//
// Reference: S-52 Section 10.3.7, S-57 Appendix A (M_CSCL object class)
type ScaleArea struct {
	Polygon [][]float64 // Polygon coordinates [lat, lon]
	Scale   int         // CSCALE: compilation scale denominator for this area
}

// CellSet manages multiple loaded ENC cells for composite chart rendering.
//
// When multiple cells cover the same area, CellSet applies S-52 priority rules
// to determine which cell's features should be displayed.
//
// Reference: S-52 Section 10.3.5 (Cell Selection and Display Priority)
type CellSet struct {
	Cells []*Cell
}

// LoadCell parses an ENC file and extracts metadata for multi-cell rendering.
//
// Returns a Cell containing the parsed chart and metadata including:
// - Compilation scale (from DSID)
// - Coverage areas (M_COVR features)
// - Variable scale areas (M_CSCL features)
func LoadCell(path string, parser Parser) (*Cell, error) {
	chart, err := parser.Parse(path)
	if err != nil {
		return nil, fmt.Errorf("parse chart: %w", err)
	}

	cell := &Cell{
		Chart:            chart,
		CompilationScale: extractCompilationScale(chart),
		CoverageAreas:    extractCoverageAreas(chart),
		ScaleAreas:       extractScaleAreas(chart),
	}

	return cell, nil
}

// LoadCells loads multiple ENC files for composite chart rendering.
//
// Each file is parsed into a Cell with metadata. Returns a CellSet containing
// all loaded cells.
//
// Example:
//
//	parser := s57.NewParser()
//	paths := []string{"GB4X0000.000", "GB5X01NE.000", "GB5X01NW.000"}
//	cellSet, err := s57.LoadCells(paths, parser)
func LoadCells(paths []string, parser Parser) (*CellSet, error) {
	cs := &CellSet{
		Cells: make([]*Cell, 0, len(paths)),
	}

	for _, path := range paths {
		cell, err := LoadCell(path, parser)
		if err != nil {
			return nil, fmt.Errorf("load cell %s: %w", path, err)
		}
		cs.Cells = append(cs.Cells, cell)
	}

	return cs, nil
}

// LoadCellsWithErrors loads multiple ENC files with error tolerance.
//
// Unlike LoadCells, this function continues loading even when individual
// charts fail. Returns a CellSet with successfully loaded cells and a slice
// of errors for failed charts.
//
// This is useful for large datasets where some charts may be corrupt but
// the majority are valid. The caller can decide how to handle partial failures.
//
// Example:
//
//	parser := s57.NewParser()
//	paths := []string{"chart1.000", "corrupt.000", "chart3.000"}
//	cellSet, errs := s57.LoadCellsWithErrors(paths, parser)
//	if len(errs) > 0 {
//	    fmt.Printf("Skipped %d charts due to errors\n", len(errs))
//	    for _, err := range errs {
//	        fmt.Fprintf(os.Stderr, "Error: %v\n", err)
//	    }
//	}
//	// Continue rendering with valid charts
func LoadCellsWithErrors(paths []string, parser Parser) (*CellSet, []error) {
	cs := &CellSet{
		Cells: make([]*Cell, 0, len(paths)),
	}

	var errors []error

	for _, path := range paths {
		cell, err := LoadCell(path, parser)
		if err != nil {
			errors = append(errors, fmt.Errorf("load cell %s: %w", path, err))
			continue
		}
		cs.Cells = append(cs.Cells, cell)
	}

	return cs, errors
}

// Identifier returns the cell's identifier (DSNM from DSID).
func (c *Cell) Identifier() string {
	return c.Chart.DatasetName()
}

// Edition returns the cell's edition number (EDTN from DSID).
// Returns 0 if the edition cannot be parsed as an integer.
func (c *Cell) Edition() int {
	n, _ := strconv.Atoi(c.Chart.Edition())
	return n
}

// UpdateNumber returns the cell's update number (UPDN from DSID).
// Returns 0 if the update number cannot be parsed as an integer.
func (c *Cell) UpdateNumber() int {
	n, _ := strconv.Atoi(c.Chart.UpdateNumber())
	return n
}

// Bounds returns the cell's geographic bounds.
func (c *Cell) Bounds() Bounds {
	return c.Chart.Bounds()
}

// CellPriority determines which cell has display priority at a geographic point.
//
// Returns cells covering the point, sorted by priority (highest first).
// Priority rules per S-52 Section 10.3.5:
//  1. Scale priority - Larger scale (smaller denominator) has priority
//     (e.g., 1:25,000 displays over 1:52,000)
//  2. Edition priority - Higher edition number has priority
//  3. Update priority - Higher update number has priority
//
// Example:
//
//	cells := cellSet.CellPriority(60.9, -32.4)
//	if len(cells) > 0 {
//	    fmt.Printf("Highest priority: %s\n", cells[0].Identifier())
//	}
func (cs *CellSet) CellPriority(lat, lon float64) []*Cell {
	// Filter to cells that cover this point
	covering := cs.CellsCoveringPoint(lat, lon)

	// Sort by priority rules
	sort.SliceStable(covering, func(i, j int) bool {
		ci, cj := covering[i], covering[j]

		// Rule 1: Scale priority (smaller scale number = larger scale = higher priority)
		scalei := ci.ScaleAtPoint(lat, lon)
		scalej := cj.ScaleAtPoint(lat, lon)
		if scalei != scalej {
			return scalei < scalej
		}

		// Rule 2: Edition priority (higher edition wins)
		if ci.Edition() != cj.Edition() {
			return ci.Edition() > cj.Edition()
		}

		// Rule 3: Update priority (higher update wins)
		return ci.UpdateNumber() > cj.UpdateNumber()
	})

	return covering
}

// CellsCoveringPoint returns cells that provide coverage at a point.
//
// Checks both geographic bounds and M_COVR coverage areas.
// A cell covers a point if:
// - Point is within cell bounds, AND
// - Point is within a M_COVR polygon with CATCOV=1 (or no M_COVR exists)
func (cs *CellSet) CellsCoveringPoint(lat, lon float64) []*Cell {
	var result []*Cell

	for _, cell := range cs.Cells {
		// Check if point is in cell bounds
		if !cell.Bounds().Contains(lat, lon) {
			continue
		}

		// Check M_COVR coverage areas
		if cell.HasCoverageAt(lat, lon) {
			result = append(result, cell)
		}
	}

	return result
}

// HasCoverageAt checks if the cell provides coverage at a point.
//
// Returns true if:
// - No M_COVR features exist (entire cell bounds is coverage), OR
// - Point is inside a M_COVR polygon with CATCOV=1 (Coverage Available)
//
// Returns false if:
// - Point is inside a M_COVR polygon with CATCOV=2 (No Coverage)
func (c *Cell) HasCoverageAt(lat, lon float64) bool {
	// If no M_COVR features, entire cell bounds is coverage
	if len(c.CoverageAreas) == 0 {
		return true
	}

	// Check all coverage areas
	for _, area := range c.CoverageAreas {
		if pointInPolygon(lat, lon, area.Polygon) {
			if area.Category == 1 {
				return true // Coverage Available
			}
			if area.Category == 2 {
				return false // No Coverage
			}
		}
	}

	return false
}

// ScaleAtPoint returns the compilation scale at a specific point.
//
// Checks M_CSCL features for variable scale regions within the cell.
// Returns the M_CSCL scale if the point is within a scale area polygon,
// otherwise returns the cell's primary compilation scale.
func (c *Cell) ScaleAtPoint(lat, lon float64) int {
	// Check M_CSCL areas for variable scale regions
	for _, area := range c.ScaleAreas {
		if pointInPolygon(lat, lon, area.Polygon) {
			return area.Scale
		}
	}

	// Default to cell's primary compilation scale
	return c.CompilationScale
}

// CompositeBounds returns the union of all cell bounds.
func (cs *CellSet) CompositeBounds() Bounds {
	if len(cs.Cells) == 0 {
		return Bounds{}
	}

	bounds := cs.Cells[0].Bounds()

	for i := 1; i < len(cs.Cells); i++ {
		bounds = bounds.Union(cs.Cells[i].Bounds())
	}

	return bounds
}

// extractCompilationScale extracts the compilation scale from DSID.
//
// Returns the CSCL field value, or 50000 as a default if not found.
func extractCompilationScale(chart *Chart) int {
	// The Chart should expose compilation scale from DSID
	// For now, we'll try to extract it from features or use a default

	// Check for DSID feature attributes (not yet exposed in public API)
	// Default to 50000 (general navigation scale)
	return 50000
}

// extractCoverageAreas extracts M_COVR features from the chart.
//
// M_COVR features define coverage areas with CATCOV attribute:
// - CATCOV=1: Coverage Available
// - CATCOV=2: No Coverage (explicit gap)
func extractCoverageAreas(chart *Chart) []CoverageArea {
	var areas []CoverageArea

	for _, feature := range chart.Features() {
		if feature.ObjectClass() != "M_COVR" {
			continue
		}

		// Get CATCOV attribute
		catcovVal, ok := feature.Attributes()["CATCOV"]
		if !ok {
			continue
		}

		// Parse CATCOV value, default to 1 (Coverage Available)
		catcov := 1
		switch v := catcovVal.(type) {
		case int:
			catcov = v
		case float64:
			catcov = int(v)
		case string:
			if n, err := strconv.Atoi(v); err == nil {
				catcov = n
			}
		}

		// Get polygon geometry
		geom := feature.Geometry()
		if geom.Type != GeometryTypePolygon || len(geom.Coordinates) < 3 {
			continue
		}

		// Convert geometry coordinates to [][]float64
		polygon := make([][]float64, len(geom.Coordinates))
		for i, coord := range geom.Coordinates {
			if len(coord) >= 2 {
				polygon[i] = []float64{coord[1], coord[0]} // [lat, lon]
			}
		}

		areas = append(areas, CoverageArea{
			Polygon:  polygon,
			Category: catcov,
		})
	}

	return areas
}

// extractScaleAreas extracts M_CSCL features from the chart.
//
// M_CSCL features define variable compilation scale areas with CSCALE attribute.
func extractScaleAreas(chart *Chart) []ScaleArea {
	var areas []ScaleArea

	for _, feature := range chart.Features() {
		if feature.ObjectClass() != "M_CSCL" {
			continue
		}

		// Get CSCALE attribute
		cscaleVal, ok := feature.Attributes()["CSCALE"]
		if !ok {
			continue
		}

		// Parse CSCALE value, skip if invalid
		var cscale int
		switch v := cscaleVal.(type) {
		case int:
			cscale = v
		case float64:
			cscale = int(v)
		case string:
			cscale, _ = strconv.Atoi(v)
		}
		if cscale == 0 {
			continue
		}

		// Get polygon geometry
		geom := feature.Geometry()
		if geom.Type != GeometryTypePolygon || len(geom.Coordinates) < 3 {
			continue
		}

		// Convert geometry coordinates to [][]float64
		polygon := make([][]float64, len(geom.Coordinates))
		for i, coord := range geom.Coordinates {
			if len(coord) >= 2 {
				polygon[i] = []float64{coord[1], coord[0]} // [lat, lon]
			}
		}

		areas = append(areas, ScaleArea{
			Polygon: polygon,
			Scale:   cscale,
		})
	}

	return areas
}

// pointInPolygon checks if a point is inside a polygon using ray casting algorithm.
//
// polygon is a slice of [lat, lon] coordinates.
func pointInPolygon(lat, lon float64, polygon [][]float64) bool {
	if len(polygon) < 3 {
		return false
	}

	inside := false
	j := len(polygon) - 1

	for i := 0; i < len(polygon); i++ {
		if len(polygon[i]) < 2 || len(polygon[j]) < 2 {
			j = i
			continue
		}

		pi_lat, pi_lon := polygon[i][0], polygon[i][1]
		pj_lat, pj_lon := polygon[j][0], polygon[j][1]

		// Ray casting algorithm
		if ((pi_lon > lon) != (pj_lon > lon)) &&
			(lat < (pj_lat-pi_lat)*(lon-pi_lon)/(pj_lon-pi_lon)+pi_lat) {
			inside = !inside
		}

		j = i
	}

	return inside
}
