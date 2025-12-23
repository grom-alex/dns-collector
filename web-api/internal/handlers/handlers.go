package handlers

import (
	"fmt"
	"log"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"dns-collector-webapi/internal/database"
	"dns-collector-webapi/internal/excel"
	"dns-collector-webapi/internal/models"
)

type Handler struct {
	db database.DB
}

func NewHandler(db database.DB) *Handler {
	return &Handler{db: db}
}

// GetStats handles GET /api/stats
func (h *Handler) GetStats(c *gin.Context) {
	var filter models.StatsFilter

	// Parse client IPs
	if clientIPs := c.Query("client_ips"); clientIPs != "" {
		filter.ClientIPs = strings.Split(clientIPs, ",")
		// Trim spaces
		for i := range filter.ClientIPs {
			filter.ClientIPs[i] = strings.TrimSpace(filter.ClientIPs[i])
		}
	}

	// Parse subnet
	filter.Subnet = c.Query("subnet")

	// Parse date range
	if dateFrom := c.Query("date_from"); dateFrom != "" {
		if t, err := time.Parse(time.RFC3339, dateFrom); err == nil {
			filter.DateFrom = t
		}
	}
	if dateTo := c.Query("date_to"); dateTo != "" {
		if t, err := time.Parse(time.RFC3339, dateTo); err == nil {
			filter.DateTo = t
		}
	}

	// Parse sorting
	filter.SortBy = c.DefaultQuery("sort_by", "timestamp")
	filter.SortOrder = c.DefaultQuery("sort_order", "desc")

	// Parse pagination
	if limit := c.Query("limit"); limit != "" {
		if l, err := strconv.Atoi(limit); err == nil {
			filter.Limit = l
		}
	} else {
		filter.Limit = 100
	}

	if offset := c.Query("offset"); offset != "" {
		if o, err := strconv.Atoi(offset); err == nil {
			filter.Offset = o
		}
	}

	// Query database
	stats, total, err := h.db.GetStats(filter)
	if err != nil {
		log.Printf("Error getting stats: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Calculate total pages
	totalPages := int(math.Ceil(float64(total) / float64(filter.Limit)))

	c.JSON(http.StatusOK, models.PaginatedResponse{
		Data:       stats,
		Total:      total,
		Limit:      filter.Limit,
		Offset:     filter.Offset,
		TotalPages: totalPages,
	})
}

// GetDomains handles GET /api/domains
func (h *Handler) GetDomains(c *gin.Context) {
	var filter models.DomainsFilter

	// Parse domain regex
	filter.DomainRegex = c.Query("domain_regex")

	// Parse date range
	if dateFrom := c.Query("date_from"); dateFrom != "" {
		if t, err := time.Parse(time.RFC3339, dateFrom); err == nil {
			filter.DateFrom = t
		}
	}
	if dateTo := c.Query("date_to"); dateTo != "" {
		if t, err := time.Parse(time.RFC3339, dateTo); err == nil {
			filter.DateTo = t
		}
	}

	// Parse sorting
	filter.SortBy = c.DefaultQuery("sort_by", "time_insert")
	filter.SortOrder = c.DefaultQuery("sort_order", "desc")

	// Parse pagination
	if limit := c.Query("limit"); limit != "" {
		if l, err := strconv.Atoi(limit); err == nil {
			filter.Limit = l
		}
	} else {
		filter.Limit = 100
	}

	if offset := c.Query("offset"); offset != "" {
		if o, err := strconv.Atoi(offset); err == nil {
			filter.Offset = o
		}
	}

	// Query database
	domains, total, err := h.db.GetDomains(filter)
	if err != nil {
		log.Printf("Error getting domains: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Calculate total pages
	totalPages := int(math.Ceil(float64(total) / float64(filter.Limit)))

	c.JSON(http.StatusOK, models.PaginatedResponse{
		Data:       domains,
		Total:      total,
		Limit:      filter.Limit,
		Offset:     filter.Offset,
		TotalPages: totalPages,
	})
}

// GetDomainByID handles GET /api/domains/:id
func (h *Handler) GetDomainByID(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid domain ID"})
		return
	}

	domain, err := h.db.GetDomainWithIPs(id)
	if err != nil {
		log.Printf("Error getting domain: %v", err)
		if strings.Contains(err.Error(), "not found") {
			c.JSON(http.StatusNotFound, gin.H{"error": "domain not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, domain)
}

// HealthCheck handles GET /health
func (h *Handler) HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
		"time":   time.Now(),
	})
}

// ExportList handles export list endpoints
func (h *Handler) ExportList(c *gin.Context, domainRegex string, includeDomains bool) {
	// Get data from database
	exportList, err := h.db.GetExportList(domainRegex)
	if err != nil {
		log.Printf("Error getting export list: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Log if export list is empty (useful for debugging)
	if len(exportList.Domains) == 0 && len(exportList.IPv4) == 0 && len(exportList.IPv6) == 0 {
		log.Printf("Export list returned empty results for regex: %s", domainRegex)
	}

	// Build plain text response
	var result strings.Builder

	// Add domains if enabled
	if includeDomains {
		for _, domain := range exportList.Domains {
			// Remove trailing dot from FQDN if present
			domain = strings.TrimSuffix(domain, ".")
			result.WriteString(domain)
			result.WriteString("\n")
		}
	}

	// Add IPv4 addresses
	for _, ip := range exportList.IPv4 {
		result.WriteString(ip)
		result.WriteString("\n")
	}

	// Add IPv6 addresses
	for _, ip := range exportList.IPv6 {
		result.WriteString(ip)
		result.WriteString("\n")
	}

	// Return as plain text with caching headers
	c.Header("Content-Type", "text/plain; charset=utf-8")
	c.Header("Cache-Control", "public, max-age=300") // 5 minutes cache
	c.String(http.StatusOK, result.String())
}

// ExportStats handles GET /api/stats/export - exports stats to Excel
func (h *Handler) ExportStats(c *gin.Context) {
	var filter models.StatsFilter

	// Parse client IPs
	if clientIPs := c.Query("client_ips"); clientIPs != "" {
		filter.ClientIPs = strings.Split(clientIPs, ",")
		// Trim spaces
		for i := range filter.ClientIPs {
			filter.ClientIPs[i] = strings.TrimSpace(filter.ClientIPs[i])
		}
	}

	// Parse subnet
	filter.Subnet = c.Query("subnet")

	// Parse date range
	if dateFrom := c.Query("date_from"); dateFrom != "" {
		if t, err := time.Parse(time.RFC3339, dateFrom); err == nil {
			filter.DateFrom = t
		}
	}
	if dateTo := c.Query("date_to"); dateTo != "" {
		if t, err := time.Parse(time.RFC3339, dateTo); err == nil {
			filter.DateTo = t
		}
	}

	// Parse sorting
	filter.SortBy = c.DefaultQuery("sort_by", "timestamp")
	filter.SortOrder = c.DefaultQuery("sort_order", "desc")

	// Set high limit for export (100K max)
	filter.Limit = 100000
	filter.Offset = 0

	// Query database
	stats, total, err := h.db.GetStats(filter)
	if err != nil {
		log.Printf("Error getting stats for export: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Check if total exceeds limit
	if total > 100000 {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{
			"error": "Dataset too large to export",
			"total": total,
			"limit": 100000,
		})
		return
	}

	// Generate Excel file
	exporter := excel.NewExporter()
	file, err := exporter.ExportStats(stats)
	if err != nil {
		log.Printf("Excel generation failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate Excel file"})
		return
	}
	defer func() {
		if err := file.Close(); err != nil {
			log.Printf("Error closing Excel file: %v", err)
		}
	}()

	// Set headers
	filename := fmt.Sprintf("dns-stats-%s.xlsx", time.Now().Format("2006-01-02"))
	c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	c.Header("Cache-Control", "no-cache")

	// Write to response
	if err := file.Write(c.Writer); err != nil {
		log.Printf("Failed to write Excel to response: %v", err)
	}
}

// ExportDomains handles GET /api/domains/export - exports domains to Excel
func (h *Handler) ExportDomains(c *gin.Context) {
	var filter models.DomainsFilter

	// Parse domain regex
	filter.DomainRegex = c.Query("domain_regex")

	// Parse date range
	if dateFrom := c.Query("date_from"); dateFrom != "" {
		if t, err := time.Parse(time.RFC3339, dateFrom); err == nil {
			filter.DateFrom = t
		}
	}
	if dateTo := c.Query("date_to"); dateTo != "" {
		if t, err := time.Parse(time.RFC3339, dateTo); err == nil {
			filter.DateTo = t
		}
	}

	// Parse sorting
	filter.SortBy = c.DefaultQuery("sort_by", "time_insert")
	filter.SortOrder = c.DefaultQuery("sort_order", "desc")

	// Set high limit for export (100K max)
	filter.Limit = 100000
	filter.Offset = 0

	// Query database with bulk IP fetch
	domains, total, err := h.db.GetDomainsWithIPs(filter)
	if err != nil {
		log.Printf("Error getting domains for export: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Check if total exceeds limit
	if total > 100000 {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{
			"error": "Dataset too large to export",
			"total": total,
			"limit": 100000,
		})
		return
	}

	// Generate Excel file
	exporter := excel.NewExporter()
	file, err := exporter.ExportDomains(domains)
	if err != nil {
		log.Printf("Excel generation failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate Excel file"})
		return
	}
	defer func() {
		if err := file.Close(); err != nil {
			log.Printf("Error closing Excel file: %v", err)
		}
	}()

	// Set headers
	filename := fmt.Sprintf("dns-domains-%s.xlsx", time.Now().Format("2006-01-02"))
	c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	c.Header("Cache-Control", "no-cache")

	// Write to response
	if err := file.Write(c.Writer); err != nil {
		log.Printf("Failed to write Excel to response: %v", err)
	}
}
