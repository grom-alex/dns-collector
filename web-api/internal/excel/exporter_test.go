package excel

import (
	"testing"
	"time"

	"dns-collector-webapi/internal/models"

	"github.com/xuri/excelize/v2"
)

func TestExportStats(t *testing.T) {
	exporter := NewExporter()

	// Test with empty data
	t.Run("Empty data", func(t *testing.T) {
		stats := []models.DomainStat{}
		file, err := exporter.ExportStats(stats)
		if err != nil {
			t.Fatalf("ExportStats failed with empty data: %v", err)
		}
		defer func() { _ = file.Close() }()

		// Verify sheet exists
		sheets := file.GetSheetList()
		if len(sheets) != 1 || sheets[0] != "DNS Statistics" {
			t.Errorf("Expected sheet 'DNS Statistics', got %v", sheets)
		}

		// Verify headers
		verifyStatsHeaders(t, file, "DNS Statistics")
	})

	// Test with sample data
	t.Run("With data", func(t *testing.T) {
		now := time.Now()
		stats := []models.DomainStat{
			{
				ID:        1,
				Domain:    "example.com",
				ClientIP:  "192.168.1.100",
				RType:     "A",
				Timestamp: now,
			},
			{
				ID:        2,
				Domain:    "test.org",
				ClientIP:  "10.0.0.50",
				RType:     "AAAA",
				Timestamp: now.Add(-1 * time.Hour),
			},
		}

		file, err := exporter.ExportStats(stats)
		if err != nil {
			t.Fatalf("ExportStats failed: %v", err)
		}
		defer func() { _ = file.Close() }()

		// Verify headers
		verifyStatsHeaders(t, file, "DNS Statistics")

		// Verify data
		sheetName := "DNS Statistics"

		// Check first data row
		id, err := file.GetCellValue(sheetName, "A2")
		if err != nil || id != "1" {
			t.Errorf("Expected ID 1, got %s (error: %v)", id, err)
		}

		domain, err := file.GetCellValue(sheetName, "B2")
		if err != nil || domain != "example.com" {
			t.Errorf("Expected domain 'example.com', got %s (error: %v)", domain, err)
		}

		clientIP, err := file.GetCellValue(sheetName, "C2")
		if err != nil || clientIP != "192.168.1.100" {
			t.Errorf("Expected client IP '192.168.1.100', got %s (error: %v)", clientIP, err)
		}

		rtype, err := file.GetCellValue(sheetName, "D2")
		if err != nil || rtype != "A" {
			t.Errorf("Expected record type 'A', got %s (error: %v)", rtype, err)
		}

		// Verify freeze panes
		panes, err := file.GetPanes(sheetName)
		if err != nil {
			t.Errorf("Failed to get panes: %v", err)
		}
		if !panes.Freeze {
			t.Error("Expected freeze panes to be enabled")
		}
	})

	// Test with large dataset
	t.Run("Large dataset", func(t *testing.T) {
		stats := make([]models.DomainStat, 1000)
		now := time.Now()
		for i := 0; i < 1000; i++ {
			stats[i] = models.DomainStat{
				ID:        int64(i + 1),
				Domain:    "example.com",
				ClientIP:  "192.168.1.1",
				RType:     "A",
				Timestamp: now,
			}
		}

		file, err := exporter.ExportStats(stats)
		if err != nil {
			t.Fatalf("ExportStats failed with large dataset: %v", err)
		}
		defer func() { _ = file.Close() }()

		// Verify last row
		lastID, err := file.GetCellValue("DNS Statistics", "A1001")
		if err != nil || lastID != "1000" {
			t.Errorf("Expected last ID 1000, got %s (error: %v)", lastID, err)
		}
	})
}

func TestExportDomains(t *testing.T) {
	exporter := NewExporter()

	// Test with empty data
	t.Run("Empty data", func(t *testing.T) {
		domains := []models.Domain{}
		file, err := exporter.ExportDomains(domains)
		if err != nil {
			t.Fatalf("ExportDomains failed with empty data: %v", err)
		}
		defer func() { _ = file.Close() }()

		// Verify sheets exist
		sheets := file.GetSheetList()
		if len(sheets) != 2 {
			t.Errorf("Expected 2 sheets, got %d", len(sheets))
		}

		// Verify Domains sheet headers
		verifyDomainsHeaders(t, file, "Domains")

		// Verify IPs sheet headers
		verifyIPsHeaders(t, file, "IP Addresses")
	})

	// Test with sample data
	t.Run("With data", func(t *testing.T) {
		now := time.Now()
		domains := []models.Domain{
			{
				ID:             1,
				Domain:         "example.com",
				TimeInsert:     now.Add(-24 * time.Hour),
				ResolvCount:    10,
				MaxResolv:      5,
				LastResolvTime: now,
				IPs: []models.IP{
					{
						ID:       1,
						DomainID: 1,
						IP:       "93.184.216.34",
						Type:     "IPv4",
						Time:     now,
					},
					{
						ID:       2,
						DomainID: 1,
						IP:       "2606:2800:220:1:248:1893:25c8:1946",
						Type:     "IPv6",
						Time:     now,
					},
				},
			},
			{
				ID:             2,
				Domain:         "test.org",
				TimeInsert:     now.Add(-48 * time.Hour),
				ResolvCount:    5,
				MaxResolv:      3,
				LastResolvTime: now.Add(-1 * time.Hour),
				IPs: []models.IP{
					{
						ID:       3,
						DomainID: 2,
						IP:       "1.2.3.4",
						Type:     "IPv4",
						Time:     now.Add(-1 * time.Hour),
					},
				},
			},
		}

		file, err := exporter.ExportDomains(domains)
		if err != nil {
			t.Fatalf("ExportDomains failed: %v", err)
		}
		defer func() { _ = file.Close() }()

		// Verify Domains sheet data
		domain, err := file.GetCellValue("Domains", "B2")
		if err != nil || domain != "example.com" {
			t.Errorf("Expected domain 'example.com', got %s (error: %v)", domain, err)
		}

		resolvCount, err := file.GetCellValue("Domains", "E2")
		if err != nil || resolvCount != "10" {
			t.Errorf("Expected resolv count '10', got %s (error: %v)", resolvCount, err)
		}

		// Verify IPs sheet data
		ipAddr, err := file.GetCellValue("IP Addresses", "B2")
		if err != nil || ipAddr != "93.184.216.34" {
			t.Errorf("Expected IP '93.184.216.34', got %s (error: %v)", ipAddr, err)
		}

		ipType, err := file.GetCellValue("IP Addresses", "C2")
		if err != nil || ipType != "IPv4" {
			t.Errorf("Expected type 'IPv4', got %s (error: %v)", ipType, err)
		}

		// Verify second IP
		ipAddr2, err := file.GetCellValue("IP Addresses", "B3")
		if err != nil || ipAddr2 != "2606:2800:220:1:248:1893:25c8:1946" {
			t.Errorf("Expected IPv6 address, got %s (error: %v)", ipAddr2, err)
		}

		// Verify freeze panes on both sheets
		for _, sheetName := range []string{"Domains", "IP Addresses"} {
			panes, err := file.GetPanes(sheetName)
			if err != nil {
				t.Errorf("Failed to get panes for %s: %v", sheetName, err)
			}
			if !panes.Freeze {
				t.Errorf("Expected freeze panes to be enabled on %s", sheetName)
			}
		}
	})

	// Test domain without IPs
	t.Run("Domain without IPs", func(t *testing.T) {
		now := time.Now()
		domains := []models.Domain{
			{
				ID:             1,
				Domain:         "noips.com",
				TimeInsert:     now,
				ResolvCount:    0,
				MaxResolv:      0,
				LastResolvTime: now,
				IPs:            []models.IP{},
			},
		}

		file, err := exporter.ExportDomains(domains)
		if err != nil {
			t.Fatalf("ExportDomains failed with domain without IPs: %v", err)
		}
		defer func() { _ = file.Close() }()

		// Verify domain exists in Domains sheet
		domain, err := file.GetCellValue("Domains", "B2")
		if err != nil || domain != "noips.com" {
			t.Errorf("Expected domain 'noips.com', got %s (error: %v)", domain, err)
		}

		// Verify IPs sheet only has headers
		ipAddr, _ := file.GetCellValue("IP Addresses", "B2")
		if ipAddr != "" {
			t.Errorf("Expected no IPs in IP Addresses sheet, got %s", ipAddr)
		}
	})
}

func verifyStatsHeaders(t *testing.T, file *excelize.File, sheetName string) {
	expectedHeaders := []struct {
		cell  string
		value string
	}{
		{"A1", "ID"},
		{"B1", "Domain"},
		{"C1", "Client IP"},
		{"D1", "Record Type"},
		{"E1", "Timestamp"},
	}

	for _, h := range expectedHeaders {
		val, err := file.GetCellValue(sheetName, h.cell)
		if err != nil {
			t.Errorf("Failed to get header %s: %v", h.cell, err)
		}
		if val != h.value {
			t.Errorf("Expected header %s to be '%s', got '%s'", h.cell, h.value, val)
		}
	}
}

func verifyDomainsHeaders(t *testing.T, file *excelize.File, sheetName string) {
	expectedHeaders := []struct {
		cell  string
		value string
	}{
		{"A1", "ID"},
		{"B1", "Domain"},
		{"C1", "First Seen"},
		{"D1", "Last Seen"},
		{"E1", "Resolution Count"},
		{"F1", "Max Resolutions"},
		{"G1", "Last Resolved"},
	}

	for _, h := range expectedHeaders {
		val, err := file.GetCellValue(sheetName, h.cell)
		if err != nil {
			t.Errorf("Failed to get header %s: %v", h.cell, err)
		}
		if val != h.value {
			t.Errorf("Expected header %s to be '%s', got '%s'", h.cell, h.value, val)
		}
	}
}

func verifyIPsHeaders(t *testing.T, file *excelize.File, sheetName string) {
	expectedHeaders := []struct {
		cell  string
		value string
	}{
		{"A1", "Domain"},
		{"B1", "IP Address"},
		{"C1", "Type"},
		{"D1", "Resolved At"},
	}

	for _, h := range expectedHeaders {
		val, err := file.GetCellValue(sheetName, h.cell)
		if err != nil {
			t.Errorf("Failed to get header %s: %v", h.cell, err)
		}
		if val != h.value {
			t.Errorf("Expected header %s to be '%s', got '%s'", h.cell, h.value, val)
		}
	}
}
