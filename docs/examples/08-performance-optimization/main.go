package main

import (
	"fmt"
	"log"

	"github.com/beetlebugorg/s57/pkg/s57"
)

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

func main() {
	// Parse only depth-related features
	fmt.Println("=== Parsing depth data only ===")
	chart, err := parseDepthDataOnly("US5MA22M.000")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Features loaded: %d\n", chart.FeatureCount())

	// Parse with validation
	fmt.Println("\n=== Parsing with validation ===")
	chart2, err := parseWithValidation("US5MA22M.000")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Features loaded: %d\n", chart2.FeatureCount())
}
