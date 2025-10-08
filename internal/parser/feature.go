package parser

import (
	"encoding/binary"
	"fmt"

	"github.com/beetlebugorg/iso8211/pkg/v1"
)

// GeometryType represents the type of geometry for a feature
type GeometryType int

const (
	// GeometryTypePoint represents a single point location
	GeometryTypePoint GeometryType = iota
	// GeometryTypeLineString represents a line composed of connected points
	GeometryTypeLineString
	// GeometryTypePolygon represents a closed polygon area
	GeometryTypePolygon
)

// String returns the string representation of the geometry type
func (g GeometryType) String() string {
	switch g {
	case GeometryTypePoint:
		return "Point"
	case GeometryTypeLineString:
		return "LineString"
	case GeometryTypePolygon:
		return "Polygon"
	default:
		return "Unknown"
	}
}

// Geometry represents the spatial representation of a feature
// S-57 §7.3: Spatial record structure
type Geometry struct {
	// Type is the geometry type (Point, LineString, or Polygon)
	Type GeometryType
	// Coordinates is an array of [longitude, latitude] pairs
	// Per GeoJSON convention: [lon, lat]
	Coordinates [][]float64
}

// datasetMetadata contains dataset identification and metadata from DSID record.
// This is private - access metadata through Chart methods.
//
// Reference: S-57 Part 3 §7.3.1.1 (31Main.pdf p3.34-3.35, table 7.4):
// Data Set Identification field structure with all subfields.
type datasetMetadata struct {
	rcnm int    // Record name (10 = dataset)
	rcid int64  // Record identification number
	expp int    // Exchange purpose (1=New, 2=Revision)
	intu int    // Intended usage
	dsnm string // Data set name - chart identifier (e.g., "GB5X01NE")
	edtn string // Edition number (e.g., "2")
	updn string // Update number (e.g., "0" for base cell)
	uadt string // Update application date (YYYYMMDD format)
	isdt string // Issue date (YYYYMMDD format)
	sted string // Edition number of S-57 (e.g., "03.1")
	prsp int    // Product specification (1=ENC, 2=ODD)
	psdn string // Product specification description
	pred string // Product specification edition number
	prof int    // Application profile (1=EN new, 2=ER revision, 3=DD)
	agen int    // Producing agency code
	comt string // Comment field
}

// DatasetName returns the dataset name (chart identifier).
// Example: "US5MA22M", "GB5X01NE"
func (m *datasetMetadata) DatasetName() string {
	return m.dsnm
}

// Edition returns the edition number as a string.
func (m *datasetMetadata) Edition() string {
	return m.edtn
}

// UpdateNumber returns the update number as a string.
func (m *datasetMetadata) UpdateNumber() string {
	return m.updn
}

// UpdateDate returns the update application date (YYYYMMDD format).
func (m *datasetMetadata) UpdateDate() string {
	return m.uadt
}

// IssueDate returns the issue date (YYYYMMDD format).
func (m *datasetMetadata) IssueDate() string {
	return m.isdt
}

// S57Edition returns the S-57 standard edition (e.g., "03.1").
func (m *datasetMetadata) S57Edition() string {
	return m.sted
}

// ProducingAgency returns the agency code.
func (m *datasetMetadata) ProducingAgency() int {
	return m.agen
}

// Comment returns the comment field.
func (m *datasetMetadata) Comment() string {
	return m.comt
}

// ExchangePurpose returns a human-readable exchange purpose string.
func (m *datasetMetadata) ExchangePurpose() string {
	switch m.expp {
	case 1:
		return "New"
	case 2:
		return "Revision"
	default:
		return "Unknown"
	}
}

// ProductSpecification returns a human-readable product specification string.
func (m *datasetMetadata) ProductSpecification() string {
	switch m.prsp {
	case 1:
		return "ENC"
	case 2:
		return "ODD"
	default:
		return "Unknown"
	}
}

// ApplicationProfile returns a human-readable application profile string.
func (m *datasetMetadata) ApplicationProfile() string {
	switch m.prof {
	case 1:
		return "EN (ENC New)"
	case 2:
		return "ER (ENC Revision)"
	case 3:
		return "DD (Data Dictionary)"
	default:
		return "Unknown"
	}
}

// Feature represents a navigational object extracted from S-57 chart data
// S-57 §2.1: Feature objects contain geometric and attribute information
type Feature struct {
	// ID is the unique feature identifier from the FRID record
	ID int64
	// ObjectClass is the S-57 object class code (e.g., "DEPCNT", "DEPARE", "BOYCAR")
	ObjectClass string
	// Geometry is the spatial representation of the feature
	Geometry Geometry
	// Attributes contains feature attributes as key-value pairs
	// Common attributes: DRVAL1 (depth), COLOUR (color), OBJNAM (name)
	Attributes map[string]interface{}
}

// Chart represents a complete S-57 Electronic Navigational Chart.
// This is the top-level structure returned by the parser.
//
// Reference: S-57 Part 3 §7 (31Main.pdf p3.31): Structure implementation
// showing how datasets are composed of metadata and feature records.
type Chart struct {
	metadata       *datasetMetadata            // Private - use accessor methods
	Features       []Feature                   // Public - array of extracted features
	spatialRecords map[spatialKey]*spatialRecord // Private - for update merging
}

// DatasetName returns the chart's dataset name (cell identifier).
func (c *Chart) DatasetName() string {
	if c.metadata == nil {
		return ""
	}
	return c.metadata.DatasetName()
}

// Edition returns the chart's edition number.
func (c *Chart) Edition() string {
	if c.metadata == nil {
		return ""
	}
	return c.metadata.Edition()
}

// UpdateNumber returns the chart's update number.
func (c *Chart) UpdateNumber() string {
	if c.metadata == nil {
		return ""
	}
	return c.metadata.UpdateNumber()
}

// UpdateDate returns the update application date (YYYYMMDD).
func (c *Chart) UpdateDate() string {
	if c.metadata == nil {
		return ""
	}
	return c.metadata.UpdateDate()
}

// IssueDate returns the issue date (YYYYMMDD).
func (c *Chart) IssueDate() string {
	if c.metadata == nil {
		return ""
	}
	return c.metadata.IssueDate()
}

// S57Edition returns the S-57 standard edition used (e.g., "03.1").
func (c *Chart) S57Edition() string {
	if c.metadata == nil {
		return ""
	}
	return c.metadata.S57Edition()
}

// ProducingAgency returns the producing agency code.
func (c *Chart) ProducingAgency() int {
	if c.metadata == nil {
		return 0
	}
	return c.metadata.ProducingAgency()
}

// Comment returns the metadata comment field.
func (c *Chart) Comment() string {
	if c.metadata == nil {
		return ""
	}
	return c.metadata.Comment()
}

// ExchangePurpose returns human-readable exchange purpose ("New" or "Revision").
func (c *Chart) ExchangePurpose() string {
	if c.metadata == nil {
		return "Unknown"
	}
	return c.metadata.ExchangePurpose()
}

// ProductSpecification returns human-readable product spec ("ENC" or "ODD").
func (c *Chart) ProductSpecification() string {
	if c.metadata == nil {
		return "Unknown"
	}
	return c.metadata.ProductSpecification()
}

// ApplicationProfile returns human-readable application profile.
func (c *Chart) ApplicationProfile() string {
	if c.metadata == nil {
		return "Unknown"
	}
	return c.metadata.ApplicationProfile()
}

// IntendedUsage returns the intended usage (navigational purpose) code.
//
// Values per S-57 specification:
//   1 = Overview, 2 = General, 3 = Coastal, 4 = Approach, 5 = Harbour, 6 = Berthing
func (c *Chart) IntendedUsage() int {
	if c.metadata == nil {
		return 0
	}
	return c.metadata.intu
}

// spatialRef represents a feature-to-spatial pointer with orientation
// S-57 §7.6.8: FSPT field contains RCID + ORNT + USAG + MASK
type spatialRef struct {
	RCID        int64 // Spatial record ID
	Orientation int   // 1=Forward, 2=Reverse, 255=Null
	Usage       int   // 1=Exterior, 2=Interior, 3=Exterior truncated
	Mask        int   // 1=Mask, 2=Show, 255=Null
}

// featureRecord represents a parsed S-57 feature record
// S-57 §7.6: Feature records contain feature identification and attributes
type featureRecord struct {
	ID            int64                  // Feature ID from FOID (for backward compatibility, use FIDN)
	AGEN          uint16                 // Producing agency from FOID
	FIDN          uint32                 // Feature identification number from FOID
	FIDS          uint16                 // Feature identification subdivision from FOID
	ObjectClass   int                    // S-57 object class code (DEPARE=42, etc.)
	GeomPrim      int                    // Geometric primitive from FRID (1=Point, 2=Line, 3=Area, 255=N/A)
	Group         int                    // Group code from FRID
	RecordVersion int                    // RVER - record version number
	UpdateInstr   int                    // RUIN - update instruction
	Attributes    map[string]interface{} // Feature attributes
	SpatialRefs   []spatialRef           // References to spatial records (from FSPT) with orientation
}

// parseFeatureRecord extracts feature data from an ISO 8211 record
// Returns nil if record is not a feature record
// S-57 §7.6.1: Feature records identified by FRID field
func parseFeatureRecord(record *iso8211.DataRecord) *featureRecord {
	// Check if this is a feature record (has FRID field)
	fridData, hasFRID := record.Fields["FRID"]
	if !hasFRID || len(fridData) < 12 {
		return nil // Need 12 bytes: RCNM(1) + RCID(4) + PRIM(1) + GRUP(1) + OBJL(2) + RVER(2) + RUIN(1)
	}

	// Parse FRID (Feature Record Identifier) per S-57 §7.6.1
	// Binary structure (12 bytes total):
	//   Byte 0: RCNM (1 byte) - Record name, value 100 = feature record
	//   Bytes 1-4: RCID (4 bytes) - Record identification number (uint32 LE)
	//   Byte 5: PRIM (1 byte) - Object geometric primitive (1=Point, 2=Line, 3=Area, 255=N/A)
	//   Byte 6: GRUP (1 byte) - Group code (1-254, 255=no group)
	//   Bytes 7-8: OBJL (2 bytes) - Object label/code (uint16 LE)
	//   Bytes 9-10: RVER (2 bytes) - Record version (uint16 LE)
	//   Byte 11: RUIN (1 byte) - Record update instruction

	rcnm := fridData[0]
	if rcnm != 100 {
		return nil // Not a feature record
	}

	featureRec := &featureRecord{
		Attributes:  make(map[string]interface{}),
		SpatialRefs: make([]spatialRef, 0),
	}

	// Extract RCID (not used currently, but available)
	// featureRec.RecordID = int64(binary.LittleEndian.Uint32(fridData[1:5]))

	// Extract PRIM (Object Geometric Primitive) from FRID byte [5]
	featureRec.GeomPrim = int(fridData[5])

	// Extract GRUP (Group) from FRID byte [6]
	featureRec.Group = int(fridData[6])

	// Extract OBJL (Object Label/Class) from FRID bytes [7:9]
	featureRec.ObjectClass = int(binary.LittleEndian.Uint16(fridData[7:9]))

	// Extract RVER (Record Version) from FRID bytes [9:11]
	featureRec.RecordVersion = int(binary.LittleEndian.Uint16(fridData[9:11]))

	// Extract RUIN (Record Update Instruction) from FRID byte [11]
	featureRec.UpdateInstr = int(fridData[11])

	// Parse FOID (Feature Object Identifier) for feature ID
	// S-57 §7.6.2: FOID structure is AGEN (2 bytes) + FIDN (4 bytes) + FIDS (2 bytes)
	if foidData, ok := record.Fields["FOID"]; ok && len(foidData) >= 8 {
		// AGEN (2 bytes at offset 0) - Producing agency code
		featureRec.AGEN = binary.LittleEndian.Uint16(foidData[0:2])

		// FIDN (4 bytes at offset 2) - Feature identification number
		featureRec.FIDN = binary.LittleEndian.Uint32(foidData[2:6])

		// FIDS (2 bytes at offset 6) - Feature identification subdivision
		featureRec.FIDS = binary.LittleEndian.Uint16(foidData[6:8])

		// Set ID for backward compatibility (use FIDN)
		featureRec.ID = int64(featureRec.FIDN)
	}

	// Parse ATTF (Feature Record Attribute) for attributes
	if attfData, ok := record.Fields["ATTF"]; ok {
		featureRec.Attributes = parseAttributes(attfData)
	}

	// Parse FSPT (Feature to Spatial Pointer) for spatial references
	if fsptData, ok := record.Fields["FSPT"]; ok {
		featureRec.SpatialRefs = parseSpatialPointers(fsptData)
	}

	return featureRec
}

// parseAttributes extracts attributes from ATTF field
// S-57 Appendix B.1: ATTF contains repeated attribute structures
func parseAttributes(data []byte) map[string]interface{} {
	attributes := make(map[string]interface{})

	// ATTF structure: repeated [ATTL(2 bytes), ATVL(variable)]
	// This is a simplified parser - real implementation needs subfield parsing
	offset := 0
	for offset+2 <= len(data) {
		// Extract attribute code (2 bytes)
		attrCode := binary.LittleEndian.Uint16(data[offset : offset+2])
		offset += 2

		// Find attribute value (terminated by unit separator 0x1F or end)
		valueEnd := offset
		for valueEnd < len(data) && data[valueEnd] != 0x1F {
			valueEnd++
		}

		if valueEnd > offset {
			// Store attribute as string for now
			// Real implementation should parse based on attribute type
			attrName := fmt.Sprintf("ATTR_%d", attrCode)
			attributes[attrName] = string(data[offset:valueEnd])
		}

		offset = valueEnd + 1 // Skip unit separator
	}

	return attributes
}

// parseSpatialPointers extracts spatial record references from FSPT field
// S-57 §7.6.8: FSPT contains pointers to VRID records - 8 bytes per pointer
func parseSpatialPointers(data []byte) []spatialRef {
	refs := make([]spatialRef, 0)

	// FSPT is repeating group per S-57 §7.6.8, each entry is 8 bytes:
	// NAME: B(40) - 5 bytes (RCNM=1, RCID=4)
	//   Offset 0: NAME_RCNM (1 byte) - Target record type
	//   Offset 1-4: NAME_RCID (4 bytes) - Target record ID (uint32 LE)
	// Offset 5: ORNT (1 byte) - Orientation (1=Forward, 2=Reverse, 255=Null)
	// Offset 6: USAG (1 byte) - Usage indicator (1=Exterior, 2=Interior, 3=Exterior truncated)
	// Offset 7: MASK (1 byte) - Masking indicator (1=Mask, 2=Show, 255=Null)

	// Binary mode: fixed 8-byte stride (not ASCII with separators)
	for i := 0; i+7 < len(data); i += 8 {
		// Extract NAME_RCID - this is the spatial record ID
		rcid := int64(binary.LittleEndian.Uint32(data[i+1 : i+5]))

		// Extract ORNT, USAG, MASK per S-57 §4.7.3.2
		orientation := int(data[i+5])
		usage := int(data[i+6])
		mask := int(data[i+7])

		refs = append(refs, spatialRef{
			RCID:        rcid,
			Orientation: orientation,
			Usage:       usage,
			Mask:        mask,
		})
	}

	return refs
}
