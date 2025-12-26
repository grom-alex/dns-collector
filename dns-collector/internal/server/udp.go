package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"time"

	"dns-collector/internal/config"
	"dns-collector/internal/database"
	"dns-collector/internal/metrics"
)

type DNSQuery struct {
	ClientIP string `json:"client_ip"`
	Domain   string `json:"domain"`
	QType    string `json:"qtype"`
	RType    string `json:"rtype"`
}

type UDPServer struct {
	cfg     *config.Config
	db      *database.Database
	metrics *metrics.Registry
	conn    *net.UDPConn
	stopCh  chan struct{}
}

func NewUDPServer(cfg *config.Config, db *database.Database, m *metrics.Registry) *UDPServer {
	return &UDPServer{
		cfg:     cfg,
		db:      db,
		metrics: m,
		stopCh:  make(chan struct{}),
	}
}

func (s *UDPServer) Start() error {
	addr := &net.UDPAddr{
		Port: s.cfg.Server.UDPPort,
		IP:   net.ParseIP("0.0.0.0"),
	}

	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return fmt.Errorf("failed to start UDP server: %w", err)
	}

	s.conn = conn
	log.Printf("UDP server listening on port %d", s.cfg.Server.UDPPort)

	go s.listen()

	return nil
}

func (s *UDPServer) listen() {
	buffer := make([]byte, 4096)

	for {
		select {
		case <-s.stopCh:
			return
		default:
			n, _, err := s.conn.ReadFromUDP(buffer)
			if err != nil {
				log.Printf("Error reading from UDP: %v", err)
				continue
			}

			// Create a copy of the data to avoid buffer reuse issues
			// This prevents "data changing underfoot" when processing in goroutine
			data := make([]byte, n)
			copy(data, buffer[:n])

			// Process message in a separate goroutine
			go s.handleMessage(data)
		}
	}
}

func (s *UDPServer) handleMessage(data []byte) {
	// Add panic recovery to prevent crashes from unexpected errors
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Recovered from panic in handleMessage: %v, raw message: %q", r, string(data))
			s.recordMetric(func(m *metrics.Registry) {
				m.ServerMessagesReceived.WithLabelValues("invalid").Inc()
			})
		}
	}()

	start := time.Now()

	// Trim any trailing null bytes or whitespace that might corrupt JSON
	data = trimInvalidJSONSuffix(data)

	var query DNSQuery
	if err := json.Unmarshal(data, &query); err != nil {
		log.Printf("Error parsing JSON: %v, raw message: %q", err, string(data))
		s.recordMetric(func(m *metrics.Registry) {
			m.ServerMessagesReceived.WithLabelValues("invalid").Inc()
		})
		return
	}

	// Validate required fields
	if query.Domain == "" {
		log.Printf("Empty domain in query")
		s.recordMetric(func(m *metrics.Registry) {
			m.ServerMessagesReceived.WithLabelValues("invalid").Inc()
		})
		return
	}
	if query.ClientIP == "" {
		query.ClientIP = "unknown"
	}
	if query.RType == "" {
		query.RType = "unknown"
	}

	log.Printf("Received DNS query: domain=%s, client=%s, rtype=%s", query.Domain, query.ClientIP, query.RType)

	// Insert statistics
	if err := s.db.InsertDomainStat(query.Domain, query.ClientIP, query.RType); err != nil {
		log.Printf("Error inserting domain stat: %v", err)
	}

	// Insert or get domain
	domain, isNew, err := s.db.InsertOrGetDomain(query.Domain, s.cfg.Resolver.MaxResolv)
	if err != nil {
		log.Printf("Error inserting domain: %v", err)
		return
	}

	// Update last_seen timestamp to track when domain was last queried
	if err := s.db.UpdateDomainLastSeen(domain.ID); err != nil {
		log.Printf("Error updating domain last_seen: %v", err)
	}

	// Record metrics
	s.recordMetric(func(m *metrics.Registry) {
		m.ServerMessagesReceived.WithLabelValues("valid").Inc()
		m.ServerDomainsReceived.WithLabelValues(query.RType).Inc()
		m.ServerProcessingTime.Observe(time.Since(start).Seconds())
		if isNew {
			m.ServerNewDomains.Inc()
		}
	})
}

// trimInvalidJSONSuffix removes trailing garbage that may corrupt JSON parsing
func trimInvalidJSONSuffix(data []byte) []byte {
	// Try to find the first valid JSON object by iterating and testing
	// Start from the end and work backwards to find the first position
	// where we have valid JSON
	for i := len(data); i > 0; i-- {
		if data[i-1] == '}' {
			// Try to parse as JSON
			var query DNSQuery
			if err := json.Unmarshal(data[:i], &query); err == nil {
				// Valid JSON found, trim everything after
				return data[:i]
			}
		}
	}

	// No valid JSON found, return as-is
	return data
}

// recordMetric safely records a metric if metrics are enabled.
func (s *UDPServer) recordMetric(f func(m *metrics.Registry)) {
	if s.metrics != nil {
		f(s.metrics)
	}
}

func (s *UDPServer) Stop() {
	close(s.stopCh)
	if s.conn != nil {
		_ = s.conn.Close()
	}
	log.Println("UDP server stopped")
}
