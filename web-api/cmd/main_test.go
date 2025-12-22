package main

import (
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
