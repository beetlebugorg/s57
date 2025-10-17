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
		counts[f.ObjectClass()]++
	}

	fmt.Printf("=== Feature Types ===\n")
	for class, count := range counts {
		fmt.Printf("%-10s: %d\n", class, count)
	}
}
