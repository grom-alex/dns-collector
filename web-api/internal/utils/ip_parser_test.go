package utils

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseIPsFromFile(t *testing.T) {
	// Create temporary directory for test files
	tmpDir := t.TempDir()

	t.Run("Valid IPs mixed IPv4 and IPv6", func(t *testing.T) {
		content := `# This is a comment
192.168.1.1
10.0.0.1

# Another comment
2001:db8::1
fe80::1
172.16.0.1
`
		tmpFile := filepath.Join(tmpDir, "valid_ips.txt")
		if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		ipv4List, ipv6List, err := ParseIPsFromFile(tmpFile)
		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}

		expectedIPv4 := []string{"192.168.1.1", "10.0.0.1", "172.16.0.1"}
		expectedIPv6 := []string{"2001:db8::1", "fe80::1"}

		if len(ipv4List) != len(expectedIPv4) {
			t.Errorf("Expected %d IPv4 addresses, got %d", len(expectedIPv4), len(ipv4List))
		}
		if len(ipv6List) != len(expectedIPv6) {
			t.Errorf("Expected %d IPv6 addresses, got %d", len(expectedIPv6), len(ipv6List))
		}

		for i, ip := range expectedIPv4 {
			if ipv4List[i] != ip {
				t.Errorf("Expected IPv4[%d] = %s, got %s", i, ip, ipv4List[i])
			}
		}

		for i, ip := range expectedIPv6 {
			if ipv6List[i] != ip {
				t.Errorf("Expected IPv6[%d] = %s, got %s", i, ip, ipv6List[i])
			}
		}
	})

	t.Run("Empty file", func(t *testing.T) {
		tmpFile := filepath.Join(tmpDir, "empty.txt")
		if err := os.WriteFile(tmpFile, []byte(""), 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		ipv4List, ipv6List, err := ParseIPsFromFile(tmpFile)
		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}

		if len(ipv4List) != 0 {
			t.Errorf("Expected 0 IPv4 addresses, got %d", len(ipv4List))
		}
		if len(ipv6List) != 0 {
			t.Errorf("Expected 0 IPv6 addresses, got %d", len(ipv6List))
		}
	})

	t.Run("Only comments and empty lines", func(t *testing.T) {
		content := `# Comment 1

# Comment 2

`
		tmpFile := filepath.Join(tmpDir, "comments_only.txt")
		if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		ipv4List, ipv6List, err := ParseIPsFromFile(tmpFile)
		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}

		if len(ipv4List) != 0 {
			t.Errorf("Expected 0 IPv4 addresses, got %d", len(ipv4List))
		}
		if len(ipv6List) != 0 {
			t.Errorf("Expected 0 IPv6 addresses, got %d", len(ipv6List))
		}
	})

	t.Run("File with invalid IPs", func(t *testing.T) {
		content := `192.168.1.1
invalid-ip
256.256.256.256
10.0.0.1
not-an-ip
`
		tmpFile := filepath.Join(tmpDir, "invalid_ips.txt")
		if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		ipv4List, _, err := ParseIPsFromFile(tmpFile)
		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}

		// Should skip invalid IPs and only return valid ones
		expectedIPv4 := []string{"192.168.1.1", "10.0.0.1"}
		if len(ipv4List) != len(expectedIPv4) {
			t.Errorf("Expected %d IPv4 addresses, got %d", len(expectedIPv4), len(ipv4List))
		}
	})

	t.Run("File does not exist", func(t *testing.T) {
		_, _, err := ParseIPsFromFile("/nonexistent/file.txt")
		if err == nil {
			t.Error("Expected error for non-existent file, got nil")
		}
	})
}

func TestValidateIPAddress(t *testing.T) {
	tests := []struct {
		ip    string
		valid bool
	}{
		{"192.168.1.1", true},
		{"10.0.0.1", true},
		{"172.16.0.1", true},
		{"2001:db8::1", true},
		{"fe80::1", true},
		{"::1", true},
		{"invalid", false},
		{"256.256.256.256", false},
		{"192.168.1", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.ip, func(t *testing.T) {
			result := ValidateIPAddress(tt.ip)
			if result != tt.valid {
				t.Errorf("ValidateIPAddress(%q) = %v, want %v", tt.ip, result, tt.valid)
			}
		})
	}
}

func TestIsIPv4(t *testing.T) {
	tests := []struct {
		ip   string
		want bool
	}{
		{"192.168.1.1", true},
		{"10.0.0.1", true},
		{"2001:db8::1", false},
		{"fe80::1", false},
		{"invalid", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.ip, func(t *testing.T) {
			result := IsIPv4(tt.ip)
			if result != tt.want {
				t.Errorf("IsIPv4(%q) = %v, want %v", tt.ip, result, tt.want)
			}
		})
	}
}

func TestIsIPv6(t *testing.T) {
	tests := []struct {
		ip   string
		want bool
	}{
		{"192.168.1.1", false},
		{"10.0.0.1", false},
		{"2001:db8::1", true},
		{"fe80::1", true},
		{"::1", true},
		{"invalid", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.ip, func(t *testing.T) {
			result := IsIPv6(tt.ip)
			if result != tt.want {
				t.Errorf("IsIPv6(%q) = %v, want %v", tt.ip, result, tt.want)
			}
		})
	}
}
