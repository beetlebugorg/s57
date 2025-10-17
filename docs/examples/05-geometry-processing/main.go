package main

import (
	"fmt"
	"log"
	"math"

	"github.com/beetlebugorg/s57/pkg/s57"
)

func processGeometry(feature s57.Feature) {
	geom := feature.Geometry()

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

func main() {
	parser := s57.NewParser()
	chart, err := parser.Parse("US5MA22M.000")
	if err != nil {
		log.Fatal(err)
	}

	// Process first few features
	count := 0
	for _, f := range chart.Features() {
		fmt.Printf("\n%s:\n", f.ObjectClass())
		processGeometry(f)

		geom := f.Geometry()
		if geom.Type == s57.GeometryTypeLineString {
			fmt.Printf("Length: %.6f degrees\n", lineLength(geom))
		}

		count++
		if count >= 3 {
			break
		}
	}
}
