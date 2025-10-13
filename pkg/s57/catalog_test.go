package s57

import (
	"testing"
)

func TestLoadCatalog(t *testing.T) {
	catalogPath := "/Users/jcollins/Projects/render57/ENCProdCat_19115.xml"

	catalog, err := LoadCatalog(catalogPath)
	if err != nil {
		t.Fatalf("Failed to load catalog: %v", err)
	}

	if len(catalog.Entries) == 0 {
		t.Fatal("No catalog entries parsed")
	}

	t.Logf("Loaded %d chart entries", len(catalog.Entries))

	// Check how many have polygons
	withPolygons := 0
	withURLs := 0
	withDates := 0
	withKeywords := 0

	for _, entry := range catalog.Entries {
		if len(entry.Polygon) > 0 {
			withPolygons++
		}
		if entry.DownloadURL != "" {
			withURLs++
		}
		if !entry.UpdateDate.IsZero() {
			withDates++
		}
		if len(entry.Keywords) > 0 {
			withKeywords++
		}
	}

	t.Logf("Charts with names: %d", len(catalog.Entries))
	t.Logf("Charts with polygons: %d", withPolygons)
	t.Logf("Charts with URLs: %d", withURLs)
	t.Logf("Charts with dates: %d", withDates)
	t.Logf("Charts with keywords: %d", withKeywords)

	// Show first entry with polygon
	for _, entry := range catalog.Entries {
		if len(entry.Polygon) > 0 {
			t.Logf("\nFirst entry with polygon:")
			t.Logf("  Name: %s", entry.Name)
			t.Logf("  Edition: %s", entry.Edition)
			t.Logf("  Update: %s", entry.UpdateDate.Format("2006-01-02"))
			t.Logf("  URL: %s", entry.DownloadURL)
			t.Logf("  Size: %.2f MB", entry.SizeMB)
			t.Logf("  Polygon points: %d", len(entry.Polygon))
			t.Logf("  Keywords: %v", entry.Keywords)

			bounds := entry.Bounds()
			t.Logf("  Bounds: lat[%.2f, %.2f] lon[%.2f, %.2f]",
				bounds.MinLat, bounds.MaxLat, bounds.MinLon, bounds.MaxLon)
			break
		}
	}

	// Test query functionality
	testBounds := Bounds{
		MinLat: 37.0,
		MaxLat: 38.0,
		MinLon: -123.0,
		MaxLon: -122.0,
	}

	matches := catalog.Query(testBounds)
	t.Logf("\nQuery for San Francisco Bay area returned %d charts", len(matches))
}
