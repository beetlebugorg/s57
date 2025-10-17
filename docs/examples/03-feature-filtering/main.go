package main

import (
	"fmt"
	"log"

	"github.com/beetlebugorg/s57/pkg/s57"
)

// Get all depth contours
func getDepthContours(chart *s57.Chart) []s57.Feature {
	var contours []s57.Feature
	for _, f := range chart.Features() {
		if f.ObjectClass() == "DEPCNT" {
			contours = append(contours, f)
		}
	}
	return contours
}

// Get all navigation aids (buoys, lights, beacons)
func getNavAids(chart *s57.Chart) []s57.Feature {
	navAidClasses := map[string]bool{
		"BOYCAR": true, "BOYINB": true, "BOYISD": true,
		"BOYLAT": true, "BOYSAW": true, "BOYSPP": true,
		"BCNCAR": true, "BCNISD": true, "BCNLAT": true,
		"LIGHTS": true,
	}

	var navAids []s57.Feature
	for _, f := range chart.Features() {
		if navAidClasses[f.ObjectClass()] {
			navAids = append(navAids, f)
		}
	}
	return navAids
}

func main() {
	parser := s57.NewParser()
	chart, err := parser.Parse("US5MA22M.000")
	if err != nil {
		log.Fatal(err)
	}

	contours := getDepthContours(chart)
	fmt.Printf("Depth contours: %d\n", len(contours))

	navAids := getNavAids(chart)
	fmt.Printf("Navigation aids: %d\n", len(navAids))
}
