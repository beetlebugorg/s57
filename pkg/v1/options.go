package s57

// ParseOptions configures parsing behavior.
type ParseOptions struct {
	SkipUnknownFeatures bool
	ValidateGeometry    bool
	ObjectClassFilter   []string

	// ApplyUpdates controls whether to automatically discover and apply
	// update files (.001, .002, etc.) when parsing a base cell (.000).
	// Default is true - updates are automatically applied.
	//
	// When true, the parser looks for sequential update files in the same
	// directory as the base file and applies them in order.
	//
	// Set to false to parse only the base cell without updates.
	ApplyUpdates bool
}

// DefaultParseOptions returns default options.
func DefaultParseOptions() ParseOptions {
	return ParseOptions{
		SkipUnknownFeatures: false,
		ValidateGeometry:    true,
		ObjectClassFilter:   nil,
		ApplyUpdates:        true, // Auto-apply updates by default
	}
}
