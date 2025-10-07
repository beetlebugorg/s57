package s57

import (
	"testing"
)

func TestPublicAPI(t *testing.T) {
	// Test that the public API works
	parser := NewParser()
	if parser == nil {
		t.Fatal("NewParser returned nil")
	}

	// Test default options
	opts := DefaultParseOptions()
	if opts.ValidateGeometry != true {
		t.Error("Default ValidateGeometry should be true")
	}
	if opts.SkipUnknownFeatures != false {
		t.Error("Default SkipUnknownFeatures should be false")
	}
}

func TestChartAccessors(t *testing.T) {
	// Create a mock chart to test accessors
	chart := &Chart{
		features:        []Feature{{id: 1, objectClass: "DEPCNT"}, {id: 2, objectClass: "LIGHTS"}},
		datasetName:     "TEST123",
		edition:         "5",
		updateNumber:    "2",
		producingAgency: 550,
	}

	if chart.DatasetName() != "TEST123" {
		t.Errorf("Expected DatasetName=TEST123, got %s", chart.DatasetName())
	}
	if chart.Edition() != "5" {
		t.Errorf("Expected Edition=5, got %s", chart.Edition())
	}
	if chart.UpdateNumber() != "2" {
		t.Errorf("Expected UpdateNumber=2, got %s", chart.UpdateNumber())
	}
	if chart.ProducingAgency() != 550 {
		t.Errorf("Expected ProducingAgency=550, got %d", chart.ProducingAgency())
	}
	if chart.FeatureCount() != 2 {
		t.Errorf("Expected FeatureCount=2, got %d", chart.FeatureCount())
	}
	if len(chart.Features()) != 2 {
		t.Errorf("Expected len(Features())=2, got %d", len(chart.Features()))
	}

	// Test feature accessors
	features := chart.Features()
	if features[0].ID() != 1 {
		t.Errorf("Expected feature ID=1, got %d", features[0].ID())
	}
	if features[0].ObjectClass() != "DEPCNT" {
		t.Errorf("Expected ObjectClass=DEPCNT, got %s", features[0].ObjectClass())
	}
}

func TestFeatureAccessors(t *testing.T) {
	// Test feature attribute access
	attrs := map[string]interface{}{
		"DRVAL1": 10.5,
		"OBJNAM": "Test Object",
	}

	feature := Feature{
		id:          123,
		objectClass: "LIGHTS",
		attributes:  attrs,
	}

	if feature.ID() != 123 {
		t.Errorf("Expected ID=123, got %d", feature.ID())
	}
	if feature.ObjectClass() != "LIGHTS" {
		t.Errorf("Expected ObjectClass=LIGHTS, got %s", feature.ObjectClass())
	}

	// Test Attribute() method
	if val, ok := feature.Attribute("DRVAL1"); !ok {
		t.Error("Expected DRVAL1 attribute to exist")
	} else if val != 10.5 {
		t.Errorf("Expected DRVAL1=10.5, got %v", val)
	}

	// Test non-existent attribute
	if _, ok := feature.Attribute("NONEXIST"); ok {
		t.Error("Expected NONEXIST attribute to not exist")
	}

	// Test Attributes() method returns all
	allAttrs := feature.Attributes()
	if len(allAttrs) != 2 {
		t.Errorf("Expected 2 attributes, got %d", len(allAttrs))
	}
}

func TestFeaturesInBounds(t *testing.T) {
	// Create features at different locations
	chart := &Chart{
		features: []Feature{
			{id: 1, geometry: Geometry{Type: GeometryTypePoint, Coordinates: [][]float64{{-71.0, 42.0}}}},
			{id: 2, geometry: Geometry{Type: GeometryTypePoint, Coordinates: [][]float64{{-71.5, 42.5}}}},
			{id: 3, geometry: Geometry{Type: GeometryTypePoint, Coordinates: [][]float64{{-72.0, 43.0}}}},
			{id: 4, geometry: Geometry{Type: GeometryTypeLineString, Coordinates: [][]float64{{-71.1, 42.1}, {-71.2, 42.2}}}},
		},
	}
	chart.buildSpatialIndex()

	// Query a viewport
	viewport := Bounds{MinLon: -71.3, MaxLon: -70.8, MinLat: 41.9, MaxLat: 42.3}
	visible := chart.FeaturesInBounds(viewport)

	// Should find features 1 and 4 (in viewport), but not 2 or 3
	if len(visible) != 2 {
		t.Errorf("Expected 2 visible features, got %d", len(visible))
	}

	// Verify the correct features were returned
	ids := make(map[int64]bool)
	for _, f := range visible {
		ids[f.ID()] = true
	}
	if !ids[1] || !ids[4] {
		t.Error("Expected features 1 and 4 to be visible")
	}
}

func TestChartBounds(t *testing.T) {
	chart := &Chart{
		features: []Feature{
			{id: 1, geometry: Geometry{Coordinates: [][]float64{{-71.0, 42.0}}}},
			{id: 2, geometry: Geometry{Coordinates: [][]float64{{-71.5, 42.5}}}},
			{id: 3, geometry: Geometry{Coordinates: [][]float64{{-70.8, 41.9}}}},
		},
	}
	chart.buildSpatialIndex()

	bounds := chart.Bounds()

	// Check that bounds encompass all features
	if bounds.MinLon != -71.5 || bounds.MaxLon != -70.8 {
		t.Errorf("Unexpected longitude bounds: %f to %f", bounds.MinLon, bounds.MaxLon)
	}
	if bounds.MinLat != 41.9 || bounds.MaxLat != 42.5 {
		t.Errorf("Unexpected latitude bounds: %f to %f", bounds.MinLat, bounds.MaxLat)
	}
}

func TestBoundsIntersects(t *testing.T) {
	b1 := Bounds{MinLon: -71.0, MaxLon: -70.0, MinLat: 42.0, MaxLat: 43.0}
	b2 := Bounds{MinLon: -70.5, MaxLon: -69.5, MinLat: 42.5, MaxLat: 43.5}
	b3 := Bounds{MinLon: -69.0, MaxLon: -68.0, MinLat: 44.0, MaxLat: 45.0}

	if !b1.Intersects(b2) {
		t.Error("b1 and b2 should intersect")
	}
	if b1.Intersects(b3) {
		t.Error("b1 and b3 should not intersect")
	}
}

func TestUsageBand(t *testing.T) {
	tests := []struct {
		band     UsageBand
		name     string
		minScale int
		maxScale int
	}{
		{UsageBandOverview, "Overview", 1500000, 0},
		{UsageBandGeneral, "General", 350000, 1500000},
		{UsageBandCoastal, "Coastal", 90000, 350000},
		{UsageBandApproach, "Approach", 22000, 90000},
		{UsageBandHarbour, "Harbour", 4000, 22000},
		{UsageBandBerthing, "Berthing", 0, 4000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.band.String() != tt.name {
				t.Errorf("Expected name %s, got %s", tt.name, tt.band.String())
			}
			min, max := tt.band.ScaleRange()
			if min != tt.minScale || max != tt.maxScale {
				t.Errorf("Expected scale range (%d, %d), got (%d, %d)",
					tt.minScale, tt.maxScale, min, max)
			}
		})
	}
}

func TestGeometryTypeString(t *testing.T) {
	tests := []struct {
		gtype    GeometryType
		expected string
	}{
		{GeometryTypePoint, "Point"},
		{GeometryTypeLineString, "LineString"},
		{GeometryTypePolygon, "Polygon"},
	}

	for _, tt := range tests {
		if tt.gtype.String() != tt.expected {
			t.Errorf("GeometryType %d: expected %s, got %s", tt.gtype, tt.expected, tt.gtype.String())
		}
	}
}
