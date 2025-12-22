package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateExportLists_Success(t *testing.T) {
	lists := []ExportListConfig{
		{
			Name:           "Test List",
			Endpoint:       "/export/test",
			DomainRegex:    ".*\\.com$",
			IncludeDomains: true,
		},
	}

	err := validateExportLists(lists)
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
}

func TestValidateExportLists_MissingName(t *testing.T) {
	lists := []ExportListConfig{
		{
			Name:           "",
			Endpoint:       "/export/test",
			DomainRegex:    ".*",
			IncludeDomains: true,
		},
	}

	err := validateExportLists(lists)
	if err == nil {
		t.Error("Expected error for missing name, got nil")
	}
	expectedMsg := "name is required"
	if err != nil && !contains(err.Error(), expectedMsg) {
		t.Errorf("Expected error containing '%s', got: %v", expectedMsg, err)
	}
}

func TestValidateExportLists_MissingEndpoint(t *testing.T) {
	lists := []ExportListConfig{
		{
			Name:           "Test List",
			Endpoint:       "",
			DomainRegex:    ".*",
			IncludeDomains: true,
		},
	}

	err := validateExportLists(lists)
	if err == nil {
		t.Error("Expected error for missing endpoint, got nil")
	}
	expectedMsg := "endpoint is required"
	if err != nil && !contains(err.Error(), expectedMsg) {
		t.Errorf("Expected error containing '%s', got: %v", expectedMsg, err)
	}
}

func TestValidateExportLists_MissingDomainRegex(t *testing.T) {
	lists := []ExportListConfig{
		{
			Name:           "Test List",
			Endpoint:       "/export/test",
			DomainRegex:    "",
			IncludeDomains: true,
		},
	}

	err := validateExportLists(lists)
	if err == nil {
		t.Error("Expected error for missing domain_regex, got nil")
	}
	expectedMsg := "domain_regex is required"
	if err != nil && !contains(err.Error(), expectedMsg) {
		t.Errorf("Expected error containing '%s', got: %v", expectedMsg, err)
	}
}

func TestValidateExportLists_DuplicateName(t *testing.T) {
	lists := []ExportListConfig{
		{
			Name:           "Test List",
			Endpoint:       "/export/test1",
			DomainRegex:    ".*",
			IncludeDomains: true,
		},
		{
			Name:           "Test List",
			Endpoint:       "/export/test2",
			DomainRegex:    ".*",
			IncludeDomains: false,
		},
	}

	err := validateExportLists(lists)
	if err == nil {
		t.Error("Expected error for duplicate name, got nil")
	}
	expectedMsg := "duplicate name"
	if err != nil && !contains(err.Error(), expectedMsg) {
		t.Errorf("Expected error containing '%s', got: %v", expectedMsg, err)
	}
}

func TestValidateExportLists_DuplicateEndpoint(t *testing.T) {
	lists := []ExportListConfig{
		{
			Name:           "Test List 1",
			Endpoint:       "/export/test",
			DomainRegex:    ".*",
			IncludeDomains: true,
		},
		{
			Name:           "Test List 2",
			Endpoint:       "/export/test",
			DomainRegex:    ".*",
			IncludeDomains: false,
		},
	}

	err := validateExportLists(lists)
	if err == nil {
		t.Error("Expected error for duplicate endpoint, got nil")
	}
	expectedMsg := "duplicate endpoint"
	if err != nil && !contains(err.Error(), expectedMsg) {
		t.Errorf("Expected error containing '%s', got: %v", expectedMsg, err)
	}
}

func TestValidateExportLists_InvalidEndpoint(t *testing.T) {
	lists := []ExportListConfig{
		{
			Name:           "Test List",
			Endpoint:       "export/test",
			DomainRegex:    ".*",
			IncludeDomains: true,
		},
	}

	err := validateExportLists(lists)
	if err == nil {
		t.Error("Expected error for endpoint not starting with /, got nil")
	}
	expectedMsg := "endpoint must start with /"
	if err != nil && !contains(err.Error(), expectedMsg) {
		t.Errorf("Expected error containing '%s', got: %v", expectedMsg, err)
	}
}

func TestValidateExportLists_EmptyList(t *testing.T) {
	lists := []ExportListConfig{}

	err := validateExportLists(lists)
	if err != nil {
		t.Errorf("Expected no error for empty list, got: %v", err)
	}
}

func TestValidateExportLists_MultipleLists(t *testing.T) {
	lists := []ExportListConfig{
		{
			Name:           "List 1",
			Endpoint:       "/export/list1",
			DomainRegex:    ".*\\.com$",
			IncludeDomains: true,
		},
		{
			Name:           "List 2",
			Endpoint:       "/export/list2",
			DomainRegex:    ".*\\.org$",
			IncludeDomains: false,
		},
		{
			Name:           "List 3",
			Endpoint:       "/export/list3",
			DomainRegex:    "^example\\..*$",
			IncludeDomains: true,
		},
	}

	err := validateExportLists(lists)
	if err != nil {
		t.Errorf("Expected no error for valid multiple lists, got: %v", err)
	}
}

func TestLoadConfig_Success(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")

	configContent := `server:
  port: 9090
  host: "127.0.0.1"
database:
  host: "localhost"
  port: 5432
  user: "testuser"
  password: "testpass"
  database: "testdb"
  ssl_mode: "disable"
logging:
  level: "debug"
cors:
  allowed_origins:
    - "http://test.com"
  allow_credentials: true
export_lists:
  - name: "Test List"
    endpoint: "/export/test"
    domain_regex: ".*\\.test$"
    include_domains: true
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to create temp config: %v", err)
	}

	cfg, err := loadConfig(configPath)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if cfg.Server.Port != 9090 {
		t.Errorf("Expected port 9090, got %d", cfg.Server.Port)
	}
	if cfg.Server.Host != "127.0.0.1" {
		t.Errorf("Expected host 127.0.0.1, got %s", cfg.Server.Host)
	}
	if len(cfg.ExportLists) != 1 {
		t.Errorf("Expected 1 export list, got %d", len(cfg.ExportLists))
	}
}

func TestLoadConfig_Defaults(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")

	configContent := `database:
  host: "localhost"
  port: 5432
  user: "testuser"
  password: "testpass"
  database: "testdb"
  ssl_mode: "disable"
logging:
  level: "info"
cors:
  allowed_origins:
    - "http://test.com"
  allow_credentials: true
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to create temp config: %v", err)
	}

	cfg, err := loadConfig(configPath)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if cfg.Server.Port != 8080 {
		t.Errorf("Expected default port 8080, got %d", cfg.Server.Port)
	}
	if cfg.Server.Host != "0.0.0.0" {
		t.Errorf("Expected default host 0.0.0.0, got %s", cfg.Server.Host)
	}
}

func TestLoadConfig_FileNotFound(t *testing.T) {
	_, err := loadConfig("/nonexistent/config.yaml")
	if err == nil {
		t.Error("Expected error for nonexistent file, got nil")
	}
	if !contains(err.Error(), "failed to read config") {
		t.Errorf("Expected 'failed to read config' error, got: %v", err)
	}
}

func TestLoadConfig_InvalidYAML(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")

	invalidYAML := "server:\n  port: \"not valid yaml\n  invalid"
	if err := os.WriteFile(configPath, []byte(invalidYAML), 0644); err != nil {
		t.Fatalf("Failed to create temp config: %v", err)
	}

	_, err := loadConfig(configPath)
	if err == nil {
		t.Error("Expected error for invalid YAML, got nil")
	}
	if !contains(err.Error(), "failed to parse config") {
		t.Errorf("Expected 'failed to parse config' error, got: %v", err)
	}
}

func TestLoadConfig_ValidationError(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")

	configContent := `server:
  port: 8080
database:
  host: "localhost"
  port: 5432
  user: "testuser"
  password: "testpass"
  database: "testdb"
  ssl_mode: "disable"
logging:
  level: "info"
cors:
  allowed_origins:
    - "http://test.com"
  allow_credentials: true
export_lists:
  - name: "Test"
    endpoint: "invalid"
    domain_regex: ".*"
    include_domains: true
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to create temp config: %v", err)
	}

	_, err := loadConfig(configPath)
	if err == nil {
		t.Error("Expected validation error, got nil")
	}
	if !contains(err.Error(), "invalid export lists configuration") {
		t.Errorf("Expected validation error, got: %v", err)
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
