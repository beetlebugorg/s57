package main

import (
	"fmt"
	"log"
	"os"

	"github.com/beetlebugorg/s57/pkg/s57"
)

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

func main() {
	// Try to parse a chart
	chart, err := safeParseChart("US5MA22M.000")
	if err != nil {
		log.Printf("Error: %v", err)
		return
	}

	fmt.Printf("Successfully loaded chart: %s\n", chart.DatasetName())
	fmt.Printf("Features: %d\n", chart.FeatureCount())

	// Try to parse a non-existent chart
	_, err = safeParseChart("NONEXISTENT.000")
	if err != nil {
		log.Printf("Expected error: %v", err)
	}
}
