package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server    ServerConfig    `yaml:"server"`
	Database  DatabaseConfig  `yaml:"database"`
	Resolver  ResolverConfig  `yaml:"resolver"`
	Logging   LoggingConfig   `yaml:"logging"`
	WebAPI    WebAPIConfig    `yaml:"webapi"`
	Retention RetentionConfig `yaml:"retention"`
	Metrics   MetricsConfig   `yaml:"metrics"`
}

type ServerConfig struct {
	UDPPort int `yaml:"udp_port"`
}

type DatabaseConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	Database string `yaml:"database"`
	SSLMode  string `yaml:"ssl_mode"`
}

type ResolverConfig struct {
	IntervalSeconds    int  `yaml:"interval_seconds"`
	MaxResolv          int  `yaml:"max_resolv"`
	TimeoutSeconds     int  `yaml:"timeout_seconds"`
	Workers            int  `yaml:"workers"`
	CyclicResolv       bool `yaml:"cyclic_resolv"`        // Enable cyclic resolution (reset after max_resolv)
	ResolvCooldownMins int  `yaml:"resolv_cooldown_mins"` // Cooldown between cycles in minutes
}

type LoggingConfig struct {
	Level string `yaml:"level"`
}

type WebAPIConfig struct {
	Port int `yaml:"port"`
}

type RetentionConfig struct {
	StatsDays            int `yaml:"stats_days"`
	CleanupIntervalHours int `yaml:"cleanup_interval_hours"`
	IPTTLDays            int `yaml:"ip_ttl_days"` // TTL for IP addresses in days
}

type MetricsConfig struct {
	Enabled  bool           `yaml:"enabled"`
	Port     int            `yaml:"port"`
	Path     string         `yaml:"path"`
	InfluxDB InfluxDBConfig `yaml:"influxdb"`
}

type InfluxDBConfig struct {
	Enabled            bool   `yaml:"enabled"`
	URL                string `yaml:"url"`
	Token              string `yaml:"token"`
	Organization       string `yaml:"organization"`
	Bucket             string `yaml:"bucket"`
	IntervalSeconds    int    `yaml:"interval_seconds"`
	InsecureSkipVerify bool   `yaml:"insecure_skip_verify"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Override with environment variables if set
	if envPassword := os.Getenv("POSTGRES_PASSWORD"); envPassword != "" {
		cfg.Database.Password = envPassword
	}
	if envSSLMode := os.Getenv("POSTGRES_SSL_MODE"); envSSLMode != "" {
		cfg.Database.SSLMode = envSSLMode
	}

	// Validate configuration
	if cfg.Server.UDPPort <= 0 || cfg.Server.UDPPort > 65535 {
		return nil, fmt.Errorf("invalid UDP port: %d", cfg.Server.UDPPort)
	}
	if cfg.Resolver.IntervalSeconds <= 0 {
		return nil, fmt.Errorf("invalid resolver interval: %d", cfg.Resolver.IntervalSeconds)
	}
	if cfg.Resolver.MaxResolv <= 0 {
		return nil, fmt.Errorf("invalid max_resolv: %d", cfg.Resolver.MaxResolv)
	}
	if cfg.Resolver.Workers <= 0 {
		cfg.Resolver.Workers = 1
	}
	if cfg.WebAPI.Port <= 0 || cfg.WebAPI.Port > 65535 {
		cfg.WebAPI.Port = 8080 // default port
	}

	// Validate retention period: must be between 1 and 365 days
	if cfg.Retention.StatsDays <= 0 {
		cfg.Retention.StatsDays = 30 // default 30 days (1 month)
	}
	if cfg.Retention.StatsDays > 365 {
		return nil, fmt.Errorf("retention stats_days must not exceed 365 days, got %d", cfg.Retention.StatsDays)
	}

	// Validate cleanup interval: must be between 1 and 168 hours (1 week)
	if cfg.Retention.CleanupIntervalHours <= 0 {
		cfg.Retention.CleanupIntervalHours = 24 // default 24 hours (once per day)
	}
	if cfg.Retention.CleanupIntervalHours > 168 {
		return nil, fmt.Errorf("retention cleanup_interval_hours must not exceed 168 hours (1 week), got %d", cfg.Retention.CleanupIntervalHours)
	}

	// Validate IP TTL: 0 means disabled, otherwise must be between 1 and 90 days
	if cfg.Retention.IPTTLDays < 0 {
		cfg.Retention.IPTTLDays = 0 // disabled
	}
	if cfg.Retention.IPTTLDays == 0 {
		cfg.Retention.IPTTLDays = 3 // default 3 days
	}
	if cfg.Retention.IPTTLDays > 90 {
		return nil, fmt.Errorf("retention ip_ttl_days must not exceed 90 days, got %d", cfg.Retention.IPTTLDays)
	}

	// Validate cyclic resolv cooldown
	if cfg.Resolver.CyclicResolv && cfg.Resolver.ResolvCooldownMins <= 0 {
		cfg.Resolver.ResolvCooldownMins = 240 // default 4 hours
	}

	// Set defaults for metrics configuration
	if cfg.Metrics.Port <= 0 || cfg.Metrics.Port > 65535 {
		cfg.Metrics.Port = 9090 // default metrics port
	}
	if cfg.Metrics.Path == "" {
		cfg.Metrics.Path = "/metrics"
	}
	if cfg.Metrics.InfluxDB.IntervalSeconds <= 0 {
		cfg.Metrics.InfluxDB.IntervalSeconds = 10 // default push interval
	}

	// Override InfluxDB token from environment variable if set
	if envToken := os.Getenv("INFLUXDB_TOKEN"); envToken != "" {
		cfg.Metrics.InfluxDB.Token = envToken
	}

	return &cfg, nil
}
