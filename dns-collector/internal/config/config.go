package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Database DatabaseConfig `yaml:"database"`
	Resolver ResolverConfig `yaml:"resolver"`
	Logging  LoggingConfig  `yaml:"logging"`
	WebAPI   WebAPIConfig   `yaml:"webapi"`
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
	IntervalSeconds int `yaml:"interval_seconds"`
	MaxResolv       int `yaml:"max_resolv"`
	TimeoutSeconds  int `yaml:"timeout_seconds"`
	Workers         int `yaml:"workers"`
}

type LoggingConfig struct {
	Level string `yaml:"level"`
}

type WebAPIConfig struct {
	Port int `yaml:"port"`
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

	return &cfg, nil
}
