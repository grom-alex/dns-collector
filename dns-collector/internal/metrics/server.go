package metrics

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"dns-collector/internal/config"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Server provides an HTTP endpoint for Prometheus metrics scraping.
type Server struct {
	cfg      config.MetricsConfig
	registry *Registry
	server   *http.Server
}

// NewServer creates a new metrics HTTP server.
func NewServer(cfg config.MetricsConfig, registry *Registry) *Server {
	return &Server{
		cfg:      cfg,
		registry: registry,
	}
}

// Start starts the metrics HTTP server.
func (s *Server) Start() error {
	mux := http.NewServeMux()
	mux.Handle(s.cfg.Path, promhttp.HandlerFor(
		s.registry.GetRegistry(),
		promhttp.HandlerOpts{
			EnableOpenMetrics: true,
		},
	))

	// Add a simple health check endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("ok")); err != nil {
			log.Printf("Error writing health response: %v", err)
		}
	})

	addr := fmt.Sprintf(":%d", s.cfg.Port)
	s.server = &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		log.Printf("Metrics server listening on %s%s", addr, s.cfg.Path)
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("Metrics server error: %v", err)
		}
	}()

	return nil
}

// Stop gracefully stops the metrics HTTP server.
func (s *Server) Stop() error {
	if s.server == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	log.Println("Stopping metrics server...")
	if err := s.server.Shutdown(ctx); err != nil {
		return fmt.Errorf("metrics server shutdown error: %w", err)
	}

	log.Println("Metrics server stopped")
	return nil
}
