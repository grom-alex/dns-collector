package database

import "dns-collector-webapi/internal/models"

// DB defines the interface for database operations
type DB interface {
	GetStats(filter models.StatsFilter) ([]models.DomainStat, int64, error)
	GetDomains(filter models.DomainsFilter) ([]models.Domain, int64, error)
	GetDomainWithIPs(id int64) (*models.Domain, error)
	GetExportList(domainRegex string) (*models.ExportList, error)
	Close() error
}
