package main

import (
	"fmt"
	"log"

	"github.com/beetlebugorg/s57/pkg/s57"
)

func printFeatureDetails(feature s57.Feature) {
	fmt.Printf("Feature: %s (ID %d)\n", feature.ObjectClass(), feature.ID())

	attrs := feature.Attributes()

	// Object name (if present)
	if name, ok := attrs["OBJNAM"].(string); ok {
		fmt.Printf("  Name: %s\n", name)
	}

	// Depth value for depth contours
	if feature.ObjectClass() == "DEPCNT" {
		if depth, ok := attrs["VALDCO"].(float64); ok {
			fmt.Printf("  Depth: %.1f meters\n", depth)
		}
	}

	// Light characteristics
	if feature.ObjectClass() == "LIGHTS" {
		if color, ok := attrs["COLOUR"].([]int); ok {
			fmt.Printf("  Color codes: %v\n", color)
		}
		if height, ok := attrs["HEIGHT"].(float64); ok {
			fmt.Printf("  Height: %.1f meters\n", height)
		}
	}

	// Sounding depth
	if feature.ObjectClass() == "SOUNDG" {
		if depth, ok := attrs["VALSOU"].(float64); ok {
			fmt.Printf("  Sounding: %.1f meters\n", depth)
		}
	}
}

func main() {
	parser := s57.NewParser()
	chart, err := parser.Parse("US5MA22M.000")
	if err != nil {
		log.Fatal(err)
	}

	// Print details for first few features
	count := 0
	for _, f := range chart.Features() {
		printFeatureDetails(f)
		count++
		if count >= 5 {
			break
		}
	}
}
