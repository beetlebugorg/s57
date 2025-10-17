package main

import (
	"fmt"
	"log"

	"github.com/beetlebugorg/s57/pkg/s57"
)

// Default: automatic update application
func parseWithUpdates(basePath string) {
	parser := s57.NewParser()

	// Automatically finds and applies .001, .002, .003, etc.
	chart, err := parser.Parse("GB5X01SW.000")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Dataset: %s\n", chart.DatasetName())
	fmt.Printf("Edition: %s\n", chart.Edition())
	fmt.Printf("Update: %s\n", chart.UpdateNumber())
	fmt.Printf("Features: %d\n", chart.FeatureCount())
}

// Parse base cell only (no updates)
func parseBaseOnly(basePath string) {
	parser := s57.NewParser()

	opts := s57.ParseOptions{
		ApplyUpdates: false,
	}

	chart, err := parser.ParseWithOptions("GB5X01SW.000", opts)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Update: %s (base cell only)\n", chart.UpdateNumber())
}

func main() {
	fmt.Println("=== With automatic updates ===")
	parseWithUpdates("")

	fmt.Println("\n=== Base cell only ===")
	parseBaseOnly("")
}
