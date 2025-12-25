package excel

import (
	"fmt"

	"dns-collector-webapi/internal/models"

	"github.com/xuri/excelize/v2"
)

// Exporter handles Excel file generation
type Exporter struct{}

// NewExporter creates a new Excel exporter
func NewExporter() *Exporter {
	return &Exporter{}
}

// ExportStats generates an Excel file with DNS statistics
func (e *Exporter) ExportStats(stats []models.DomainStat) (*excelize.File, error) {
	f := excelize.NewFile()
	defer func() {
		_ = f.Close() // Ignore errors in cleanup
	}()

	sheetName := "DNS Statistics"
	index, err := f.NewSheet(sheetName)
	if err != nil {
		return nil, fmt.Errorf("failed to create sheet: %w", err)
	}

	f.SetActiveSheet(index)

	// Delete default Sheet1
	_ = f.DeleteSheet("Sheet1") // Ignore error if Sheet1 doesn't exist

	// Define header style
	headerStyle, err := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{
			Bold:  true,
			Color: "FFFFFF",
		},
		Fill: excelize.Fill{
			Type:    "pattern",
			Color:   []string{"4A90E2"},
			Pattern: 1,
		},
		Alignment: &excelize.Alignment{
			Horizontal: "left",
			Vertical:   "center",
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create header style: %w", err)
	}

	// Set headers
	headers := []string{"ID", "Domain", "Client IP", "Record Type", "Timestamp"}
	for i, header := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		if err := f.SetCellValue(sheetName, cell, header); err != nil {
			return nil, fmt.Errorf("failed to set header: %w", err)
		}
		if err := f.SetCellStyle(sheetName, cell, cell, headerStyle); err != nil {
			return nil, fmt.Errorf("failed to set header style: %w", err)
		}
	}

	// Set column widths
	columnWidths := map[string]float64{
		"A": 10,  // ID
		"B": 35,  // Domain
		"C": 18,  // Client IP
		"D": 12,  // Record Type
		"E": 20,  // Timestamp
	}
	for col, width := range columnWidths {
		if err := f.SetColWidth(sheetName, col, col, width); err != nil {
			return nil, fmt.Errorf("failed to set column width: %w", err)
		}
	}

	// Create date style
	dateStyle, err := f.NewStyle(&excelize.Style{
		CustomNumFmt: strPtr("yyyy-mm-dd hh:mm:ss"),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create date style: %w", err)
	}

	// Write data
	for i, stat := range stats {
		row := i + 2 // Start from row 2 (after header)

		// ID
		cell, _ := excelize.CoordinatesToCellName(1, row)
		if err := f.SetCellValue(sheetName, cell, stat.ID); err != nil {
			return nil, fmt.Errorf("failed to set cell value: %w", err)
		}

		// Domain
		cell, _ = excelize.CoordinatesToCellName(2, row)
		if err := f.SetCellValue(sheetName, cell, stat.Domain); err != nil {
			return nil, fmt.Errorf("failed to set cell value: %w", err)
		}

		// Client IP
		cell, _ = excelize.CoordinatesToCellName(3, row)
		if err := f.SetCellValue(sheetName, cell, stat.ClientIP); err != nil {
			return nil, fmt.Errorf("failed to set cell value: %w", err)
		}

		// Record Type
		cell, _ = excelize.CoordinatesToCellName(4, row)
		if err := f.SetCellValue(sheetName, cell, stat.RType); err != nil {
			return nil, fmt.Errorf("failed to set cell value: %w", err)
		}

		// Timestamp
		cell, _ = excelize.CoordinatesToCellName(5, row)
		if err := f.SetCellValue(sheetName, cell, stat.Timestamp); err != nil {
			return nil, fmt.Errorf("failed to set cell value: %w", err)
		}
		if err := f.SetCellStyle(sheetName, cell, cell, dateStyle); err != nil {
			return nil, fmt.Errorf("failed to set date style: %w", err)
		}
	}

	// Freeze first row
	if err := f.SetPanes(sheetName, &excelize.Panes{
		Freeze:      true,
		Split:       false,
		XSplit:      0,
		YSplit:      1,
		TopLeftCell: "A2",
		ActivePane:  "bottomLeft",
	}); err != nil {
		return nil, fmt.Errorf("failed to freeze panes: %w", err)
	}

	// Add auto-filter
	if len(stats) > 0 {
		lastCol, _ := excelize.CoordinatesToCellName(5, len(stats)+1)
		filterRange := fmt.Sprintf("A1:%s", lastCol)
		if err := f.AutoFilter(sheetName, filterRange, []excelize.AutoFilterOptions{}); err != nil {
			return nil, fmt.Errorf("failed to add auto-filter: %w", err)
		}
	}

	return f, nil
}

// ExportDomains generates an Excel file with domains and their IPs
func (e *Exporter) ExportDomains(domains []models.Domain) (*excelize.File, error) {
	f := excelize.NewFile()
	defer func() {
		_ = f.Close() // Ignore errors in cleanup
	}()

	// Create Domains sheet
	domainsSheet := "Domains"
	index, err := f.NewSheet(domainsSheet)
	if err != nil {
		return nil, fmt.Errorf("failed to create domains sheet: %w", err)
	}

	f.SetActiveSheet(index)

	// Delete default Sheet1
	_ = f.DeleteSheet("Sheet1") // Ignore error if Sheet1 doesn't exist

	// Define header style
	headerStyle, err := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{
			Bold:  true,
			Color: "FFFFFF",
		},
		Fill: excelize.Fill{
			Type:    "pattern",
			Color:   []string{"4A90E2"},
			Pattern: 1,
		},
		Alignment: &excelize.Alignment{
			Horizontal: "left",
			Vertical:   "center",
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create header style: %w", err)
	}

	// Create date style
	dateStyle, err := f.NewStyle(&excelize.Style{
		CustomNumFmt: strPtr("yyyy-mm-dd hh:mm:ss"),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create date style: %w", err)
	}

	// Sheet 1: Domains
	headers := []string{"ID", "Domain", "First Seen", "Last Seen", "Resolution Count", "Max Resolutions", "Last Resolved"}
	for i, header := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		if err := f.SetCellValue(domainsSheet, cell, header); err != nil {
			return nil, fmt.Errorf("failed to set header: %w", err)
		}
		if err := f.SetCellStyle(domainsSheet, cell, cell, headerStyle); err != nil {
			return nil, fmt.Errorf("failed to set header style: %w", err)
		}
	}

	// Set column widths for Domains sheet
	columnWidths := map[string]float64{
		"A": 10, // ID
		"B": 35, // Domain
		"C": 20, // First Seen
		"D": 20, // Last Seen
		"E": 18, // Resolution Count
		"F": 18, // Max Resolutions
		"G": 20, // Last Resolved
	}
	for col, width := range columnWidths {
		if err := f.SetColWidth(domainsSheet, col, col, width); err != nil {
			return nil, fmt.Errorf("failed to set column width: %w", err)
		}
	}

	// Write domains data
	for i, domain := range domains {
		row := i + 2

		cells := []struct {
			col   int
			value interface{}
			style int
		}{
			{1, domain.ID, 0},
			{2, domain.Domain, 0},
			{3, domain.TimeInsert, dateStyle},
			{4, domain.LastSeen, dateStyle},
			{5, domain.ResolvCount, 0},
			{6, domain.MaxResolv, 0},
			{7, domain.LastResolvTime, dateStyle},
		}

		for _, c := range cells {
			cell, _ := excelize.CoordinatesToCellName(c.col, row)
			if err := f.SetCellValue(domainsSheet, cell, c.value); err != nil {
				return nil, fmt.Errorf("failed to set cell value: %w", err)
			}
			if c.style != 0 {
				if err := f.SetCellStyle(domainsSheet, cell, cell, c.style); err != nil {
					return nil, fmt.Errorf("failed to set cell style: %w", err)
				}
			}
		}
	}

	// Freeze first row on Domains sheet
	if err := f.SetPanes(domainsSheet, &excelize.Panes{
		Freeze:      true,
		Split:       false,
		XSplit:      0,
		YSplit:      1,
		TopLeftCell: "A2",
		ActivePane:  "bottomLeft",
	}); err != nil {
		return nil, fmt.Errorf("failed to freeze panes: %w", err)
	}

	// Add auto-filter to Domains sheet
	if len(domains) > 0 {
		lastCol, _ := excelize.CoordinatesToCellName(7, len(domains)+1)
		filterRange := fmt.Sprintf("A1:%s", lastCol)
		if err := f.AutoFilter(domainsSheet, filterRange, []excelize.AutoFilterOptions{}); err != nil {
			return nil, fmt.Errorf("failed to add auto-filter: %w", err)
		}
	}

	// Sheet 2: IP Addresses
	ipsSheet := "IP Addresses"
	if _, err := f.NewSheet(ipsSheet); err != nil {
		return nil, fmt.Errorf("failed to create IPs sheet: %w", err)
	}

	ipHeaders := []string{"Domain", "IP Address", "Type", "Resolved At"}
	for i, header := range ipHeaders {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		if err := f.SetCellValue(ipsSheet, cell, header); err != nil {
			return nil, fmt.Errorf("failed to set header: %w", err)
		}
		if err := f.SetCellStyle(ipsSheet, cell, cell, headerStyle); err != nil {
			return nil, fmt.Errorf("failed to set header style: %w", err)
		}
	}

	// Set column widths for IPs sheet
	ipColumnWidths := map[string]float64{
		"A": 35, // Domain
		"B": 20, // IP Address
		"C": 10, // Type
		"D": 20, // Resolved At
	}
	for col, width := range ipColumnWidths {
		if err := f.SetColWidth(ipsSheet, col, col, width); err != nil {
			return nil, fmt.Errorf("failed to set column width: %w", err)
		}
	}

	// Write IPs data
	ipRow := 2
	for _, domain := range domains {
		for _, ip := range domain.IPs {
			cells := []struct {
				col   int
				value interface{}
				style int
			}{
				{1, domain.Domain, 0},
				{2, ip.IP, 0},
				{3, ip.Type, 0},
				{4, ip.Time, dateStyle},
			}

			for _, c := range cells {
				cell, _ := excelize.CoordinatesToCellName(c.col, ipRow)
				if err := f.SetCellValue(ipsSheet, cell, c.value); err != nil {
					return nil, fmt.Errorf("failed to set cell value: %w", err)
				}
				if c.style != 0 {
					if err := f.SetCellStyle(ipsSheet, cell, cell, c.style); err != nil {
						return nil, fmt.Errorf("failed to set cell style: %w", err)
					}
				}
			}
			ipRow++
		}
	}

	// Freeze first row on IPs sheet
	if err := f.SetPanes(ipsSheet, &excelize.Panes{
		Freeze:      true,
		Split:       false,
		XSplit:      0,
		YSplit:      1,
		TopLeftCell: "A2",
		ActivePane:  "bottomLeft",
	}); err != nil {
		return nil, fmt.Errorf("failed to freeze panes: %w", err)
	}

	// Add auto-filter to IPs sheet
	if ipRow > 2 {
		lastCol, _ := excelize.CoordinatesToCellName(4, ipRow-1)
		filterRange := fmt.Sprintf("A1:%s", lastCol)
		if err := f.AutoFilter(ipsSheet, filterRange, []excelize.AutoFilterOptions{}); err != nil {
			return nil, fmt.Errorf("failed to add auto-filter: %w", err)
		}
	}

	return f, nil
}

// strPtr returns a pointer to a string (helper for excelize style)
func strPtr(s string) *string {
	return &s
}
