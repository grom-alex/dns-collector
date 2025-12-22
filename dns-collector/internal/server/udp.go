package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net"

	"dns-collector/internal/config"
	"dns-collector/internal/database"
)

type DNSQuery struct {
	ClientIP string `json:"client_ip"`
	Domain   string `json:"domain"`
	QType    string `json:"qtype"`
	RType    string `json:"rtype"`
}

type UDPServer struct {
	cfg    *config.Config
	db     *database.Database
	conn   *net.UDPConn
	stopCh chan struct{}
}

func NewUDPServer(cfg *config.Config, db *database.Database) *UDPServer {
	return &UDPServer{
		cfg:    cfg,
		db:     db,
		stopCh: make(chan struct{}),
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

			// Process message in a separate goroutine
			go s.handleMessage(buffer[:n])
		}
	}
}

func (s *UDPServer) handleMessage(data []byte) {
	var query DNSQuery
	if err := json.Unmarshal(data, &query); err != nil {
		log.Printf("Error parsing JSON: %v, raw message: %q", err, string(data))
		return
	}

	// Validate required fields
	if query.Domain == "" {
		log.Printf("Empty domain in query")
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
	if _, err := s.db.InsertOrGetDomain(query.Domain, s.cfg.Resolver.MaxResolv); err != nil {
		log.Printf("Error inserting domain: %v", err)
	}
}

func (s *UDPServer) Stop() {
	close(s.stopCh)
	if s.conn != nil {
		_ = s.conn.Close()
	}
	log.Println("UDP server stopped")
}
