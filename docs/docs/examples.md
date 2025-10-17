---
sidebar_position: 4
---

# Examples

Practical code examples for common S-57 parsing tasks.

## Quick Start

```go
package main

import (
    "fmt"
    "log"

    "github.com/beetlebugorg/s57/pkg/s57"
)

func main() {
    // Create parser
    parser := s57.NewParser()

    // Parse chart file
    chart, err := parser.Parse("US5MA22M.000")
    if err != nil {
        log.Fatal(err)
    }

    // Print chart info
    fmt.Printf("Chart: %s\n", chart.DatasetName())
    fmt.Printf("Edition: %s\n", chart.Edition())
    fmt.Printf("Features: %d\n", chart.FeatureCount())

    // Get chart bounds
    bounds := chart.Bounds()
    fmt.Printf("Bounds: [%.4f,%.4f] to [%.4f,%.4f]\n",
        bounds.MinLon, bounds.MinLat,
        bounds.MaxLon, bounds.MaxLat)
}
```

## Viewport Queries

Get features visible in a specific geographic area:

```go
func renderViewport(chart *s57.Chart, viewport s57.Bounds) {
    // Query R-tree index for visible features (O(log n))
    features := chart.FeaturesInBounds(viewport)

    fmt.Printf("Visible features: %d\n", len(features))

    for _, feature := range features {
        fmt.Printf("  %s: %s\n",
            feature.ObjectClass,
            feature.Geometry.Type)
    }
}

// Example viewport (Boston Harbor area)
viewport := s57.Bounds{
    MinLon: -71.1, MaxLon: -71.0,
    MinLat: 42.3, MaxLat: 42.4,
}
renderViewport(chart, viewport)
```

## Feature Filtering

Extract specific feature types:

```go
// Get all depth contours
func getDepthContours(chart *s57.Chart) []s57.Feature {
    var contours []s57.Feature
    for _, f := range chart.Features() {
        if f.ObjectClass == "DEPCNT" {
            contours = append(contours, f)
        }
    }
    return contours
}

// Get all navigation aids (buoys, lights, beacons)
func getNavAids(chart *s57.Chart) []s57.Feature {
    navAidClasses := map[string]bool{
        "BOYCAR": true, "BOYINB": true, "BOYISD": true,
        "BOYLAT": true, "BOYSAW": true, "BOYSPP": true,
        "BCNCAR": true, "BCNISD": true, "BCNLAT": true,
        "LIGHTS": true,
    }

    var navAids []s57.Feature
    for _, f := range chart.Features() {
        if navAidClasses[f.ObjectClass] {
            navAids = append(navAids, f)
        }
    }
    return navAids
}
```

## Working with Attributes

Access feature attributes safely:

```go
func printFeatureDetails(feature s57.Feature) {
    fmt.Printf("Feature: %s (ID %d)\n", feature.ObjectClass, feature.ID)

    // Object name (if present)
    if name, ok := feature.Attributes["OBJNAM"].(string); ok {
        fmt.Printf("  Name: %s\n", name)
    }

    // Depth value for depth contours
    if feature.ObjectClass == "DEPCNT" {
        if depth, ok := feature.Attributes["VALDCO"].(float64); ok {
            fmt.Printf("  Depth: %.1f meters\n", depth)
        }
    }

    // Light characteristics
    if feature.ObjectClass == "LIGHTS" {
        if color, ok := feature.Attributes["COLOUR"].([]int); ok {
            fmt.Printf("  Color codes: %v\n", color)
        }
        if height, ok := feature.Attributes["HEIGHT"].(float64); ok {
            fmt.Printf("  Height: %.1f meters\n", height)
        }
    }

    // Sounding depth
    if feature.ObjectClass == "SOUNDG" {
        if depth, ok := feature.Attributes["VALSOU"].(float64); ok {
            fmt.Printf("  Sounding: %.1f meters\n", depth)
        }
    }
}
```

## Geometry Processing

Work with feature geometries:

```go
func processGeometry(feature s57.Feature) {
    geom := feature.Geometry

    switch geom.Type {
    case s57.GeometryTypePoint:
        // Single point
        lon, lat := geom.Coordinates[0][0], geom.Coordinates[0][1]
        fmt.Printf("Point: %.6f, %.6f\n", lon, lat)

    case s57.GeometryTypeLineString:
        // Multiple connected points
        fmt.Printf("LineString with %d points:\n", len(geom.Coordinates))
        for i, coord := range geom.Coordinates {
            fmt.Printf("  %d: %.6f, %.6f\n", i, coord[0], coord[1])
        }

    case s57.GeometryTypePolygon:
        // Closed ring (first point == last point)
        fmt.Printf("Polygon with %d vertices:\n", len(geom.Coordinates)-1)
        for i, coord := range geom.Coordinates {
            fmt.Printf("  %d: %.6f, %.6f\n", i, coord[0], coord[1])
        }
    }
}

// Calculate line length (simplified, assumes small distances)
func lineLength(geom s57.Geometry) float64 {
    if geom.Type != s57.GeometryTypeLineString {
        return 0
    }

    length := 0.0
    for i := 1; i < len(geom.Coordinates); i++ {
        prev := geom.Coordinates[i-1]
        curr := geom.Coordinates[i]

        dx := curr[0] - prev[0]
        dy := curr[1] - prev[1]
        length += math.Sqrt(dx*dx + dy*dy)
    }
    return length
}
```

## Update File Handling

Handle S-57 update files:

```go
// Default: automatic update application
func parseWithUpdates(basePath string) {
    parser := s57.NewParser()

    // Automatically finds and applies .001, .002, .003, etc.
    chart, err := parser.Parse("GB5X01SW.000")
    if err != nil {
        log.Fatal(err)
    }

    fmt.Printf("Dataset: %s\n", chart.DatasetName())
    fmt.Printf("Edition: %s\n", chart.Edition())
    fmt.Printf("Update: %s\n", chart.UpdateNumber())
    fmt.Printf("Features: %d\n", chart.FeatureCount())
}

// Parse base cell only (no updates)
func parseBaseOnly(basePath string) {
    parser := s57.NewParser()

    opts := s57.ParseOptions{
        ApplyUpdates: false,
    }

    chart, err := parser.ParseWithOptions("GB5X01SW.000", opts)
    if err != nil {
        log.Fatal(err)
    }

    fmt.Printf("Update: %s (base cell only)\n", chart.UpdateNumber())
}
```

## Chart Catalog

Process multiple charts:

```go
type ChartInfo struct {
    Path     string
    Name     string
    Bounds   s57.Bounds
    Features int
}

func buildCatalog(chartPaths []string) ([]ChartInfo, error) {
    parser := s57.NewParser()
    catalog := make([]ChartInfo, 0, len(chartPaths))

    for _, path := range chartPaths {
        chart, err := parser.Parse(path)
        if err != nil {
            log.Printf("Failed to parse %s: %v\n", path, err)
            continue
        }

        info := ChartInfo{
            Path:     path,
            Name:     chart.DatasetName(),
            Bounds:   chart.Bounds(),
            Features: chart.FeatureCount(),
        }
        catalog = append(catalog, info)
    }

    return catalog, nil
}

// Find charts covering a location
func findChartsForLocation(catalog []ChartInfo, lon, lat float64) []ChartInfo {
    var matches []ChartInfo
    for _, info := range catalog {
        if info.Bounds.Contains(lon, lat) {
            matches = append(matches, info)
        }
    }
    return matches
}
```

## Performance Optimization

Optimize parsing for specific use cases:

```go
// Parse only specific features for faster loading
func parseDepthDataOnly(path string) (*s57.Chart, error) {
    parser := s57.NewParser()

    opts := s57.ParseOptions{
        ObjectClassFilter: []string{
            "DEPCNT", // Depth contours
            "DEPARE", // Depth areas
            "SOUNDG", // Soundings
        },
    }

    return parser.ParseWithOptions(path, opts)
}

// Strict mode with validation
func parseWithValidation(path string) (*s57.Chart, error) {
    parser := s57.NewParser()

    opts := s57.ParseOptions{
        ValidateGeometry:    true,
        SkipUnknownFeatures: true,
    }

    return parser.ParseWithOptions(path, opts)
}
```

## Error Handling

Robust error handling:

```go
func safeParseChart(path string) (*s57.Chart, error) {
    parser := s57.NewParser()

    chart, err := parser.Parse(path)
    if err != nil {
        // Check if file exists
        if os.IsNotExist(err) {
            return nil, fmt.Errorf("chart file not found: %s", path)
        }

        // Log detailed error
        log.Printf("Failed to parse %s: %v", path, err)
        return nil, err
    }

    // Validate chart data
    if chart.FeatureCount() == 0 {
        log.Printf("Warning: %s contains no features", path)
    }

    bounds := chart.Bounds()
    if bounds.MinLon == bounds.MaxLon || bounds.MinLat == bounds.MaxLat {
        log.Printf("Warning: %s has invalid bounds", path)
    }

    return chart, nil
}
```

## Complete Example: Chart Info Viewer

Prints chart metadata and feature statistics:

```go
package main

import (
    "flag"
    "fmt"
    "log"

    "github.com/beetlebugorg/s57/pkg/s57"
)

func main() {
    chartPath := flag.String("chart", "", "Path to S-57 chart file")
    flag.Parse()

    if *chartPath == "" {
        log.Fatal("Please provide -chart path")
    }

    // Parse chart
    parser := s57.NewParser()
    chart, err := parser.Parse(*chartPath)
    if err != nil {
        log.Fatal(err)
    }

    // Print metadata
    fmt.Printf("=== Chart Information ===\n")
    fmt.Printf("Dataset: %s\n", chart.DatasetName())
    fmt.Printf("Edition: %s\n", chart.Edition())
    fmt.Printf("Update: %s\n", chart.UpdateNumber())
    fmt.Printf("Features: %d\n\n", chart.FeatureCount())

    // Print bounds
    bounds := chart.Bounds()
    fmt.Printf("=== Geographic Bounds ===\n")
    fmt.Printf("Longitude: %.6f to %.6f\n", bounds.MinLon, bounds.MaxLon)
    fmt.Printf("Latitude: %.6f to %.6f\n\n", bounds.MinLat, bounds.MaxLat)

    // Count features by type
    counts := make(map[string]int)
    for _, f := range chart.Features() {
        counts[f.ObjectClass]++
    }

    fmt.Printf("=== Feature Types ===\n")
    for class, count := range counts {
        fmt.Printf("%-10s: %d\n", class, count)
    }
}
```

Usage:
```bash
go run viewer.go -chart US5MA22M.000
```
