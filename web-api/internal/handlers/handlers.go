package handlers

import (
	"log"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"dns-collector-webapi/internal/database"
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
