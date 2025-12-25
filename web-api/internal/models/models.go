package models

import "time"

// DomainStat represents a DNS query statistic
type DomainStat struct {
	ID        int64     `json:"id"`
	Domain    string    `json:"domain"`
	ClientIP  string    `json:"client_ip"`
	RType     string    `json:"rtype"`
	Timestamp time.Time `json:"timestamp"`
}

// Domain represents a domain with its resolution info
type Domain struct {
	ID             int64     `json:"id"`
	Domain         string    `json:"domain"`
	TimeInsert     time.Time `json:"time_insert"`
	ResolvCount    int       `json:"resolv_count"`
	MaxResolv      int       `json:"max_resolv"`
	LastResolvTime time.Time `json:"last_resolv_time"`
	IPs            []IP      `json:"ips,omitempty"`
}

// IP represents an IP address associated with a domain
type IP struct {
	ID       int64     `json:"id"`
	DomainID int64     `json:"domain_id"`
	IP       string    `json:"ip"`
	Type     string    `json:"type"`
	Time     time.Time `json:"time"`
}

// StatsFilter represents filters for stats queries
type StatsFilter struct {
	ClientIPs []string  `json:"client_ips"`
	Subnet    string    `json:"subnet"`
	DateFrom  time.Time `json:"date_from"`
	DateTo    time.Time `json:"date_to"`
	SortBy    string    `json:"sort_by"`
	SortOrder string    `json:"sort_order"` // asc or desc
	Limit     int       `json:"limit"`
	Offset    int       `json:"offset"`
}

// DomainsFilter represents filters for domains queries
type DomainsFilter struct {
	DomainRegex string    `json:"domain_regex"`
	DateFrom    time.Time `json:"date_from"`
	DateTo      time.Time `json:"date_to"`
	SortBy      string    `json:"sort_by"`
	SortOrder   string    `json:"sort_order"` // asc or desc
	Limit       int       `json:"limit"`
	Offset      int       `json:"offset"`
}

// PaginatedResponse represents a paginated API response
type PaginatedResponse struct {
	Data       interface{} `json:"data"`
	Total      int64       `json:"total"`
	Limit      int         `json:"limit"`
	Offset     int         `json:"offset"`
	TotalPages int         `json:"total_pages"`
}

// ExportList represents data for plain text export
type ExportList struct {
	Domains []string
	IPv4    []string
	IPv6    []string
}

// ExcludedIPInfo contains information about IP address excluded from export
type ExcludedIPInfo struct {
	IP                string   `json:"ip"`                  // IP address
	MatchedDomains    []string `json:"matched_domains"`     // Domains matching the regex
	NonMatchedDomains []string `json:"non_matched_domains"` // Domains NOT matching the regex
}
