package s57

// ParseOptions configures parsing behavior.
type ParseOptions struct {
	SkipUnknownFeatures bool
	ValidateGeometry    bool
	ObjectClassFilter   []string
}

// DefaultParseOptions returns default options.
func DefaultParseOptions() ParseOptions {
	return ParseOptions{
		SkipUnknownFeatures: false,
		ValidateGeometry:    true,
		ObjectClassFilter:   nil,
	}
}
