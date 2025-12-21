package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"dns-collector/internal/config"
	"dns-collector/internal/database"
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
	defer db.Close()

	log.Println("Database connected successfully")

	// Run database migrations
	log.Println("Running database migrations...")
	if err := db.RunMigrations(); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}
	log.Println("Migrations completed successfully")

	// Create and start UDP server
	udpServer := server.NewUDPServer(cfg, db)
	if err := udpServer.Start(); err != nil {
		log.Fatalf("Failed to start UDP server: %v", err)
	}
	defer udpServer.Stop()

	// Create and start DNS resolver
	dnsResolver := resolver.NewResolver(cfg, db)
	dnsResolver.Start()
	defer dnsResolver.Stop()

	log.Println("DNS Collector is running. Press Ctrl+C to stop.")

	// Wait for interrupt signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	<-sigCh

	log.Println("\nShutting down gracefully...")
}
