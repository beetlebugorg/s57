package main

import (
	"fmt"
	"log"

	"github.com/beetlebugorg/s57/pkg/s57"
)

func main() {
	// Parse chart
	parser := s57.NewParser()
	chart, err := parser.Parse("US5MA22M.000")
	if err != nil {
		log.Fatal(err)
	}

	// Define viewport (Boston Harbor area)
	viewport := s57.Bounds{
		MinLon: -71.1, MaxLon: -71.0,
		MinLat: 42.3, MaxLat: 42.4,
	}

	// Query R-tree index for visible features (O(log n))
	features := chart.FeaturesInBounds(viewport)

	fmt.Printf("Visible features: %d\n", len(features))

	for _, feature := range features {
		geom := feature.Geometry()
		fmt.Printf("  %s: %s\n",
			feature.ObjectClass(),
			geom.Type)
	}
}
