---
sidebar_position: 3
---

# API Reference

Complete reference for the S-57 parser API.

## Core Types

### Parser

The main entry point for parsing S-57 files.

```go
type Parser struct {
    // Configuration options
}

func NewParser() *Parser
```

**Methods:**

```go
// Parse a base cell file (automatically applies updates)
func (p *Parser) Parse(filepath string) (*Chart, error)

// Parse with custom options
func (p *Parser) ParseWithOptions(filepath string, opts ParseOptions) (*Chart, error)
```

### Chart

Represents a complete S-57 chart with features and metadata.

```go
type Chart struct {
    // Internal state
}
```

**Methods:**

```go
// Get all features in the chart
func (c *Chart) Features() []Feature

// Get features within a bounding box (uses R-tree index)
func (c *Chart) FeaturesInBounds(bounds Bounds) []Feature

// Get chart bounding box
func (c *Chart) Bounds() Bounds

// Get feature count
func (c *Chart) FeatureCount() int


// Get dataset name (chart ID)
func (c *Chart) DatasetName() string

// Get edition number
func (c *Chart) Edition() string

// Get update number (0 for base cell)
func (c *Chart) UpdateNumber() string
```

### Feature

Represents an S-57 feature object (buoy, depth area, etc.).

```go
type Feature struct {
    ID          int64                  // Feature record ID
    ObjectClass string                 // S-57 object class (e.g., "DEPCNT", "BOYCAR")
    Geometry    Geometry               // Spatial representation
    Attributes  map[string]interface{} // Feature attributes
}
```

**Common Object Classes:**

- `DEPCNT` - Depth contour
- `DEPARE` - Depth area
- `BOYCAR` - Cardinal buoy
- `LIGHTS` - Light
- `LNDARE` - Land area
- `COALNE` - Coastline
- `WRECKS` - Wreck
- `OBSTRN` - Obstruction

### Geometry

Spatial representation of a feature.

```go
type Geometry struct {
    Type        GeometryType // Point, LineString, or Polygon
    Coordinates [][]float64  // [lon, lat] pairs
}

type GeometryType string

const (
    GeometryTypePoint      GeometryType = "Point"
    GeometryTypeLineString GeometryType = "LineString"
    GeometryTypePolygon    GeometryType = "Polygon"
)
```

**Coordinate Format:**

Coordinates are always `[longitude, latitude]` in decimal degrees (WGS84).

```go
// Point: single coordinate
geometry.Coordinates[0] // [lon, lat] or [lon, lat, depth] for soundings

// LineString: array of coordinates
for _, coord := range geometry.Coordinates {
    lon, lat := coord[0], coord[1]
}

// Polygon: array of coordinates (first == last)
for _, coord := range geometry.Coordinates {
    lon, lat := coord[0], coord[1]
}
```

### Bounds

Geographic bounding box.

```go
type Bounds struct {
    MinLon float64 // West longitude
    MaxLon float64 // East longitude
    MinLat float64 // South latitude
    MaxLat float64 // North latitude
}

// Check if two bounds intersect
func (b Bounds) Intersects(other Bounds) bool

// Check if bounds contains a point
func (b Bounds) Contains(lon, lat float64) bool
```

### ParseOptions

Control parsing behavior.

```go
type ParseOptions struct {
    // Apply update files (.001, .002, etc.) automatically
    ApplyUpdates bool // default: true

    // Skip features with unknown object classes
    SkipUnknownFeatures bool // default: false

    // Validate geometry during construction
    ValidateGeometry bool // default: false

    // Filter to specific object classes (empty = parse all)
    ObjectClassFilter []string
}
```

**Examples:**

```go
// Parse only depth-related features
opts := s57.ParseOptions{
    ObjectClassFilter: []string{"DEPCNT", "DEPARE", "SOUNDG"},
}

// Parse without applying updates
opts := s57.ParseOptions{
    ApplyUpdates: false,
}

// Strict validation
opts := s57.ParseOptions{
    ValidateGeometry: true,
    SkipUnknownFeatures: true,
}
```

## Common Patterns

### Basic Parsing

```go
parser := s57.NewParser()
chart, err := parser.Parse("US5MA22M.000")
if err != nil {
    log.Fatal(err)
}
```

### Viewport Query

```go
// Get features visible in viewport
bounds := s57.Bounds{
    MinLon: -71.1, MaxLon: -71.0,
    MinLat: 42.3, MaxLat: 42.4,
}
visible := chart.FeaturesInBounds(bounds)
```

### Feature Filtering

```go
// Get all depth contours
var contours []s57.Feature
for _, f := range chart.Features() {
    if f.ObjectClass == "DEPCNT" {
        contours = append(contours, f)
    }
}
```

### Attribute Access

```go
// Safe attribute access with type checking
if depth, ok := feature.Attributes["VALDCO"].(float64); ok {
    fmt.Printf("Depth: %.1f meters\n", depth)
}

// Common attributes
if name, ok := feature.Attributes["OBJNAM"].(string); ok {
    fmt.Printf("Name: %s\n", name)
}
```

### Update Handling

```go
// Default: automatically apply all updates
chart, _ := parser.Parse("GB5X01SW.000")
fmt.Printf("Update: %s\n", chart.UpdateNumber()) // "003" if .001,.002,.003 exist

// Disable updates
opts := s57.ParseOptions{ApplyUpdates: false}
chart, _ := parser.ParseWithOptions("GB5X01SW.000", opts)
fmt.Printf("Update: %s\n", chart.UpdateNumber()) // "000"
```

## Error Handling

All parsing errors return detailed error information:

```go
chart, err := parser.Parse("invalid.000")
if err != nil {
    // Errors include file path and specific issue
    log.Printf("Parse error: %v\n", err)
    return
}
```

Common errors:
- File not found
- Invalid ISO 8211 format
- Missing required S-57 records
- Corrupt geometry data
- Update file sequence gaps
