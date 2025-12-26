package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"gopkg.in/yaml.v3"

	"dns-collector-webapi/internal/database"
	"dns-collector-webapi/internal/handlers"
	"dns-collector-webapi/internal/metrics"
)

type Config struct {
	Server struct {
		Port int    `yaml:"port"`
		Host string `yaml:"host"`
	} `yaml:"server"`
	Database struct {
		Host     string `yaml:"host"`
		Port     int    `yaml:"port"`
		User     string `yaml:"user"`
		Password string `yaml:"password"`
		Database string `yaml:"database"`
		SSLMode  string `yaml:"ssl_mode"`
	} `yaml:"database"`
	Logging struct {
		Level string `yaml:"level"`
	} `yaml:"logging"`
	CORS struct {
		AllowedOrigins   []string `yaml:"allowed_origins"`
		AllowCredentials bool     `yaml:"allow_credentials"`
	} `yaml:"cors"`
	Metrics     MetricsConfig      `yaml:"metrics"`
	ExportLists []ExportListConfig `yaml:"export_lists"`
}

type MetricsConfig struct {
	Enabled  bool                   `yaml:"enabled"`
	Path     string                 `yaml:"path"`
	InfluxDB metrics.InfluxDBConfig `yaml:"influxdb"`
}

type ExportListConfig struct {
	Name                 string `yaml:"name"`
	Endpoint             string `yaml:"endpoint"`
	DomainRegex          string `yaml:"domain_regex"`
	IncludeDomains       bool   `yaml:"include_domains"`
	IncludeIPv4          *bool  `yaml:"include_ipv4,omitempty"`
	IncludeIPv6          *bool  `yaml:"include_ipv6,omitempty"`
	ExcludeSharedIPs     *bool  `yaml:"exclude_shared_ips,omitempty"`
	ExcludedIPsEndpoint  string `yaml:"excluded_ips_endpoint,omitempty"`
	AdditionalIPsFile    string `yaml:"additional_ips_file,omitempty"`
}

// GetIncludeIPv4 returns the value of IncludeIPv4 or default (true)
func (c *ExportListConfig) GetIncludeIPv4() bool {
	if c.IncludeIPv4 == nil {
		return true
	}
	return *c.IncludeIPv4
}

// GetIncludeIPv6 returns the value of IncludeIPv6 or default (true)
func (c *ExportListConfig) GetIncludeIPv6() bool {
	if c.IncludeIPv6 == nil {
		return true
	}
	return *c.IncludeIPv6
}

// GetExcludeSharedIPs returns the value of ExcludeSharedIPs or default (false)
func (c *ExportListConfig) GetExcludeSharedIPs() bool {
	if c.ExcludeSharedIPs == nil {
		return false
	}
	return *c.ExcludeSharedIPs
}

func loadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	// Set defaults
	if cfg.Server.Port == 0 {
		cfg.Server.Port = 8080
	}
	if cfg.Server.Host == "" {
		cfg.Server.Host = "0.0.0.0"
	}

	// Set defaults for metrics
	if cfg.Metrics.Path == "" {
		cfg.Metrics.Path = "/metrics"
	}
	if cfg.Metrics.InfluxDB.IntervalSeconds <= 0 {
		cfg.Metrics.InfluxDB.IntervalSeconds = 10
	}

	// Override InfluxDB token from environment variable if set
	if envToken := os.Getenv("INFLUXDB_TOKEN"); envToken != "" {
		cfg.Metrics.InfluxDB.Token = envToken
	}

	// Validate export lists configuration
	if err := validateExportLists(cfg.ExportLists); err != nil {
		return nil, fmt.Errorf("invalid export lists configuration: %w", err)
	}

	return &cfg, nil
}

func validateExportLists(lists []ExportListConfig) error {
	endpoints := make(map[string]bool)
	names := make(map[string]bool)

	for i, list := range lists {
		// Check required fields
		if list.Name == "" {
			return fmt.Errorf("export list at index %d: name is required", i)
		}
		if list.Endpoint == "" {
			return fmt.Errorf("export list '%s': endpoint is required", list.Name)
		}
		if list.DomainRegex == "" {
			return fmt.Errorf("export list '%s': domain_regex is required", list.Name)
		}

		// Check for duplicate names
		if names[list.Name] {
			return fmt.Errorf("export list '%s': duplicate name", list.Name)
		}
		names[list.Name] = true

		// Check for duplicate endpoints
		if endpoints[list.Endpoint] {
			return fmt.Errorf("export list '%s': duplicate endpoint '%s'", list.Name, list.Endpoint)
		}
		endpoints[list.Endpoint] = true

		// Validate endpoint starts with / (defensive check for length)
		if len(list.Endpoint) == 0 || list.Endpoint[0] != '/' {
			return fmt.Errorf("export list '%s': endpoint must start with /", list.Name)
		}

		// Validate at least one of include_domains, include_ipv4, include_ipv6 is true
		includeIPv4 := list.GetIncludeIPv4()
		includeIPv6 := list.GetIncludeIPv6()

		if !includeIPv4 && !includeIPv6 && !list.IncludeDomains {
			return fmt.Errorf("export list '%s': at least one of include_domains, include_ipv4, or include_ipv6 must be true", list.Name)
		}

		// Warn if exclude_shared_ips is enabled but no IP types are enabled
		if list.GetExcludeSharedIPs() && !includeIPv4 && !includeIPv6 {
			log.Printf("Warning: export list '%s' has exclude_shared_ips=true but no IP types enabled", list.Name)
		}

		// Validate excluded_ips_endpoint
		if list.ExcludedIPsEndpoint != "" {
			// Must start with /
			if !strings.HasPrefix(list.ExcludedIPsEndpoint, "/") {
				return fmt.Errorf("export list '%s': excluded_ips_endpoint must start with '/'", list.Name)
			}

			// Check for duplicate endpoints (with both main and excluded endpoints)
			if endpoints[list.ExcludedIPsEndpoint] {
				return fmt.Errorf("export list '%s': excluded_ips_endpoint '%s' conflicts with another endpoint", list.Name, list.ExcludedIPsEndpoint)
			}
			endpoints[list.ExcludedIPsEndpoint] = true

			// Warn if excluded endpoint is set but exclude_shared_ips is false
			if !list.GetExcludeSharedIPs() {
				log.Printf("Warning: export list '%s' has excluded_ips_endpoint but exclude_shared_ips is false", list.Name)
			}
		}

		// Validate additional_ips_file
		if list.AdditionalIPsFile != "" {
			// Must be absolute path
			if !filepath.IsAbs(list.AdditionalIPsFile) {
				return fmt.Errorf("export list '%s': additional_ips_file must be an absolute path", list.Name)
			}

			// Must be within allowed config directory
			// Note: We'll check AllowedConfigDir constant when we create the utils package
			allowedConfigDir := "/app/config"
			if !strings.HasPrefix(list.AdditionalIPsFile, allowedConfigDir) {
				return fmt.Errorf("export list '%s': additional_ips_file must be within %s", list.Name, allowedConfigDir)
			}

			// Check if file exists (warning only, not error)
			if _, err := os.Stat(list.AdditionalIPsFile); os.IsNotExist(err) {
				log.Printf("Warning: export list '%s' references non-existent additional_ips_file: %s", list.Name, list.AdditionalIPsFile)
			}
		}
	}

	return nil
}

func main() {
	configPath := flag.String("config", "config/config.yaml", "Path to configuration file")
	flag.Parse()

	log.Println("Starting DNS Collector Web API...")

	// Load configuration
	cfg, err := loadConfig(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Override with environment variables if set
	if envPassword := os.Getenv("POSTGRES_PASSWORD"); envPassword != "" {
		cfg.Database.Password = envPassword
	}
	if envSSLMode := os.Getenv("POSTGRES_SSL_MODE"); envSSLMode != "" {
		cfg.Database.SSLMode = envSSLMode
	}

	// Set logging level
	if cfg.Logging.Level == "debug" {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	// Initialize database
	db, err := database.New(
		cfg.Database.Host,
		cfg.Database.Port,
		cfg.Database.User,
		cfg.Database.Password,
		cfg.Database.Database,
		cfg.Database.SSLMode,
	)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			log.Printf("Error closing database: %v", err)
		}
	}()

	log.Println("Database connected successfully")

	// Run database migrations
	log.Println("Running database migrations...")
	if err := db.RunMigrations(); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}
	log.Println("Migrations completed successfully")

	// Initialize handlers
	h := handlers.NewHandler(db)

	// Create Gin router
	router := gin.Default()

	// Configure CORS
	corsConfig := cors.Config{
		AllowOrigins:     cfg.CORS.AllowedOrigins,
		AllowMethods:     []string{"GET", "POST", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept"},
		AllowCredentials: cfg.CORS.AllowCredentials,
		MaxAge:           12 * time.Hour,
	}
	router.Use(cors.New(corsConfig))

	// Initialize metrics if enabled
	if cfg.Metrics.Enabled {
		metricsRegistry := metrics.NewRegistry()

		// Add metrics middleware
		router.Use(metrics.GinMiddleware(metricsRegistry))

		// Register /metrics endpoint
		router.GET(cfg.Metrics.Path, metrics.PrometheusHandler(metricsRegistry))

		// Start InfluxDB client if enabled
		if cfg.Metrics.InfluxDB.Enabled {
			influxClient := metrics.NewInfluxDBClient(cfg.Metrics.InfluxDB, metricsRegistry)
			if err := influxClient.Start(); err != nil {
				log.Printf("Warning: Failed to start InfluxDB client: %v", err)
			} else {
				defer func() {
					if err := influxClient.Stop(); err != nil {
						log.Printf("Error stopping InfluxDB client: %v", err)
					}
				}()
			}
		}

		log.Printf("Metrics enabled at %s", cfg.Metrics.Path)
	}

	// Health check
	router.GET("/health", h.HealthCheck)

	// API routes
	api := router.Group("/api")
	{
		api.GET("/stats", h.GetStats)
		api.GET("/stats/export", h.ExportStats)
		api.GET("/domains", h.GetDomains)
		api.GET("/domains/export", h.ExportDomains)
		api.GET("/domains/:id", h.GetDomainByID)
	}

	// Register export list endpoints
	for _, exportList := range cfg.ExportLists {
		// Capture loop variables to avoid closure issues
		domainRegex := exportList.DomainRegex
		includeDomains := exportList.IncludeDomains
		includeIPv4 := exportList.GetIncludeIPv4()
		includeIPv6 := exportList.GetIncludeIPv6()
		excludeSharedIPs := exportList.GetExcludeSharedIPs()
		additionalIPsFile := exportList.AdditionalIPsFile
		endpoint := exportList.Endpoint
		excludedEndpoint := exportList.ExcludedIPsEndpoint
		listName := exportList.Name

		// Register main export endpoint
		router.GET(endpoint, func(c *gin.Context) {
			h.ExportList(c,
				domainRegex,
				includeDomains,
				includeIPv4,
				includeIPv6,
				excludeSharedIPs,
				additionalIPsFile,
			)
		})
		log.Printf("Registered export list '%s' at %s", listName, endpoint)

		// Register excluded IPs endpoint if configured
		if excludedEndpoint != "" {
			router.GET(excludedEndpoint, func(c *gin.Context) {
				h.ExportExcludedIPs(c,
					domainRegex,
					includeIPv4,
					includeIPv6,
				)
			})
			log.Printf("Registered excluded IPs endpoint for '%s' at %s", listName, excludedEndpoint)
		}
	}

	// Serve static files for frontend
	router.Static("/assets", "./frontend/dist/assets")
	router.StaticFile("/", "./frontend/dist/index.html")
	router.NoRoute(func(c *gin.Context) {
		c.File("./frontend/dist/index.html")
	})

	// Create HTTP server
	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	srv := &http.Server{
		Addr:    addr,
		Handler: router,
	}

	// Start server in goroutine
	go func() {
		log.Printf("Web API server listening on %s", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("Server forced to shutdown: %v", err)
	}

	log.Println("Server stopped")
}
