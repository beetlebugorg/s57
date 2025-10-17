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
