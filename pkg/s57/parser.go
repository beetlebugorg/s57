package s57

import (
	"github.com/beetlebugorg/s57/internal/parser"
)

// Parser parses S-57 Electronic Navigational Chart files.
//
// Create a parser with NewParser and use Parse or ParseWithOptions to read charts.
type Parser interface {
	// Parse reads an S-57 file and returns the parsed chart.
	//
	// The filename should point to an S-57 base cell (.000) or update file (.001, .002, etc.).
	// Returns an error if the file cannot be read or parsed according to S-57 Edition 3.1.
	Parse(filename string) (*Chart, error)

	// ParseWithOptions parses an S-57 file with custom options.
	//
	// Use ParseOptions to control validation, error handling, and feature filtering.
	ParseWithOptions(filename string, opts ParseOptions) (*Chart, error)
}

// NewParser creates a new S-57 parser with default settings.
//
// Example:
//
//	parser := s57.NewParser()
//	chart, err := parser.Parse("US5MA22M.000")
func NewParser() Parser {
	return &parserWrapper{
		internal: parser.NewParser(),
	}
}

// parserWrapper wraps the internal parser and converts types
type parserWrapper struct {
	internal parser.Parser
}

func (p *parserWrapper) Parse(filename string) (*Chart, error) {
	internalChart, err := p.internal.Parse(filename)
	if err != nil {
		return nil, err
	}
	return convertChart(internalChart), nil
}

func (p *parserWrapper) ParseWithOptions(filename string, opts ParseOptions) (*Chart, error) {
	internalOpts := parser.ParseOptions{
		SkipUnknownFeatures: opts.SkipUnknownFeatures,
		ValidateGeometry:    opts.ValidateGeometry,
		ObjectClassFilter:   opts.ObjectClassFilter,
	}
	internalChart, err := p.internal.ParseWithOptions(filename, internalOpts)
	if err != nil {
		return nil, err
	}
	return convertChart(internalChart), nil
}
