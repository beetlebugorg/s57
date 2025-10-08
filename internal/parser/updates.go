package parser

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/beetlebugorg/iso8211/pkg/v1"
)

// UpdateInstruction represents the RUIN (Record Update Instruction) field values
// S-57 Part 3 §8.4.2.2 and §8.4.3.2
type UpdateInstruction int

const (
	// UpdateInsert indicates a record should be inserted (RUIN = 1)
	UpdateInsert UpdateInstruction = 1

	// UpdateDelete indicates a record should be deleted (RUIN = 2)
	UpdateDelete UpdateInstruction = 2

	// UpdateModify indicates a record should be modified (RUIN = 3)
	UpdateModify UpdateInstruction = 3
)

// findUpdateFiles discovers sequential update files for a base cell
//
// Given "GB5X01SW.000", looks for "GB5X01SW.001", "GB5X01SW.002", etc.
// in the same directory. Returns paths in order.
func findUpdateFiles(baseFilename string) ([]string, error) {
	// Get base filename without extension
	dir := filepath.Dir(baseFilename)
	base := filepath.Base(baseFilename)

	// Remove extension (.000)
	baseName := strings.TrimSuffix(base, filepath.Ext(base))

	var updates []string

	// Look for sequential updates: .001, .002, .003, etc.
	for updateNum := 1; updateNum <= 999; updateNum++ {
		updateFile := filepath.Join(dir, fmt.Sprintf("%s.%03d", baseName, updateNum))

		// Check if file exists
		if _, err := os.Stat(updateFile); err == nil {
			updates = append(updates, updateFile)
		} else if os.IsNotExist(err) {
			// Stop at first missing update (updates must be sequential)
			break
		} else {
			return nil, fmt.Errorf("error checking for update file %s: %w", updateFile, err)
		}
	}

	return updates, nil
}

// applyUpdates applies update files to parsed chart data
//
// Updates are applied at the record level before geometry construction.
// This modifies featureRecords and spatialRecords in place.
func applyUpdates(baseChart *chartData, updateFiles []string, params datasetParams) error {
	for _, updateFile := range updateFiles {
		if err := applyUpdate(baseChart, updateFile, params); err != nil {
			return fmt.Errorf("failed to apply update %s: %w", updateFile, err)
		}
	}
	return nil
}

// chartData holds the intermediate chart state during update merging
type chartData struct {
	features       []*featureRecord
	spatialRecords map[spatialKey]*spatialRecord
	metadata       *datasetMetadata

	// Index for fast lookup during updates
	featuresByID map[int64]*featureRecord
}

// applyUpdate applies a single update file to the chart data
func applyUpdate(chart *chartData, updateFile string, params datasetParams) error {
	// Parse update file
	reader, err := iso8211.NewReader(updateFile)
	if err != nil {
		return fmt.Errorf("failed to open update file: %w", err)
	}
	defer reader.Close()

	isoFile, err := reader.Parse()
	if err != nil {
		return fmt.Errorf("failed to parse update file: %w", err)
	}

	// Process each record in update file
	for _, record := range isoFile.Records {
		// Feature record (FRID)
		if fridData, ok := record.Fields["FRID"]; ok && len(fridData) >= 12 {
			if err := applyFeatureUpdate(chart, record, fridData); err != nil {
				return err
			}
			continue
		}

		// Spatial record (VRID)
		if vridData, ok := record.Fields["VRID"]; ok && len(vridData) >= 8 {
			if err := applySpatialUpdate(chart, record, vridData, params); err != nil {
				return err
			}
			continue
		}
	}

	return nil
}

// applyFeatureUpdate handles INSERT/DELETE/MODIFY for features
func applyFeatureUpdate(chart *chartData, record *iso8211.DataRecord, fridData []byte) error {
	ruin := UpdateInstruction(fridData[11])

	// Parse feature record
	featureRec := parseFeatureRecord(record)
	if featureRec == nil {
		return fmt.Errorf("failed to parse feature record")
	}

	switch ruin {
	case UpdateInsert:
		// Add new feature
		if _, exists := chart.featuresByID[featureRec.ID]; exists {
			return fmt.Errorf("INSERT: feature %d already exists", featureRec.ID)
		}
		chart.features = append(chart.features, featureRec)
		chart.featuresByID[featureRec.ID] = featureRec

	case UpdateDelete:
		// Remove existing feature
		existing, exists := chart.featuresByID[featureRec.ID]
		if !exists {
			return fmt.Errorf("DELETE: feature %d not found", featureRec.ID)
		}

		// Remove from index
		delete(chart.featuresByID, featureRec.ID)

		// Remove from slice
		for i, f := range chart.features {
			if f == existing {
				chart.features = append(chart.features[:i], chart.features[i+1:]...)
				break
			}
		}

	case UpdateModify:
		// Update existing feature
		existing, exists := chart.featuresByID[featureRec.ID]
		if !exists {
			return fmt.Errorf("MODIFY: feature %d not found", featureRec.ID)
		}

		// Replace feature data
		*existing = *featureRec
		// Keep reference in index
		chart.featuresByID[featureRec.ID] = existing

	default:
		return fmt.Errorf("unknown RUIN value for feature: %d", ruin)
	}

	return nil
}

// applySpatialUpdate handles INSERT/DELETE/MODIFY for spatial records
func applySpatialUpdate(chart *chartData, record *iso8211.DataRecord, vridData []byte, params datasetParams) error {
	ruin := UpdateInstruction(vridData[7])

	// Parse spatial record
	spatialRec := parseSpatialRecordWithParams(record, params)
	if spatialRec == nil {
		return fmt.Errorf("failed to parse spatial record")
	}

	// Build key from record type and ID
	key := spatialKey{
		RCNM: int(spatialRec.RecordType),
		RCID: spatialRec.ID,
	}

	switch ruin {
	case UpdateInsert:
		// Add new spatial record
		if _, exists := chart.spatialRecords[key]; exists {
			return fmt.Errorf("INSERT: spatial record %v already exists", key)
		}
		chart.spatialRecords[key] = spatialRec

	case UpdateDelete:
		// Remove existing spatial record
		if _, exists := chart.spatialRecords[key]; !exists {
			return fmt.Errorf("DELETE: spatial record %v not found", key)
		}
		delete(chart.spatialRecords, key)

	case UpdateModify:
		// Update existing spatial record
		if _, exists := chart.spatialRecords[key]; !exists {
			return fmt.Errorf("MODIFY: spatial record %v not found", key)
		}
		chart.spatialRecords[key] = spatialRec

	default:
		return fmt.Errorf("unknown RUIN value for spatial: %d", ruin)
	}

	return nil
}
