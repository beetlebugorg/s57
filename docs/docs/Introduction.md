---
slug: /
sidebar_position: 1
---

# S-57 Parser for Go

A pure Go parser for IHO S-57 Electronic Navigational Chart (ENC) files, the international standard for digital hydrographic data used worldwide in maritime navigation.

:::caution Learning Project

**Experimental software for learning and development only.** This project explores AI-assisted development with complex marine electronic navigational charts standards.

**⚠️ Not for production use or actual navigation.**

**Documentation as Specification:** This documentation serves as a living specification that drives development. We strive to ensure documented features are implemented and working, but we're actively refining our development and testing workflows. Some features may be in progress.

**Not affiliated with IHO (International Hydrographic Organization).** Independent third-party implementation of publicly available specifications.

:::

## What is S-57?

S-57 is the data transfer standard developed by the International Hydrographic Organization (IHO) for digital hydrographic data. It's used globally for Electronic Navigational Charts (ENCs) in maritime navigation systems (ECDIS).

## Key Features

- ✅ **IHO S-57 Edition 3.1 parsing**
- ✅ **All S-57 feature types** - Depth contours, buoys, lights, wrecks, and more
- ✅ **Spatial topology support** - Points, lines, and polygons with geometry construction
- ✅ **Automatic update merging** - Handles .001, .002, etc. update files automatically
- ✅ **Spatial indexing** - R-tree for viewport queries
- ✅ **Pure Go** - No unsafe code

## Use Cases

- **Learning** - Understand S-57 chart format and structure
- **GIS Applications** - Work with official hydrographic data
- **Maritime Research** - Analyze bathymetry, hazards, and navigation features
- **Chart Conversion** - Transform S-57 data to other formats

## How It Works

```go
// 1. Create parser
parser := s57.NewParser()

// 2. Parse ENC file (automatically applies updates)
chart, err := parser.Parse("US5MA22M.000")

// 3. Query features in a viewport
bounds := s57.Bounds{
    MinLon: -71.1, MaxLon: -71.0,
    MinLat: 42.3, MaxLat: 42.4,
}
features := chart.FeaturesInBounds(bounds)

// 4. Render or analyze features
for _, feature := range features {
    fmt.Printf("%s: %v\n", feature.ObjectClass, feature.Geometry)
}
```

## S-57 Data Structure

S-57 charts contain:

- **Feature Objects** - Navigation features like buoys, lights, depth areas
- **Spatial Objects** - Geometric primitives (nodes, edges, faces)
- **Attributes** - Descriptive data (depth values, colors, names)
- **Topology** - Relationships between spatial elements

The parser automatically:
- Builds complete geometries from spatial primitives
- Links features to their geometry
- Applies coordinate transformations
- Handles update files (INSERT/DELETE/MODIFY operations)

## Next Steps

- [Installation](installation.md) - Get started with the parser
- [API Reference](api-reference.md) - Complete API documentation
- [Examples](examples.md) - Code examples and common patterns
