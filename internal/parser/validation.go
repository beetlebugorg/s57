package parser

import (
	"fmt"
)

// ValidateCoordinate validates a single coordinate pair
// S-57 coordinates must be within valid geographic bounds
func ValidateCoordinate(lat, lon float64) error {
	if lat < -90.0 || lat > 90.0 {
		return &ErrInvalidCoordinate{Lat: lat, Lon: lon}
	}
	if lon < -180.0 || lon > 180.0 {
		return &ErrInvalidCoordinate{Lat: lat, Lon: lon}
	}
	return nil
}

// ValidateGeometry validates geometry per S-57 spatial rules
// S-57 §7.3: Geometry validation rules
func ValidateGeometry(geometry *Geometry) error {
	if geometry == nil {
		return &ErrInvalidGeometry{Reason: "geometry is nil"}
	}

	// Allow empty coordinates for meta-features (PRIM=255) like C_AGGR, M_COVR
	// These features have no spatial representation
	if len(geometry.Coordinates) == 0 {
		return nil // Empty geometry is valid for meta-features
	}

	// DEBUG: Log geometry details
	// fmt.Fprintf(os.Stderr, "Validating %s with %d coords\n", geometry.Type, len(geometry.Coordinates))

	// Validate coordinate count based on geometry type
	switch geometry.Type {
	case GeometryTypePoint:
		if len(geometry.Coordinates) != 1 {
			return &ErrInvalidGeometry{
				Type:   geometry.Type,
				Reason: fmt.Sprintf("point must have exactly 1 coordinate, got %d", len(geometry.Coordinates)),
			}
		}

	case GeometryTypeLineString:
		if len(geometry.Coordinates) < 2 {
			return &ErrInvalidGeometry{
				Type:   geometry.Type,
				Reason: fmt.Sprintf("linestring must have at least 2 coordinates, got %d", len(geometry.Coordinates)),
			}
		}

	case GeometryTypePolygon:
		if len(geometry.Coordinates) < 3 {
			return &ErrInvalidGeometry{
				Type:   geometry.Type,
				Reason: fmt.Sprintf("polygon must have at least 3 coordinates, got %d", len(geometry.Coordinates)),
			}
		}

		// Validate polygon closure (first coordinate must equal last)
		// Use epsilon tolerance for floating point comparison (S-57 §4.2.1)
		first := geometry.Coordinates[0]
		last := geometry.Coordinates[len(geometry.Coordinates)-1]
		if len(first) != 2 || len(last) != 2 {
			return &ErrInvalidGeometry{
				Type:   geometry.Type,
				Reason: "coordinate pairs must have exactly 2 values [lon, lat]",
			}
		}
		const epsilon = 1e-9 // Tolerance for floating point comparison
		dlat := first[1] - last[1]
		dlon := first[0] - last[0]
		if dlat*dlat+dlon*dlon > epsilon*epsilon {
			return &ErrInvalidGeometry{
				Type:   geometry.Type,
				Reason: fmt.Sprintf("polygon is not closed (distance between first and last: %.10f)", dlat*dlat+dlon*dlon),
			}
		}
	}

	// Validate each coordinate pair
	for i, coord := range geometry.Coordinates {
		if len(coord) != 2 {
			return &ErrInvalidGeometry{
				Type:   geometry.Type,
				Reason: fmt.Sprintf("coordinate %d must have exactly 2 values [lon, lat], got %d", i, len(coord)),
			}
		}
		lon, lat := coord[0], coord[1]
		if err := ValidateCoordinate(lat, lon); err != nil {
			return &ErrInvalidGeometry{
				Type:   geometry.Type,
				Reason: fmt.Sprintf("coordinate %d invalid: %v", i, err),
			}
		}
	}

	return nil
}

// ValidateFeature validates a feature per S-57 rules
func ValidateFeature(feature *Feature) error {
	if feature == nil {
		return fmt.Errorf("feature is nil")
	}

	if feature.ObjectClass == "" {
		return fmt.Errorf("feature has empty object class")
	}

	if err := ValidateGeometry(&feature.Geometry); err != nil {
		return fmt.Errorf("feature %d: %w", feature.ID, err)
	}

	return nil
}
