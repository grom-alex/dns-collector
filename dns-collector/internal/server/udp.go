package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"dns-collector/internal/config"
	"dns-collector/internal/metrics"
	"dns-collector/internal/writer"
)

type DNSQuery struct {
	ClientIP string `json:"client_ip"`
	Domain   string `json:"domain"`
	QType    string `json:"qtype"`
	RType    string `json:"rtype"`
}

type UDPServer struct {
	cfg        *config.Config
	writer     *writer.Writer
	metrics    *metrics.Registry
	conn       *net.UDPConn
	stopCh     chan struct{}
	jobs       chan []byte
	listenDone chan struct{}
	wg         sync.WaitGroup
}

func NewUDPServer(cfg *config.Config, w *writer.Writer, m *metrics.Registry) *UDPServer {
	return &UDPServer{
		cfg:     cfg,
		writer:  w,
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
	s.jobs = make(chan []byte, s.cfg.Server.UDPQueueSize)
	s.listenDone = make(chan struct{})

	for i := 0; i < s.cfg.Server.UDPWorkers; i++ {
		s.wg.Add(1)
		go s.worker()
	}
	go s.listen()

	log.Printf("UDP server listening on port %d (%d workers, queue %d)",
		s.cfg.Server.UDPPort, s.cfg.Server.UDPWorkers, s.cfg.Server.UDPQueueSize)
	return nil
}

func (s *UDPServer) listen() {
	defer close(s.listenDone)

	buffer := make([]byte, 4096)

	for {
		select {
		case <-s.stopCh:
			return
		default:
		}

		n, _, err := s.conn.ReadFromUDP(buffer)
		if err != nil {
			select {
			case <-s.stopCh:
				return
			default:
			}
			log.Printf("Error reading from UDP: %v", err)
			continue
		}

		// The read buffer is reused; hand a private copy to a worker.
		data := make([]byte, n)
		copy(data, buffer[:n])

		select {
		case s.jobs <- data:
		default:
			// Queue is full: shed load instead of spawning unbounded goroutines.
			s.recordMetric(func(m *metrics.Registry) {
				m.ServerMessagesDropped.WithLabelValues("udp_queue_full").Inc()
			})
		}
	}
}

func (s *UDPServer) worker() {
	defer s.wg.Done()
	for data := range s.jobs {
		s.handleMessage(data)
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

	if s.writer == nil {
		log.Printf("UDP writer is not initialized; dropping query for %s", query.Domain)
		s.recordMetric(func(m *metrics.Registry) {
			m.ServerMessagesDropped.WithLabelValues("writer_unavailable").Inc()
		})
		return
	}
	s.writer.Submit(query.Domain, query.ClientIP, query.RType)

	s.recordMetric(func(m *metrics.Registry) {
		m.ServerMessagesReceived.WithLabelValues("valid").Inc()
		m.ServerDomainsReceived.WithLabelValues(query.RType).Inc()
		m.ServerProcessingTime.Observe(time.Since(start).Seconds())
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
	if s.listenDone != nil {
		<-s.listenDone
	}
	if s.jobs != nil {
		close(s.jobs)
		s.wg.Wait()
	}
	log.Println("UDP server stopped")
}
