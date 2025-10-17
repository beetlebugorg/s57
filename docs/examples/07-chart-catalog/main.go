package main

import (
	"fmt"
	"log"

	"github.com/beetlebugorg/s57/pkg/s57"
)

type ChartInfo struct {
	Path     string
	Name     string
	Bounds   s57.Bounds
	Features int
}

func buildCatalog(chartPaths []string) ([]ChartInfo, error) {
	parser := s57.NewParser()
	catalog := make([]ChartInfo, 0, len(chartPaths))

	for _, path := range chartPaths {
		chart, err := parser.Parse(path)
		if err != nil {
			log.Printf("Failed to parse %s: %v\n", path, err)
			continue
		}

		info := ChartInfo{
			Path:     path,
			Name:     chart.DatasetName(),
			Bounds:   chart.Bounds(),
			Features: chart.FeatureCount(),
		}
		catalog = append(catalog, info)
	}

	return catalog, nil
}

// Find charts covering a location
func findChartsForLocation(catalog []ChartInfo, lon, lat float64) []ChartInfo {
	var matches []ChartInfo
	for _, info := range catalog {
		if info.Bounds.Contains(lon, lat) {
			matches = append(matches, info)
		}
	}
	return matches
}

func main() {
	// Example with single chart
	chartPaths := []string{"US5MA22M.000"}

	catalog, err := buildCatalog(chartPaths)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Catalog contains %d charts\n\n", len(catalog))

	for _, info := range catalog {
		fmt.Printf("Chart: %s\n", info.Name)
		fmt.Printf("  Path: %s\n", info.Path)
		fmt.Printf("  Features: %d\n", info.Features)
		fmt.Printf("  Bounds: [%.4f,%.4f] to [%.4f,%.4f]\n",
			info.Bounds.MinLon, info.Bounds.MinLat,
			info.Bounds.MaxLon, info.Bounds.MaxLat)
	}

	// Example location query
	lon, lat := -71.05, 42.35
	matches := findChartsForLocation(catalog, lon, lat)
	fmt.Printf("\nCharts containing location %.4f, %.4f: %d\n", lon, lat, len(matches))
}
