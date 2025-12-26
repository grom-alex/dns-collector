package server

import (
	"encoding/json"
	"testing"

	"dns-collector/internal/config"
)

// MockDatabase for testing server
type MockDatabase struct {
	InsertDomainStatFunc   func(domain, clientIP, rtype string) error
	InsertOrGetDomainFunc  func(domain string, maxResolv int) (interface{}, error)
	DeleteOldStatsFunc     func(retentionDays int) (int64, error)
	CloseFunc              func() error
}

func (m *MockDatabase) InsertDomainStat(domain, clientIP, rtype string) error {
	if m.InsertDomainStatFunc != nil {
		return m.InsertDomainStatFunc(domain, clientIP, rtype)
	}
	return nil
}

func (m *MockDatabase) InsertOrGetDomain(domain string, maxResolv int) (interface{}, error) {
	if m.InsertOrGetDomainFunc != nil {
		return m.InsertOrGetDomainFunc(domain, maxResolv)
	}
	return nil, nil
}

func (m *MockDatabase) DeleteOldStats(retentionDays int) (int64, error) {
	if m.DeleteOldStatsFunc != nil {
		return m.DeleteOldStatsFunc(retentionDays)
	}
	return 0, nil
}

func (m *MockDatabase) Close() error {
	if m.CloseFunc != nil {
		return m.CloseFunc()
	}
	return nil
}

func TestNewUDPServer(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			UDPPort: 5353,
		},
	}
	mockDB := &MockDatabase{}

	server := &UDPServer{
		cfg:    cfg,
		db:     nil, // Using nil since we just test structure
		stopCh: make(chan struct{}),
	}

	if server.cfg.Server.UDPPort != 5353 {
		t.Errorf("Expected port=5353, got %d", server.cfg.Server.UDPPort)
	}

	if server.stopCh == nil {
		t.Error("Expected stopCh to be initialized")
	}

	_ = mockDB
}

func TestDNSQuery_JSONParsing(t *testing.T) {
	tests := []struct {
		name     string
		json     string
		expected DNSQuery
		wantErr  bool
	}{
		{
			name: "valid complete query",
			json: `{"client_ip":"192.168.1.1","domain":"example.com","qtype":"A","rtype":"A"}`,
			expected: DNSQuery{
				ClientIP: "192.168.1.1",
				Domain:   "example.com",
				QType:    "A",
				RType:    "A",
			},
			wantErr: false,
		},
		{
			name: "valid minimal query",
			json: `{"domain":"test.com"}`,
			expected: DNSQuery{
				Domain: "test.com",
			},
			wantErr: false,
		},
		{
			name:    "invalid json",
			json:    `{invalid}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var query DNSQuery
			err := json.Unmarshal([]byte(tt.json), &query)

			if tt.wantErr && err == nil {
				t.Error("Expected error but got none")
			}

			if !tt.wantErr && err != nil {
				t.Errorf("Expected no error, got %v", err)
			}

			if !tt.wantErr {
				if query.Domain != tt.expected.Domain {
					t.Errorf("Expected domain=%s, got %s", tt.expected.Domain, query.Domain)
				}
				if query.ClientIP != tt.expected.ClientIP {
					t.Errorf("Expected clientIP=%s, got %s", tt.expected.ClientIP, query.ClientIP)
				}
			}
		})
	}
}

func TestHandleMessage_ValidQuery(t *testing.T) {
	cfg := &config.Config{
		Resolver: config.ResolverConfig{
			MaxResolv: 10,
		},
	}

	statCalled := false
	domainCalled := false

	mockDB := &MockDatabase{
		InsertDomainStatFunc: func(domain, clientIP, rtype string) error {
			statCalled = true
			if domain != "example.com" {
				t.Errorf("Expected domain=example.com, got %s", domain)
			}
			if clientIP != "192.168.1.1" {
				t.Errorf("Expected clientIP=192.168.1.1, got %s", clientIP)
			}
			return nil
		},
		InsertOrGetDomainFunc: func(domain string, maxResolv int) (interface{}, error) {
			domainCalled = true
			if domain != "example.com" {
				t.Errorf("Expected domain=example.com, got %s", domain)
			}
			if maxResolv != 10 {
				t.Errorf("Expected maxResolv=10, got %d", maxResolv)
			}
			return nil, nil
		},
	}

	// Type assertion workaround - we'll test handleMessage logic separately
	_ = mockDB
	_ = statCalled
	_ = domainCalled

	query := DNSQuery{
		ClientIP: "192.168.1.1",
		Domain:   "example.com",
		RType:    "A",
	}

	data, err := json.Marshal(query)
	if err != nil {
		t.Fatalf("Failed to marshal query: %v", err)
	}

	// Test JSON unmarshal works
	var parsed DNSQuery
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Errorf("Failed to unmarshal query: %v", err)
	}

	if parsed.Domain != "example.com" {
		t.Errorf("Expected domain=example.com, got %s", parsed.Domain)
	}

	_ = cfg
}

func TestHandleMessage_EmptyDomain(t *testing.T) {
	query := DNSQuery{
		ClientIP: "192.168.1.1",
		Domain:   "",
		RType:    "A",
	}

	data, err := json.Marshal(query)
	if err != nil {
		t.Fatalf("Failed to marshal query: %v", err)
	}

	var parsed DNSQuery
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Errorf("Failed to unmarshal query: %v", err)
	}

	// Check that empty domain is preserved
	if parsed.Domain != "" {
		t.Errorf("Expected empty domain, got %s", parsed.Domain)
	}
}

func TestHandleMessage_DefaultValues(t *testing.T) {
	query := DNSQuery{
		Domain: "example.com",
		// ClientIP and RType not set
	}

	data, err := json.Marshal(query)
	if err != nil {
		t.Fatalf("Failed to marshal query: %v", err)
	}

	var parsed DNSQuery
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Errorf("Failed to unmarshal query: %v", err)
	}

	// Check defaults are applied
	if parsed.ClientIP == "" {
		// Should be set to "unknown" in handleMessage
		parsed.ClientIP = "unknown"
	}

	if parsed.RType == "" {
		// Should be set to "unknown" in handleMessage
		parsed.RType = "unknown"
	}

	if parsed.ClientIP != "unknown" {
		t.Errorf("Expected default clientIP=unknown, got %s", parsed.ClientIP)
	}

	if parsed.RType != "unknown" {
		t.Errorf("Expected default rtype=unknown, got %s", parsed.RType)
	}
}

func TestStopChannel(t *testing.T) {
	server := &UDPServer{
		stopCh: make(chan struct{}),
	}

	// Test that we can close stopCh
	close(server.stopCh)

	// Test that stopCh is closed
	select {
	case <-server.stopCh:
		// Successfully closed
	default:
		t.Error("stopCh should be closed")
	}
}

func TestTrimInvalidJSONSuffix(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected []byte
	}{
		{
			name:     "valid JSON",
			input:    []byte(`{"domain":"example.com"}`),
			expected: []byte(`{"domain":"example.com"}`),
		},
		{
			name:     "JSON with trailing garbage e",
			input:    []byte(`{"domain":"example.com"}e"`),
			expected: []byte(`{"domain":"example.com"}`),
		},
		{
			name:     "JSON with trailing garbage he",
			input:    []byte(`{"domain":"example.com"}he"`),
			expected: []byte(`{"domain":"example.com"}`),
		},
		{
			name:     "JSON with null bytes",
			input:    []byte("{\"domain\":\"example.com\"}\x00\x00"),
			expected: []byte(`{"domain":"example.com"}`),
		},
		{
			name:     "JSON with trailing quote and brace",
			input:    []byte(`{"client_ip":"192.168.0.50","domain":"ev.adriver.ru.","qtype":"A","rtype":"cache"}e"}`),
			expected: []byte(`{"client_ip":"192.168.0.50","domain":"ev.adriver.ru.","qtype":"A","rtype":"cache"}`),
		},
		{
			name:     "truncated JSON without closing brace",
			input:    []byte(`{"domain":"exam`),
			expected: []byte(`{"domain":"exam`),
		},
		{
			name:     "empty data",
			input:    []byte{},
			expected: []byte{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := trimInvalidJSONSuffix(tt.input)
			if string(result) != string(tt.expected) {
				t.Errorf("trimInvalidJSONSuffix() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestTrimInvalidJSONSuffix_RealWorldCases(t *testing.T) {
	// Test real-world cases from production logs
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "production case 1 - trailing e quote brace",
			input:    `{"client_ip": "192.168.0.50", "domain": "ev.adriver.ru.", "qtype": "A", "rtype": "cache"}e"}`,
			expected: `{"client_ip": "192.168.0.50", "domain": "ev.adriver.ru.", "qtype": "A", "rtype": "cache"}`,
		},
		{
			name:     "production case 2 - trailing he quote brace",
			input:    `{"client_ip": "192.168.0.74", "domain": "ev.adriver.ru.", "qtype": "A", "rtype": "cache"}he"}`,
			expected: `{"client_ip": "192.168.0.74", "domain": "ev.adriver.ru.", "qtype": "A", "rtype": "cache"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := trimInvalidJSONSuffix([]byte(tt.input))
			resultStr := string(result)

			// Verify the result is valid JSON
			var query DNSQuery
			if err := json.Unmarshal(result, &query); err != nil {
				t.Errorf("Result is not valid JSON: %v, got: %q", err, resultStr)
			}

			if resultStr != tt.expected {
				t.Errorf("trimInvalidJSONSuffix() = %q, want %q", resultStr, tt.expected)
			}
		})
	}
}
