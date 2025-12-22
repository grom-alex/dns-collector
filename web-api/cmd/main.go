package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"gopkg.in/yaml.v3"

	"dns-collector-webapi/internal/database"
	"dns-collector-webapi/internal/handlers"
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
	ExportLists []ExportListConfig `yaml:"export_lists"`
}

type ExportListConfig struct {
	Name           string `yaml:"name"`
	Endpoint       string `yaml:"endpoint"`
	DomainRegex    string `yaml:"domain_regex"`
	IncludeDomains bool   `yaml:"include_domains"`
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

		// Validate endpoint starts with /
		if list.Endpoint[0] != '/' {
			return fmt.Errorf("export list '%s': endpoint must start with /", list.Name)
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

	// Health check
	router.GET("/health", h.HealthCheck)

	// API routes
	api := router.Group("/api")
	{
		api.GET("/stats", h.GetStats)
		api.GET("/domains", h.GetDomains)
		api.GET("/domains/:id", h.GetDomainByID)
	}

	// Register export list endpoints
	for _, exportList := range cfg.ExportLists {
		listConfig := exportList
		router.GET(listConfig.Endpoint, func(c *gin.Context) {
			h.ExportList(c, listConfig.DomainRegex, listConfig.IncludeDomains)
		})
		log.Printf("Registered export list '%s' at %s", listConfig.Name, listConfig.Endpoint)
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
