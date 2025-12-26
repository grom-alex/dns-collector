package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"dns-collector/internal/cleanup"
	"dns-collector/internal/config"
	"dns-collector/internal/database"
	"dns-collector/internal/metrics"
	"dns-collector/internal/resolver"
	"dns-collector/internal/server"
)

func main() {
	configPath := flag.String("config", "config/config.yaml", "Path to configuration file")
	flag.Parse()

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	log.Printf("Starting DNS Collector...")
	log.Printf("Configuration loaded from: %s", *configPath)

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

	// Initialize metrics registry
	var metricsRegistry *metrics.Registry
	if cfg.Metrics.Enabled {
		metricsRegistry = metrics.NewRegistry()

		// Start metrics HTTP server
		metricsServer := metrics.NewServer(cfg.Metrics, metricsRegistry)
		if err := metricsServer.Start(); err != nil {
			log.Fatalf("Failed to start metrics server: %v", err)
		}
		defer func() {
			if err := metricsServer.Stop(); err != nil {
				log.Printf("Error stopping metrics server: %v", err)
			}
		}()

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

		log.Printf("Metrics enabled on port %d", cfg.Metrics.Port)
	}

	// Create and start UDP server
	udpServer := server.NewUDPServer(cfg, db, metricsRegistry)
	if err := udpServer.Start(); err != nil {
		log.Fatalf("Failed to start UDP server: %v", err)
	}
	defer udpServer.Stop()

	// Create and start DNS resolver
	dnsResolver := resolver.NewResolver(cfg, db, metricsRegistry)
	dnsResolver.Start()
	defer dnsResolver.Stop()

	// Create and start cleanup service
	cleanupService := cleanup.NewService(cfg, db, metricsRegistry)
	cleanupService.Start()
	defer cleanupService.Stop()

	log.Println("DNS Collector is running. Press Ctrl+C to stop.")

	// Wait for interrupt signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	<-sigCh

	log.Println("\nShutting down gracefully...")
}
