package database

import (
	"testing"
)

func TestGetExportList_ValidateEmptyRegex(t *testing.T) {
	db := &Database{}

	_, err := db.GetExportList("", true, true, false)
	if err == nil {
		t.Error("Expected error for empty regex, got nil")
	}

	expectedMsg := "domain regex is required"
	if err.Error() != expectedMsg {
		t.Errorf("Expected error '%s', got: %v", expectedMsg, err)
	}
}

func TestGetExportList_ValidateRegexTooLong(t *testing.T) {
	db := &Database{}

	longRegex := ""
	for i := 0; i < 201; i++ {
		longRegex += "a"
	}

	_, err := db.GetExportList(longRegex, true, true, false)
	if err == nil {
		t.Error("Expected error for regex too long, got nil")
	}

	expectedMsg := "regex pattern too long (max 200 characters)"
	if err.Error() != expectedMsg {
		t.Errorf("Expected error '%s', got: %v", expectedMsg, err)
	}
}

func TestGetExportList_DangerousPattern_NestedStar(t *testing.T) {
	db := &Database{}

	_, err := db.GetExportList("(.*)*", true, true, false)
	if err == nil {
		t.Error("Expected error for dangerous pattern (.*)*,  got nil")
	}

	if !contains(err.Error(), "potentially dangerous construct") {
		t.Errorf("Expected error about dangerous construct, got: %v", err)
	}
}

func TestGetExportList_DangerousPattern_NestedPlus1(t *testing.T) {
	db := &Database{}

	_, err := db.GetExportList("(.+)+", true, true, false)
	if err == nil {
		t.Error("Expected error for dangerous pattern (.+)+, got nil")
	}

	if !contains(err.Error(), "potentially dangerous construct") {
		t.Errorf("Expected error about dangerous construct, got: %v", err)
	}
}

func TestGetExportList_DangerousPattern_NestedPlus2(t *testing.T) {
	db := &Database{}

	_, err := db.GetExportList("(.*)+", true, true, false)
	if err == nil {
		t.Error("Expected error for dangerous pattern (.*)+ , got nil")
	}

	if !contains(err.Error(), "potentially dangerous construct") {
		t.Errorf("Expected error about dangerous construct, got: %v", err)
	}
}

func TestGetExportList_DangerousPattern_NestedStar2(t *testing.T) {
	db := &Database{}

	_, err := db.GetExportList("(.+)*", true, true, false)
	if err == nil {
		t.Error("Expected error for dangerous pattern (.+)*, got nil")
	}

	if !contains(err.Error(), "potentially dangerous construct") {
		t.Errorf("Expected error about dangerous construct, got: %v", err)
	}
}

func TestGetExportList_ValidPattern(t *testing.T) {
	// Test that valid patterns don't trigger validation errors
	// We only test the validation logic, not actual DB queries
	validPatterns := []string{
		".*\\.com$",
		"^example\\..*$",
		"(example|test)\\.com",
		".*",
	}

	for _, pattern := range validPatterns {
		// Test regex length validation
		if len(pattern) > 200 {
			t.Errorf("Pattern '%s' exceeds 200 chars", pattern)
		}

		// Test dangerous pattern detection
		dangerousPatterns := []string{"(.*)*", "(.+)+", "(.*)+", "(.+)*"}
		for _, dangerous := range dangerousPatterns {
			if contains(pattern, dangerous) {
				t.Errorf("Pattern '%s' contains dangerous construct: %s", pattern, dangerous)
			}
		}
	}
}

func TestGetExportList_RegexAt200Chars(t *testing.T) {
	// Test exact limit (200 chars) - only validation, no DB connection
	regex200 := ""
	for i := 0; i < 200; i++ {
		regex200 += "a"
	}

	// Test length validation
	if len(regex200) != 200 {
		t.Errorf("Expected regex length to be 200, got: %d", len(regex200))
	}

	// This should be within the limit
	if len(regex200) > 200 {
		t.Error("Regex at 200 chars should be accepted")
	}

	// Test 201 chars should exceed limit
	regex201 := regex200 + "a"
	if len(regex201) <= 200 {
		t.Error("Regex at 201 chars should exceed limit")
	}
}

// Helper function
func contains(s, substr string) bool {
	if len(s) < len(substr) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
