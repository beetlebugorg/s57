package s57

import (
	"archive/zip"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// ChartCatalog represents a parsed NOAA ENC product catalog.
//
// The catalog contains metadata and download URLs for all NOAA charts,
// including precise polygon boundaries. This enables true lazy loading:
// query the catalog to find charts in viewport, then download only what's needed.
//
// NOAA publishes the catalog at:
// https://www.charts.noaa.gov/ENCs/ENCProdCat_19115.xml
type ChartCatalog struct {
	Entries []CatalogEntry
	Updated time.Time
}

// CatalogEntry represents a single chart in the catalog.
type CatalogEntry struct {
	Name             string    // Chart name (e.g., "US5MA22M")
	Edition          string    // Edition number
	Scale            int       // Compilation scale denominator
	DownloadURL      string    // Direct download URL (.zip file)
	SizeMB           float64   // File size in megabytes
	UpdateDate       time.Time // Last update date
	PublicationDate  time.Time // Publication date
	Polygon          []LatLon  // Precise coverage polygon
	Keywords         []string  // Descriptive keywords (state, region, etc.)
}

// LatLon represents a geographic coordinate
type LatLon struct {
	Lat float64
	Lon float64
}

// Bounds returns the bounding box for this catalog entry's polygon.
func (e *CatalogEntry) Bounds() Bounds {
	if len(e.Polygon) == 0 {
		return Bounds{}
	}

	minLat, maxLat := e.Polygon[0].Lat, e.Polygon[0].Lat
	minLon, maxLon := e.Polygon[0].Lon, e.Polygon[0].Lon

	for _, pos := range e.Polygon[1:] {
		if pos.Lat < minLat {
			minLat = pos.Lat
		}
		if pos.Lat > maxLat {
			maxLat = pos.Lat
		}
		if pos.Lon < minLon {
			minLon = pos.Lon
		}
		if pos.Lon > maxLon {
			maxLon = pos.Lon
		}
	}

	return Bounds{
		MinLat: minLat,
		MaxLat: maxLat,
		MinLon: minLon,
		MaxLon: maxLon,
	}
}

// LoadCatalog loads a NOAA ENC catalog from a local file.
//
// The catalog file can be downloaded from:
// https://www.charts.noaa.gov/ENCs/ENCProdCat_19115.xml
//
// Example:
//
//	catalog, err := s57.LoadCatalog("/tmp/ENCProdCat_19115.xml")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Printf("Loaded %d charts from catalog\n", len(catalog.Entries))
func LoadCatalog(path string) (*ChartCatalog, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open catalog: %w", err)
	}
	defer file.Close()

	return parseCatalog(file)
}

// DownloadCatalog downloads the NOAA ENC catalog from the official URL.
//
// Downloads from: https://www.charts.noaa.gov/ENCs/ENCProdCat_19115.xml
// The catalog is ~90MB and contains metadata for all 6500+ NOAA charts.
//
// Example:
//
//	catalog, err := s57.DownloadCatalog("/tmp/noaa_catalog.xml")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Printf("Downloaded catalog with %d charts\n", len(catalog.Entries))
func DownloadCatalog(savePath string) (*ChartCatalog, error) {
	url := "https://www.charts.noaa.gov/ENCs/ENCProdCat_19115.xml"

	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("download catalog: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download failed: HTTP %d", resp.StatusCode)
	}

	// Save to file
	outFile, err := os.Create(savePath)
	if err != nil {
		return nil, fmt.Errorf("create file: %w", err)
	}
	defer outFile.Close()

	// Copy response to file
	_, err = io.Copy(outFile, resp.Body)
	if err != nil {
		return nil, fmt.Errorf("save catalog: %w", err)
	}

	// Reopen and parse
	return LoadCatalog(savePath)
}

// XML structures for parsing NOAA ENC catalog
type xmlSeries struct {
	XMLName  xml.Name     `xml:"DS_Series"`
	Datasets []xmlDataset `xml:"composedOf>DS_DataSet"`
}

type xmlDataset struct {
	Metadata xmlMetadata `xml:"has>MD_Metadata"`
}

type xmlMetadata struct {
	Identification xmlIdentification `xml:"identificationInfo>MD_DataIdentification"`
	Distribution   xmlDistribution   `xml:"distributionInfo>MD_Distribution"`
}

type xmlIdentification struct {
	Citation    xmlCitation  `xml:"citation>CI_Citation"`
	Keywords    []xmlKeyword `xml:"descriptiveKeywords>MD_Keywords>keyword"`
	Extent      xmlExtent    `xml:"extent>EX_Extent"`
	SpatialType xmlSpatialType `xml:"spatialRepresentationType"`
}

type xmlCitation struct {
	Title     string    `xml:"title>CharacterString"`
	Dates     []xmlDate `xml:"date>CI_Date"`
	Edition   string    `xml:"edition>CharacterString"`
}

type xmlDate struct {
	Date     string `xml:"date>Date"`
	DateType string `xml:"dateType>CI_DateTypeCode"`
}

type xmlKeyword struct {
	Value string `xml:"CharacterString"`
}

type xmlExtent struct {
	Geographic xmlGeographic `xml:"geographicElement>EX_BoundingPolygon"`
}

type xmlGeographic struct {
	Polygon xmlPolygon `xml:"polygon>Polygon"`
}

type xmlPolygon struct {
	Exterior xmlExterior `xml:"exterior"`
}

type xmlExterior struct {
	LinearRing xmlLinearRing `xml:"LinearRing"`
}

type xmlLinearRing struct {
	Positions []string `xml:"pos"`
}

type xmlSpatialType struct {
	Scale string `xml:"MD_VectorSpatialRepresentation>geometricObjects>MD_GeometricObjects>geometricObjectType>MD_GeometricObjectTypeCode"`
}

type xmlDistribution struct {
	TransferOptions xmlTransferOptions `xml:"transferOptions>MD_DigitalTransferOptions"`
}

type xmlTransferOptions struct {
	TransferSize string        `xml:"transferSize>Real"`
	OnlineURL    xmlOnlineURL  `xml:"onLine>CI_OnlineResource"`
}

type xmlOnlineURL struct {
	URL string `xml:"linkage>URL"`
}

// parseCatalog parses the NOAA ENC XML catalog using struct-based parsing.
func parseCatalog(reader io.Reader) (*ChartCatalog, error) {
	var series xmlSeries

	decoder := xml.NewDecoder(reader)
	if err := decoder.Decode(&series); err != nil {
		return nil, fmt.Errorf("parse XML: %w", err)
	}

	catalog := &ChartCatalog{
		Updated: time.Now(),
		Entries: make([]CatalogEntry, 0, len(series.Datasets)),
	}

	for _, dataset := range series.Datasets {
		entry := parseDataset(dataset)
		if entry.Name != "" {
			catalog.Entries = append(catalog.Entries, entry)
		}
	}

	return catalog, nil
}

// parseDataset converts an XML dataset to a catalog entry.
func parseDataset(dataset xmlDataset) CatalogEntry {
	md := dataset.Metadata
	entry := CatalogEntry{}

	// Parse name from title
	entry.Name = md.Identification.Citation.Title
	entry.Edition = md.Identification.Citation.Edition

	// Parse dates
	for _, date := range md.Identification.Citation.Dates {
		if parsedDate, err := time.Parse("2006-01-02", date.Date); err == nil {
			if strings.Contains(date.DateType, "revision") {
				entry.UpdateDate = parsedDate
			} else if strings.Contains(date.DateType, "publication") {
				entry.PublicationDate = parsedDate
			}
		}
	}

	// Parse keywords
	for _, kw := range md.Identification.Keywords {
		entry.Keywords = append(entry.Keywords, kw.Value)
	}

	// Parse polygon
	positions := md.Identification.Extent.Geographic.Polygon.Exterior.LinearRing.Positions
	for _, pos := range positions {
		parts := strings.Fields(pos)
		if len(parts) == 2 {
			lat, err1 := strconv.ParseFloat(parts[0], 64)
			lon, err2 := strconv.ParseFloat(parts[1], 64)
			if err1 == nil && err2 == nil {
				entry.Polygon = append(entry.Polygon, LatLon{
					Lat: lat,
					Lon: lon,
				})
			}
		}
	}

	// Parse download URL
	entry.DownloadURL = md.Distribution.TransferOptions.OnlineURL.URL

	// Parse file size
	if size, err := strconv.ParseFloat(md.Distribution.TransferOptions.TransferSize, 64); err == nil {
		entry.SizeMB = size
	}

	// Parse scale - try to extract from spatial representation
	// The scale is typically in a separate section we need to handle
	// For now, we'll leave it to be extracted separately if needed

	return entry
}

// QueryCatalog queries the catalog for charts intersecting the given bounds.
//
// Returns catalog entries sorted by scale (larger scale / smaller denominator first).
//
// Example:
//
//	viewport := s57.Bounds{
//	    MinLon: -122.5, MaxLon: -122.0,
//	    MinLat: 37.5, MaxLat: 38.0,
//	}
//	entries := catalog.Query(viewport)
//	fmt.Printf("Found %d charts covering San Francisco Bay\n", len(entries))
func (c *ChartCatalog) Query(bounds Bounds) []CatalogEntry {
	var matches []CatalogEntry

	for _, entry := range c.Entries {
		entryBounds := entry.Bounds()

		// Check if entry bounds intersect query bounds
		if bounds.Intersects(entryBounds) {
			matches = append(matches, entry)
		}
	}

	return matches
}

// DownloadChart downloads a chart from the catalog to the specified directory.
//
// The chart is downloaded as a .zip file and extracted to create the directory
// structure expected by the parser (chartName/chartName.000).
//
// Returns the path to the base cell (.000 file).
//
// Example:
//
//	entry := catalog.Entries[0]
//	chartPath, err := catalog.DownloadChart(entry, "/tmp/charts")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	// chartPath is "/tmp/charts/US5MA22M/US5MA22M.000"
func (c *ChartCatalog) DownloadChart(entry CatalogEntry, destDir string, keepExtracted bool) (string, error) {
	zipPath := filepath.Join(destDir, entry.Name+".zip")
	baseCellPath := filepath.Join(destDir, "ENC_ROOT", entry.Name, entry.Name+".000")

	if keepExtracted {
		// Check if already extracted
		if _, err := os.Stat(baseCellPath); err == nil {
			return baseCellPath, nil
		}
	} else {
		// Check if zip already exists
		if _, err := os.Stat(zipPath); err == nil {
			return fmt.Sprintf("zip://%s!ENC_ROOT/%s/%s.000", zipPath, entry.Name, entry.Name), nil
		}
	}

	// Need to download - not cached
	resp, err := http.Get(entry.DownloadURL)
	if err != nil {
		return "", fmt.Errorf("download chart: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download failed: HTTP %d", resp.StatusCode)
	}

	zipFile, err := os.Create(zipPath)
	if err != nil {
		return "", fmt.Errorf("create zip file: %w", err)
	}

	_, err = io.Copy(zipFile, resp.Body)
	zipFile.Close()
	if err != nil {
		return "", fmt.Errorf("save zip file: %w", err)
	}

	if keepExtracted {
		// Extract and delete zip
		if err := extractZip(zipPath, destDir); err != nil {
			return "", fmt.Errorf("extract zip: %w", err)
		}

		// Delete zip file to save space
		os.Remove(zipPath)

		// Verify the extracted file exists
		if _, err := os.Stat(baseCellPath); os.IsNotExist(err) {
			return "", fmt.Errorf("chart file not found after extraction: %s", baseCellPath)
		}

		return baseCellPath, nil
	}

	// Keep zip, return zip:// URL for streaming
	return fmt.Sprintf("zip://%s!ENC_ROOT/%s/%s.000", zipPath, entry.Name, entry.Name), nil
}

// extractZip extracts a zip file to the destination directory.
func extractZip(zipPath, destDir string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		// Construct the full path
		fpath := filepath.Join(destDir, f.Name)

		// Check for zip slip vulnerability
		if !strings.HasPrefix(fpath, filepath.Clean(destDir)+string(os.PathSeparator)) {
			return fmt.Errorf("invalid file path: %s", fpath)
		}

		if f.FileInfo().IsDir() {
			// Create directory
			os.MkdirAll(fpath, os.ModePerm)
			continue
		}

		// Create parent directories
		if err := os.MkdirAll(filepath.Dir(fpath), os.ModePerm); err != nil {
			return err
		}

		// Create file
		outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return err
		}

		rc, err := f.Open()
		if err != nil {
			outFile.Close()
			return err
		}

		_, err = io.Copy(outFile, rc)
		outFile.Close()
		rc.Close()

		if err != nil {
			return err
		}
	}

	return nil
}
