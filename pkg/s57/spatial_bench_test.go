package s57

import (
	"testing"
)

// Benchmark R-tree spatial index vs linear scan for viewport queries.
// This demonstrates the performance improvement from O(n) to O(log n).

// BenchmarkFeaturesInBounds_Rtree benchmarks viewport queries with R-tree index.
func BenchmarkFeaturesInBounds_Rtree(b *testing.B) {
	// Create a chart with 10,000 features spread across a region
	chart := createLargeChart(10000)
	chart.buildSpatialIndex() // Build R-tree index

	// Small viewport (typical zoom level - shows ~100 features)
	viewport := Bounds{
		MinLon: -71.1,
		MaxLon: -71.0,
		MinLat: 42.0,
		MaxLat: 42.1,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = chart.FeaturesInBounds(viewport)
	}
}

// BenchmarkFeaturesInBounds_Linear benchmarks viewport queries with linear scan.
func BenchmarkFeaturesInBounds_Linear(b *testing.B) {
	// Create a chart with 10,000 features spread across a region
	chart := createLargeChart(10000)
	// DON'T build spatial index - force linear scan
	chart.spatialIndex = nil

	// Small viewport (typical zoom level - shows ~100 features)
	viewport := Bounds{
		MinLon: -71.1,
		MaxLon: -71.0,
		MinLat: 42.0,
		MaxLat: 42.1,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = chart.FeaturesInBounds(viewport)
	}
}

// BenchmarkFeaturesInBounds_Rtree_LargeViewport benchmarks with large viewport.
func BenchmarkFeaturesInBounds_Rtree_LargeViewport(b *testing.B) {
	chart := createLargeChart(10000)
	chart.buildSpatialIndex()

	// Large viewport (zoomed out - shows ~1000 features)
	viewport := Bounds{
		MinLon: -72.0,
		MaxLon: -71.0,
		MinLat: 42.0,
		MaxLat: 43.0,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = chart.FeaturesInBounds(viewport)
	}
}

// BenchmarkFeaturesInBounds_Linear_LargeViewport benchmarks linear with large viewport.
func BenchmarkFeaturesInBounds_Linear_LargeViewport(b *testing.B) {
	chart := createLargeChart(10000)
	chart.spatialIndex = nil

	// Large viewport (zoomed out - shows ~1000 features)
	viewport := Bounds{
		MinLon: -72.0,
		MaxLon: -71.0,
		MinLat: 42.0,
		MaxLat: 43.0,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = chart.FeaturesInBounds(viewport)
	}
}

// BenchmarkBuildSpatialIndex benchmarks R-tree construction.
func BenchmarkBuildSpatialIndex(b *testing.B) {
	charts := make([]*Chart, b.N)
	for i := 0; i < b.N; i++ {
		charts[i] = createLargeChart(10000)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		charts[i].buildSpatialIndex()
	}
}

// createLargeChart creates a synthetic chart with many features for benchmarking.
func createLargeChart(numFeatures int) *Chart {
	features := make([]Feature, numFeatures)

	// Distribute features across a 2° x 2° region
	// This simulates a typical harbor chart
	lonMin, lonMax := -72.0, -70.0
	latMin, latMax := 42.0, 44.0

	for i := 0; i < numFeatures; i++ {
		// Create features at pseudo-random locations
		// Use simple deterministic pattern for reproducibility
		lon := lonMin + float64(i%1000)/1000.0*(lonMax-lonMin)
		lat := latMin + float64(i/1000)/float64(numFeatures/1000)*(latMax-latMin)

		// Mix of points, lines, and areas
		var geomType GeometryType
		var coords [][]float64

		switch i % 3 {
		case 0: // Point (buoy, light, etc)
			geomType = GeometryTypePoint
			coords = [][]float64{{lon, lat}}
		case 1: // Line (depth contour, cable, etc)
			geomType = GeometryTypeLineString
			coords = [][]float64{
				{lon, lat},
				{lon + 0.01, lat + 0.01},
				{lon + 0.02, lat},
			}
		case 2: // Area (restricted area, anchorage, etc)
			geomType = GeometryTypePolygon
			coords = [][]float64{
				{lon, lat},
				{lon + 0.01, lat},
				{lon + 0.01, lat + 0.01},
				{lon, lat + 0.01},
				{lon, lat}, // Close polygon
			}
		}

		features[i] = Feature{
			id: int64(i + 1),
			geometry: Geometry{
				Type:        geomType,
				Coordinates: coords,
			},
			objectClass: "TESOBJ",
			attributes:  make(map[string]interface{}),
		}
	}

	return &Chart{
		features: features,
	}
}
