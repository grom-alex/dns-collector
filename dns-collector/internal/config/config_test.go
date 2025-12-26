package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_ValidConfig(t *testing.T) {
	// Create temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `
server:
  udp_port: 5353

database:
  host: "localhost"
  port: 5432
  user: "test_user"
  password: "test_pass"
  database: "test_db"
  ssl_mode: "disable"

resolver:
  interval_seconds: 10
  max_resolv: 5
  timeout_seconds: 5
  workers: 3

logging:
  level: "info"

webapi:
  port: 8080
`

	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to create test config: %v", err)
	}

	// Load config
	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	// Validate loaded values
	if cfg.Server.UDPPort != 5353 {
		t.Errorf("Expected UDPPort=5353, got %d", cfg.Server.UDPPort)
	}
	if cfg.Database.Host != "localhost" {
		t.Errorf("Expected Host=localhost, got %s", cfg.Database.Host)
	}
	if cfg.Database.Port != 5432 {
		t.Errorf("Expected Port=5432, got %d", cfg.Database.Port)
	}
	if cfg.Resolver.IntervalSeconds != 10 {
		t.Errorf("Expected IntervalSeconds=10, got %d", cfg.Resolver.IntervalSeconds)
	}
	if cfg.Resolver.Workers != 3 {
		t.Errorf("Expected Workers=3, got %d", cfg.Resolver.Workers)
	}
}

func TestLoad_EnvVariableOverride(t *testing.T) {
	// Set environment variables
	_ = os.Setenv("POSTGRES_PASSWORD", "env_password")
	_ = os.Setenv("POSTGRES_SSL_MODE", "require")
	defer func() {
		_ = os.Unsetenv("POSTGRES_PASSWORD")
		_ = os.Unsetenv("POSTGRES_SSL_MODE")
	}()

	// Create temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `
server:
  udp_port: 5353
database:
  host: "localhost"
  port: 5432
  user: "test_user"
  password: "file_password"
  database: "test_db"
  ssl_mode: "disable"
resolver:
  interval_seconds: 10
  max_resolv: 5
  timeout_seconds: 5
  workers: 1
logging:
  level: "info"
webapi:
  port: 8080
`

	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to create test config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	// Verify env variables override file values
	if cfg.Database.Password != "env_password" {
		t.Errorf("Expected Password=env_password, got %s", cfg.Database.Password)
	}
	if cfg.Database.SSLMode != "require" {
		t.Errorf("Expected SSLMode=require, got %s", cfg.Database.SSLMode)
	}
}

func TestLoad_InvalidUDPPort(t *testing.T) {
	tests := []struct {
		name string
		port string
	}{
		{"negative port", "-1"},
		{"zero port", "0"},
		{"port too large", "70000"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			configPath := filepath.Join(tmpDir, "config.yaml")

			configContent := `server:
  udp_port: ` + tt.port + `
database:
  host: "localhost"
  port: 5432
  user: "test"
  password: "test"
  database: "test"
  ssl_mode: "disable"
resolver:
  interval_seconds: 10
  max_resolv: 5
  timeout_seconds: 5
  workers: 1
logging:
  level: "info"
`

			if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
				t.Fatalf("Failed to create test config: %v", err)
			}

			_, err := Load(configPath)
			if err == nil {
				t.Error("Expected error for invalid UDP port, got nil")
			}
		})
	}
}

func TestLoad_InvalidResolverConfig(t *testing.T) {
	tests := []struct {
		name             string
		intervalSeconds  string
		maxResolv        string
		expectError      bool
	}{
		{"negative interval", "-1", "5", true},
		{"zero interval", "0", "5", true},
		{"negative max_resolv", "10", "-1", true},
		{"zero max_resolv", "10", "0", true},
		{"valid config", "10", "5", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			configPath := filepath.Join(tmpDir, "config.yaml")

			configContent := `server:
  udp_port: 5353
database:
  host: "localhost"
  port: 5432
  user: "test"
  password: "test"
  database: "test"
  ssl_mode: "disable"
resolver:
  interval_seconds: ` + tt.intervalSeconds + `
  max_resolv: ` + tt.maxResolv + `
  timeout_seconds: 5
  workers: 1
logging:
  level: "info"
`

			if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
				t.Fatalf("Failed to create test config: %v", err)
			}

			_, err := Load(configPath)
			if tt.expectError && err == nil {
				t.Error("Expected error but got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
		})
	}
}

func TestLoad_DefaultWorkers(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `
server:
  udp_port: 5353
database:
  host: "localhost"
  port: 5432
  user: "test"
  password: "test"
  database: "test"
  ssl_mode: "disable"
resolver:
  interval_seconds: 10
  max_resolv: 5
  timeout_seconds: 5
  workers: 0
logging:
  level: "info"
`

	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to create test config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	// Workers should default to 1 if set to 0 or negative
	if cfg.Resolver.Workers != 1 {
		t.Errorf("Expected Workers=1 (default), got %d", cfg.Resolver.Workers)
	}
}

func TestLoad_DefaultWebAPIPort(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `
server:
  udp_port: 5353
database:
  host: "localhost"
  port: 5432
  user: "test"
  password: "test"
  database: "test"
  ssl_mode: "disable"
resolver:
  interval_seconds: 10
  max_resolv: 5
  timeout_seconds: 5
  workers: 1
logging:
  level: "info"
webapi:
  port: 0
`

	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to create test config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	// WebAPI port should default to 8080 if invalid
	if cfg.WebAPI.Port != 8080 {
		t.Errorf("Expected WebAPI.Port=8080 (default), got %d", cfg.WebAPI.Port)
	}
}

func TestLoad_DefaultRetentionDays(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `server:
  udp_port: 5353
database:
  host: "localhost"
  port: 5432
  user: "test"
  password: "test"
  database: "test"
  ssl_mode: "disable"
resolver:
  interval_seconds: 10
  max_resolv: 5
  timeout_seconds: 5
logging:
  level: "info"
`

	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to create test config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Check default retention period is 30 days
	if cfg.Retention.StatsDays != 30 {
		t.Errorf("Expected default Retention.StatsDays=30, got %d", cfg.Retention.StatsDays)
	}
}

func TestLoad_RetentionConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `server:
  udp_port: 5353
database:
  host: "localhost"
  port: 5432
  user: "test"
  password: "test"
  database: "test"
  ssl_mode: "disable"
resolver:
  interval_seconds: 10
  max_resolv: 5
  timeout_seconds: 5
  workers: 2
logging:
  level: "info"
webapi:
  port: 8080
retention:
  stats_days: 90
`

	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to create test config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if cfg.Retention.StatsDays != 90 {
		t.Errorf("Expected Retention.StatsDays=90, got %d", cfg.Retention.StatsDays)
	}
}

func TestLoad_RetentionValidation(t *testing.T) {
	tests := []struct {
		name        string
		statsDays   string
		expectError bool
	}{
		{"valid 30 days", "30", false},
		{"valid 365 days", "365", false},
		{"invalid exceeds max", "400", true},
		{"valid 1 day", "1", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			configPath := filepath.Join(tmpDir, "config.yaml")

			configContent := `server:
  udp_port: 5353
database:
  host: "localhost"
  port: 5432
  user: "test"
  password: "test"
  database: "test"
  ssl_mode: "disable"
resolver:
  interval_seconds: 10
  max_resolv: 5
  timeout_seconds: 5
logging:
  level: "info"
retention:
  stats_days: ` + tt.statsDays + `
`

			if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
				t.Fatalf("Failed to create test config: %v", err)
			}

			_, err := Load(configPath)
			if tt.expectError && err == nil {
				t.Error("Expected error but got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
		})
	}
}

func TestLoad_CleanupIntervalValidation(t *testing.T) {
	tests := []struct {
		name          string
		intervalHours string
		expectError   bool
		expectedValue int
	}{
		{"valid 24 hours", "24", false, 24},
		{"valid 12 hours", "12", false, 12},
		{"invalid exceeds max", "200", true, 0},
		{"default on zero", "0", false, 24},
		{"default on negative", "-5", false, 24},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			configPath := filepath.Join(tmpDir, "config.yaml")

			configContent := `server:
  udp_port: 5353
database:
  host: "localhost"
  port: 5432
  user: "test"
  password: "test"
  database: "test"
  ssl_mode: "disable"
resolver:
  interval_seconds: 10
  max_resolv: 5
  timeout_seconds: 5
logging:
  level: "info"
retention:
  stats_days: 30
  cleanup_interval_hours: ` + tt.intervalHours + `
`

			if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
				t.Fatalf("Failed to create test config: %v", err)
			}

			cfg, err := Load(configPath)
			if tt.expectError && err == nil {
				t.Error("Expected error but got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
			if !tt.expectError && cfg.Retention.CleanupIntervalHours != tt.expectedValue {
				t.Errorf("Expected CleanupIntervalHours=%d, got %d", tt.expectedValue, cfg.Retention.CleanupIntervalHours)
			}
		})
	}
}

func TestLoad_FileNotFound(t *testing.T) {
	_, err := Load("/nonexistent/path/config.yaml")
	if err == nil {
		t.Error("Expected error for non-existent file, got nil")
	}
}

func TestLoad_InvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	invalidContent := `
server:
  udp_port: 5353
  invalid yaml structure {{{
`

	if err := os.WriteFile(configPath, []byte(invalidContent), 0644); err != nil {
		t.Fatalf("Failed to create test config: %v", err)
	}

	_, err := Load(configPath)
	if err == nil {
		t.Error("Expected error for invalid YAML, got nil")
	}
}

func TestLoad_DomainTTLValidation(t *testing.T) {
	tests := []struct {
		name          string
		domainTTLDays string
		expectError   bool
		expectedValue int
	}{
		{"valid 30 days", "30", false, 30},
		{"valid 365 days", "365", false, 365},
		{"invalid exceeds max", "400", true, 0},
		{"valid 1 day", "1", false, 1},
		{"disabled on zero", "0", false, 0},
		{"disabled on negative", "-5", false, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			configPath := filepath.Join(tmpDir, "config.yaml")

			configContent := `server:
  udp_port: 5353
database:
  host: "localhost"
  port: 5432
  user: "test"
  password: "test"
  database: "test"
  ssl_mode: "disable"
resolver:
  interval_seconds: 10
  max_resolv: 5
  timeout_seconds: 5
logging:
  level: "info"
retention:
  stats_days: 30
  domain_ttl_days: ` + tt.domainTTLDays + `
`

			if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
				t.Fatalf("Failed to create test config: %v", err)
			}

			cfg, err := Load(configPath)
			if tt.expectError && err == nil {
				t.Error("Expected error but got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
			if !tt.expectError && cfg.Retention.DomainTTLDays != tt.expectedValue {
				t.Errorf("Expected DomainTTLDays=%d, got %d", tt.expectedValue, cfg.Retention.DomainTTLDays)
			}
		})
	}
}
