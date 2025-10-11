package s57_test

import (
	"testing"

	s57 "github.com/beetlebugorg/s57/pkg/v1"
)

func TestSOUNDGWith3DCoordinates(t *testing.T) {
	// Parse the test chart that contains SOUNDG features (2 features)
	parser := s57.NewParser()
	chart, err := parser.Parse("/Users/jcollins/Desktop/S52/S-64_ENC_Unencrypted_TDS/2.1.1 Power Up/ENC_ROOT/GB5X01SW.000")
	if err != nil {
		t.Fatalf("Failed to parse chart: %v", err)
	}

	// Find all SOUNDG features
	features := chart.Features()
	var soundings []s57.Feature
	for _, f := range features {
		if f.ObjectClass() == "SOUNDG" {
			soundings = append(soundings, f)
		}
	}

	if len(soundings) == 0 {
		t.Fatal("No SOUNDG features found in chart")
	}

	t.Logf("Found %d SOUNDG feature(s)", len(soundings))

	// Check ALL SOUNDG features
	for idx, soundg := range soundings {
		t.Logf("\n=== SOUNDG feature %d (FIDN=%d) ===", idx, soundg.ID())
		geom := soundg.Geometry()

		t.Logf("SOUNDG geometry type: %v", geom.Type)
		t.Logf("SOUNDG has %d coordinate(s)", len(geom.Coordinates))

	if len(geom.Coordinates) == 0 {
		t.Fatal("SOUNDG has no coordinates")
	}

	// Check if we captured 3D coordinates (lon, lat, depth)
	for i, coord := range geom.Coordinates {
		t.Logf("Coordinate %d: %v (length=%d)", i, coord, len(coord))

		if len(coord) < 2 {
			t.Errorf("Coordinate %d has less than 2 values: %v", i, coord)
			continue
		}

		lon := coord[0]
		lat := coord[1]
		t.Logf("  Lon: %f, Lat: %f", lon, lat)

		// Check if we have a Z coordinate (depth)
		if len(coord) >= 3 {
			depth := coord[2]
			t.Logf("  Depth: %f meters", depth)

			// SOUNDG depths should typically be positive (below sea level)
			if depth <= 0 {
				t.Logf("  Warning: Depth is zero or negative: %f", depth)
			}
		} else {
			t.Errorf("Coordinate %d missing Z value (depth)", i)
		}
	}

		// Check attributes
		attrs := soundg.Attributes()
		t.Logf("SOUNDG attributes: %+v", attrs)
	}
}
