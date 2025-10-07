package parser

import (
	"encoding/binary"

	"github.com/beetlebugorg/iso8211/pkg/v1"
)

// datasetParams holds dataset-level parameters from DSPM record
// S-57 §7.3.2: Data Set Parameter Record
type datasetParams struct {
	COMF int32 // Coordinate multiplication factor (typically 10^7)
	SOMF int32 // Sounding (3D) multiplication factor (typically 10)
	HDAT int   // Horizontal geodetic datum
	VDAT int   // Vertical datum
	SDAT int   // Sounding datum
	CSCL int32 // Compilation scale
}

// defaultDatasetParams returns default parameters when DSPM is not found
func defaultDatasetParams() datasetParams {
	return datasetParams{
		COMF: 10000000, // 10^7 - standard for lat/lon in units of 0.00000001 degrees
		SOMF: 10,       // Standard for depth in decimeters
	}
}

// extractDatasetParams extracts DSPM record parameters
// S-57 §7.3.2.1: DSPM field structure
func extractDatasetParams(isoFile *iso8211.ISO8211File) datasetParams {
	params := defaultDatasetParams()

	// Look for DSPM record (Data Set Parameters)
	for _, record := range isoFile.Records {
		if dspmData, ok := record.Fields["DSPM"]; ok {
			params = parseDSPM(dspmData)
			break // Use first DSPM found
		}
	}

	return params
}

// parseDSPM parses the DSPM field per S-57 §7.3.2.1
// Binary format:
//
//	RCNM (1 byte) - Record name, value 20 = dataset parameters
//	RCID (4 bytes) - Record ID (uint32 LE)
//	HDAT (1 byte) - Horizontal datum
//	VDAT (1 byte) - Vertical datum
//	SDAT (1 byte) - Sounding datum
//	CSCL (4 bytes) - Compilation scale (uint32 LE)
//	DUNI (1 byte) - Depth units
//	HUNI (1 byte) - Height units
//	PUNI (1 byte) - Position units
//	COUN (1 byte) - Coordinate units
//	COMF (4 bytes) - Coordinate multiplication factor (int32 LE)
//	SOMF (4 bytes) - Sounding multiplication factor (int32 LE)
//	COMT (variable) - Comment
func parseDSPM(data []byte) datasetParams {
	params := defaultDatasetParams()

	// Minimum size check: RCNM(1) + RCID(4) + HDAT(1) + VDAT(1) + SDAT(1) + CSCL(4)
	//                     + DUNI(1) + HUNI(1) + PUNI(1) + COUN(1) + COMF(4) + SOMF(4) = 24 bytes
	if len(data) < 24 {
		return params
	}

	// Check RCNM (should be 20 for DSPM)
	rcnm := data[0]
	if rcnm != 20 {
		return params
	}

	// Extract fields at fixed offsets
	offset := 1 // Skip RCNM

	// RCID (4 bytes) - not used currently
	offset += 4

	// HDAT (1 byte)
	params.HDAT = int(data[offset])
	offset++

	// VDAT (1 byte)
	params.VDAT = int(data[offset])
	offset++

	// SDAT (1 byte)
	params.SDAT = int(data[offset])
	offset++

	// CSCL (4 bytes)
	params.CSCL = int32(binary.LittleEndian.Uint32(data[offset : offset+4]))
	offset += 4

	// Skip DUNI, HUNI, PUNI, COUN (4 bytes total)
	offset += 4

	// COMF (4 bytes) - int32 signed
	if offset+4 <= len(data) {
		params.COMF = int32(binary.LittleEndian.Uint32(data[offset : offset+4]))
		offset += 4
	}

	// SOMF (4 bytes) - int32 signed
	if offset+4 <= len(data) {
		params.SOMF = int32(binary.LittleEndian.Uint32(data[offset : offset+4]))
	}

	// Validation: If COMF is 0 or negative, use default
	if params.COMF <= 0 {
		params.COMF = 10000000
	}
	if params.SOMF <= 0 {
		params.SOMF = 10
	}

	return params
}

// convertCoordinate converts an integer coordinate to float64 using COMF
func convertCoordinate(value int32, comf int32) float64 {
	if comf <= 0 {
		comf = 10000000 // Default if invalid
	}
	return float64(value) / float64(comf)
}
