package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"dns-collector-webapi/internal/models"
)

// MockDatabase implements database.DB interface for testing
type MockDatabase struct {
	GetStatsFunc          func(filter models.StatsFilter) ([]models.DomainStat, int64, error)
	GetDomainsFunc        func(filter models.DomainsFilter) ([]models.Domain, int64, error)
	GetDomainWithIPsFunc  func(id int64) (*models.Domain, error)
	GetDomainsWithIPsFunc func(filter models.DomainsFilter) ([]models.Domain, int64, error)
	GetExportListFunc     func(domainRegex string) (*models.ExportList, error)
}

func (m *MockDatabase) GetStats(filter models.StatsFilter) ([]models.DomainStat, int64, error) {
	if m.GetStatsFunc != nil {
		return m.GetStatsFunc(filter)
	}
	return nil, 0, nil
}

func (m *MockDatabase) GetDomains(filter models.DomainsFilter) ([]models.Domain, int64, error) {
	if m.GetDomainsFunc != nil {
		return m.GetDomainsFunc(filter)
	}
	return nil, 0, nil
}

func (m *MockDatabase) GetDomainWithIPs(id int64) (*models.Domain, error) {
	if m.GetDomainWithIPsFunc != nil {
		return m.GetDomainWithIPsFunc(id)
	}
	return nil, nil
}

func (m *MockDatabase) GetDomainsWithIPs(filter models.DomainsFilter) ([]models.Domain, int64, error) {
	if m.GetDomainsWithIPsFunc != nil {
		return m.GetDomainsWithIPsFunc(filter)
	}
	return nil, 0, nil
}

func (m *MockDatabase) GetExportList(domainRegex string) (*models.ExportList, error) {
	if m.GetExportListFunc != nil {
		return m.GetExportListFunc(domainRegex)
	}
	return &models.ExportList{}, nil
}

func (m *MockDatabase) Close() error {
	return nil
}

func setupTestRouter() (*gin.Engine, *MockDatabase) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	mockDB := &MockDatabase{}
	return router, mockDB
}

func TestHealthCheck(t *testing.T) {
	router, mockDB := setupTestRouter()
	h := NewHandler(mockDB)
	router.GET("/health", h.HealthCheck)

	req, _ := http.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if response["status"] != "ok" {
		t.Errorf("Expected status=ok, got %v", response["status"])
	}

	if _, ok := response["time"]; !ok {
		t.Error("Expected time field in response")
	}
}

func TestGetStats_Success(t *testing.T) {
	router, mockDB := setupTestRouter()

	mockStats := []models.DomainStat{
		{
			ID:        1,
			Domain:    "example.com",
			ClientIP:  "192.168.1.1",
			RType:     "A",
			Timestamp: time.Now(),
		},
		{
			ID:        2,
			Domain:    "test.com",
			ClientIP:  "192.168.1.2",
			RType:     "AAAA",
			Timestamp: time.Now(),
		},
	}

	mockDB.GetStatsFunc = func(filter models.StatsFilter) ([]models.DomainStat, int64, error) {
		return mockStats, 2, nil
	}

	h := NewHandler(mockDB)
	router.GET("/api/stats", h.GetStats)

	req, _ := http.NewRequest(http.MethodGet, "/api/stats", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response models.PaginatedResponse
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if response.Total != 2 {
		t.Errorf("Expected total=2, got %d", response.Total)
	}

	if response.Limit != 100 {
		t.Errorf("Expected default limit=100, got %d", response.Limit)
	}
}

func TestGetStats_WithPagination(t *testing.T) {
	router, mockDB := setupTestRouter()

	var capturedFilter models.StatsFilter
	mockDB.GetStatsFunc = func(filter models.StatsFilter) ([]models.DomainStat, int64, error) {
		capturedFilter = filter
		return []models.DomainStat{}, 50, nil
	}

	h := NewHandler(mockDB)
	router.GET("/api/stats", h.GetStats)

	req, _ := http.NewRequest(http.MethodGet, "/api/stats?limit=20&offset=10", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	if capturedFilter.Limit != 20 {
		t.Errorf("Expected limit=20, got %d", capturedFilter.Limit)
	}

	if capturedFilter.Offset != 10 {
		t.Errorf("Expected offset=10, got %d", capturedFilter.Offset)
	}

	var response models.PaginatedResponse
	_ = json.Unmarshal(w.Body.Bytes(), &response)

	// Total pages should be ceil(50 / 20) = 3
	if response.TotalPages != 3 {
		t.Errorf("Expected total_pages=3, got %d", response.TotalPages)
	}
}

func TestGetStats_DatabaseError(t *testing.T) {
	router, mockDB := setupTestRouter()

	mockDB.GetStatsFunc = func(filter models.StatsFilter) ([]models.DomainStat, int64, error) {
		return nil, 0, errors.New("database connection failed")
	}

	h := NewHandler(mockDB)
	router.GET("/api/stats", h.GetStats)

	req, _ := http.NewRequest(http.MethodGet, "/api/stats", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 500, got %d", w.Code)
	}

	var response map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &response)

	if _, ok := response["error"]; !ok {
		t.Error("Expected error field in response")
	}
}

func TestGetDomainByID_Success(t *testing.T) {
	router, mockDB := setupTestRouter()

	mockDomain := &models.Domain{
		ID:             1,
		Domain:         "example.com",
		TimeInsert:     time.Now(),
		ResolvCount:    5,
		MaxResolv:      10,
		LastResolvTime: time.Now(),
		IPs: []models.IP{
			{ID: 1, DomainID: 1, IP: "192.168.1.1", Type: "A", Time: time.Now()},
		},
	}

	mockDB.GetDomainWithIPsFunc = func(id int64) (*models.Domain, error) {
		if id == 1 {
			return mockDomain, nil
		}
		return nil, errors.New("domain not found")
	}

	h := NewHandler(mockDB)
	router.GET("/api/domains/:id", h.GetDomainByID)

	req, _ := http.NewRequest(http.MethodGet, "/api/domains/1", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response models.Domain
	_ = json.Unmarshal(w.Body.Bytes(), &response)

	if response.Domain != "example.com" {
		t.Errorf("Expected domain=example.com, got %s", response.Domain)
	}

	if len(response.IPs) != 1 {
		t.Errorf("Expected 1 IP, got %d", len(response.IPs))
	}
}

func TestGetDomainByID_InvalidID(t *testing.T) {
	router, mockDB := setupTestRouter()
	h := NewHandler(mockDB)
	router.GET("/api/domains/:id", h.GetDomainByID)

	req, _ := http.NewRequest(http.MethodGet, "/api/domains/invalid", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}

	var response map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &response)

	if response["error"] != "invalid domain ID" {
		t.Errorf("Expected 'invalid domain ID' error, got %v", response["error"])
	}
}

func TestGetDomainByID_NotFound(t *testing.T) {
	router, mockDB := setupTestRouter()

	mockDB.GetDomainWithIPsFunc = func(id int64) (*models.Domain, error) {
		return nil, errors.New("domain not found")
	}

	h := NewHandler(mockDB)
	router.GET("/api/domains/:id", h.GetDomainByID)

	req, _ := http.NewRequest(http.MethodGet, "/api/domains/999", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestGetStats_WithClientIPs(t *testing.T) {
	router, mockDB := setupTestRouter()

	var capturedFilter models.StatsFilter
	mockDB.GetStatsFunc = func(filter models.StatsFilter) ([]models.DomainStat, int64, error) {
		capturedFilter = filter
		return []models.DomainStat{}, 0, nil
	}

	h := NewHandler(mockDB)
	router.GET("/api/stats", h.GetStats)

	req, _ := http.NewRequest(http.MethodGet, "/api/stats?client_ips=192.168.1.1,192.168.1.2", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	if len(capturedFilter.ClientIPs) != 2 {
		t.Errorf("Expected 2 client IPs, got %d", len(capturedFilter.ClientIPs))
	}
}

func TestGetStats_WithSubnet(t *testing.T) {
	router, mockDB := setupTestRouter()

	var capturedFilter models.StatsFilter
	mockDB.GetStatsFunc = func(filter models.StatsFilter) ([]models.DomainStat, int64, error) {
		capturedFilter = filter
		return []models.DomainStat{}, 0, nil
	}

	h := NewHandler(mockDB)
	router.GET("/api/stats", h.GetStats)

	req, _ := http.NewRequest(http.MethodGet, "/api/stats?subnet=192.168.1.0/24", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	if capturedFilter.Subnet != "192.168.1.0/24" {
		t.Errorf("Expected subnet=192.168.1.0/24, got %s", capturedFilter.Subnet)
	}
}


func TestGetStats_WithSorting(t *testing.T) {
	router, mockDB := setupTestRouter()

	var capturedFilter models.StatsFilter
	mockDB.GetStatsFunc = func(filter models.StatsFilter) ([]models.DomainStat, int64, error) {
		capturedFilter = filter
		return []models.DomainStat{}, 0, nil
	}

	h := NewHandler(mockDB)
	router.GET("/api/stats", h.GetStats)

	req, _ := http.NewRequest(http.MethodGet, "/api/stats?sort_by=domain&sort_order=asc", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	if capturedFilter.SortBy != "domain" {
		t.Errorf("Expected sort_by=domain, got %s", capturedFilter.SortBy)
	}

	if capturedFilter.SortOrder != "asc" {
		t.Errorf("Expected sort_order=asc, got %s", capturedFilter.SortOrder)
	}
}

func TestGetDomains_Success(t *testing.T) {
	router, mockDB := setupTestRouter()

	mockDomains := []models.Domain{
		{
			ID:             1,
			Domain:         "example.com",
			TimeInsert:     time.Now(),
			ResolvCount:    5,
			MaxResolv:      10,
			LastResolvTime: time.Now(),
		},
	}

	mockDB.GetDomainsFunc = func(filter models.DomainsFilter) ([]models.Domain, int64, error) {
		return mockDomains, 1, nil
	}

	h := NewHandler(mockDB)
	router.GET("/api/domains", h.GetDomains)

	req, _ := http.NewRequest(http.MethodGet, "/api/domains", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response models.PaginatedResponse
	_ = json.Unmarshal(w.Body.Bytes(), &response)

	if response.Total != 1 {
		t.Errorf("Expected total=1, got %d", response.Total)
	}
}

func TestGetDomains_WithDomainRegex(t *testing.T) {
	router, mockDB := setupTestRouter()

	var capturedFilter models.DomainsFilter
	mockDB.GetDomainsFunc = func(filter models.DomainsFilter) ([]models.Domain, int64, error) {
		capturedFilter = filter
		return []models.Domain{}, 0, nil
	}

	h := NewHandler(mockDB)
	router.GET("/api/domains", h.GetDomains)

	req, _ := http.NewRequest(http.MethodGet, "/api/domains?domain_regex=^example", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	if capturedFilter.DomainRegex != "^example" {
		t.Errorf("Expected domain_regex=^example, got %s", capturedFilter.DomainRegex)
	}
}

func TestGetDomains_DatabaseError(t *testing.T) {
	router, mockDB := setupTestRouter()

	mockDB.GetDomainsFunc = func(filter models.DomainsFilter) ([]models.Domain, int64, error) {
		return nil, 0, errors.New("database error")
	}

	h := NewHandler(mockDB)
	router.GET("/api/domains", h.GetDomains)

	req, _ := http.NewRequest(http.MethodGet, "/api/domains", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 500, got %d", w.Code)
	}
}

func TestExportList_Success(t *testing.T) {
	router, mockDB := setupTestRouter()

	mockDB.GetExportListFunc = func(domainRegex string) (*models.ExportList, error) {
		return &models.ExportList{
			Domains: []string{"example.com.", "test.com."}, // Domains with trailing dots (FQDN format from DB)
			IPv4:    []string{"192.0.2.1", "192.0.2.2"},
			IPv6:    []string{"2001:db8::1", "2001:db8::2"},
		}, nil
	}

	h := NewHandler(mockDB)
	router.GET("/export/test", func(c *gin.Context) {
		h.ExportList(c, ".*", true)
	})

	req, _ := http.NewRequest(http.MethodGet, "/export/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	contentType := w.Header().Get("Content-Type")
	if contentType != "text/plain; charset=utf-8" {
		t.Errorf("Expected Content-Type 'text/plain; charset=utf-8', got '%s'", contentType)
	}

	// Expected output should NOT have trailing dots
	expected := "example.com\ntest.com\n192.0.2.1\n192.0.2.2\n2001:db8::1\n2001:db8::2\n"
	if w.Body.String() != expected {
		t.Errorf("Expected body:\n%s\nGot:\n%s", expected, w.Body.String())
	}
}

func TestExportList_IPsOnly(t *testing.T) {
	router, mockDB := setupTestRouter()

	mockDB.GetExportListFunc = func(domainRegex string) (*models.ExportList, error) {
		return &models.ExportList{
			Domains: []string{"example.com", "test.com"},
			IPv4:    []string{"192.0.2.1"},
			IPv6:    []string{"2001:db8::1"},
		}, nil
	}

	h := NewHandler(mockDB)
	router.GET("/export/ips", func(c *gin.Context) {
		h.ExportList(c, ".*", false) // include_domains = false
	})

	req, _ := http.NewRequest(http.MethodGet, "/export/ips", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Domains should NOT be included
	expected := "192.0.2.1\n2001:db8::1\n"
	if w.Body.String() != expected {
		t.Errorf("Expected body:\n%s\nGot:\n%s", expected, w.Body.String())
	}
}

func TestExportList_EmptyResults(t *testing.T) {
	router, mockDB := setupTestRouter()

	mockDB.GetExportListFunc = func(domainRegex string) (*models.ExportList, error) {
		return &models.ExportList{
			Domains: []string{},
			IPv4:    []string{},
			IPv6:    []string{},
		}, nil
	}

	h := NewHandler(mockDB)
	router.GET("/export/empty", func(c *gin.Context) {
		h.ExportList(c, "^nomatch$", true)
	})

	req, _ := http.NewRequest(http.MethodGet, "/export/empty", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	if w.Body.String() != "" {
		t.Errorf("Expected empty body, got: %s", w.Body.String())
	}
}

func TestExportList_DatabaseError(t *testing.T) {
	router, mockDB := setupTestRouter()

	mockDB.GetExportListFunc = func(domainRegex string) (*models.ExportList, error) {
		return nil, errors.New("database connection failed")
	}

	h := NewHandler(mockDB)
	router.GET("/export/error", func(c *gin.Context) {
		h.ExportList(c, ".*", true)
	})

	req, _ := http.NewRequest(http.MethodGet, "/export/error", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 500, got %d", w.Code)
	}
}

func TestExportList_OnlyIPv4(t *testing.T) {
	router, mockDB := setupTestRouter()

	mockDB.GetExportListFunc = func(domainRegex string) (*models.ExportList, error) {
		return &models.ExportList{
			Domains: []string{},
			IPv4:    []string{"192.0.2.1", "192.0.2.2"},
			IPv6:    []string{},
		}, nil
	}

	h := NewHandler(mockDB)
	router.GET("/export/ipv4", func(c *gin.Context) {
		h.ExportList(c, ".*", false)
	})

	req, _ := http.NewRequest(http.MethodGet, "/export/ipv4", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	expected := "192.0.2.1\n192.0.2.2\n"
	if w.Body.String() != expected {
		t.Errorf("Expected body:\n%s\nGot:\n%s", expected, w.Body.String())
	}
}

func TestExportList_RemoveTrailingDot(t *testing.T) {
	router, mockDB := setupTestRouter()

	mockDB.GetExportListFunc = func(domainRegex string) (*models.ExportList, error) {
		return &models.ExportList{
			// Domains in FQDN format with trailing dots (as stored in DB)
			Domains: []string{
				"gecko16-normal-c-useast1a.tiktokv.com.",
				"gecko16-platform-ycru.tiktokv.com.",
				"gecko31-normal-useast1a.tiktokv.com.",
			},
			IPv4: []string{},
			IPv6: []string{},
		}, nil
	}

	h := NewHandler(mockDB)
	router.GET("/export/trailing", func(c *gin.Context) {
		h.ExportList(c, ".*", true)
	})

	req, _ := http.NewRequest(http.MethodGet, "/export/trailing", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Expected output without trailing dots
	expected := "gecko16-normal-c-useast1a.tiktokv.com\n" +
		"gecko16-platform-ycru.tiktokv.com\n" +
		"gecko31-normal-useast1a.tiktokv.com\n"

	if w.Body.String() != expected {
		t.Errorf("Expected body:\n%s\nGot:\n%s", expected, w.Body.String())
	}

	// Explicitly verify no trailing dots
	lines := strings.Split(strings.TrimSpace(w.Body.String()), "\n")
	for i, line := range lines {
		if strings.HasSuffix(line, ".") {
			t.Errorf("Line %d still has trailing dot: %s", i+1, line)
		}
	}
}
