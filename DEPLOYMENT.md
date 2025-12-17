# Deployment Guide

## Quick Links

- **Development**: See [QUICKSTART_WEBAPI.md](QUICKSTART_WEBAPI.md)
- **Production**: See [deploy/production/README.md](deploy/production/README.md)

## Version 2.0.0

Latest release with PostgreSQL backend.

### Docker Images

```bash
# DNS Collector
registry.gromas.ru/apps/dns-collector/dns-collector:2.0.0
registry.gromas.ru/apps/dns-collector/dns-collector:latest

# Web API
registry.gromas.ru/apps/dns-collector/web-api:2.0.0
registry.gromas.ru/apps/dns-collector/web-api:latest
```

## Development Deployment

Quick start for local development:

```bash
# Clone repository
git clone git@github.com:grom-alex/dns-collector.git
cd dns-collector

# Start all services
docker-compose up -d

# Access web interface
open http://localhost:8080
```

## Production Deployment

For production deployment with security best practices:

```bash
# Navigate to production deployment
cd deploy/production

# Configure environment
cp .env.example .env
nano .env

# Update configs with production settings
nano config/dns-collector.yaml
nano config/web-api.yaml

# Deploy
docker-compose up -d
```

**Important**: See [deploy/production/README.md](deploy/production/README.md) for:
- Security checklist
- Configuration details
- Backup procedures
- Monitoring setup
- Troubleshooting

## Architecture

### Components

1. **PostgreSQL** - Database backend (port 5432)
2. **dns-collector** - DNS query collection service (UDP 5353)
3. **web-api** - REST API and web interface (HTTP 8080)

### Data Flow

```
DNS Clients → dns-collector → PostgreSQL ← web-api ← Web Browser
```

## Migration from v1.x

**Breaking change**: v2.0 uses PostgreSQL instead of SQLite.

There is **no automatic migration**. This requires a fresh deployment.

Steps:
1. Backup SQLite data if needed (for reference)
2. Deploy v2.0 with PostgreSQL
3. Configure new system
4. Manually migrate critical data if required

## Configuration Structure

```
service/
├── config/
│   ├── config.yaml        # Production config
│   └── config.dev.yaml    # Development config
├── cmd/
├── internal/
└── Dockerfile
```

## Environment Variables

Key variables for production:

```bash
POSTGRES_USER=dns_collector
POSTGRES_PASSWORD=your_strong_password
POSTGRES_DB=dns_collector
TZ=UTC
```

## Health Checks

```bash
# PostgreSQL
docker exec dns-collector-postgres pg_isready -U dns_collector

# Web API
curl http://localhost:8080/health

# DNS Collector (send test request)
echo '{"domain": "test.com", "client_ip": "127.0.0.1"}' | nc -u -w1 localhost 5353
```

## Monitoring

### Logs

```bash
# All services
docker-compose logs -f

# Specific service
docker-compose logs -f dns-collector
docker-compose logs -f web-api
docker-compose logs -f postgres
```

### Metrics

Web interface provides:
- DNS query statistics
- Domain resolution status
- Client IP filtering
- Date range filtering

## Backup

### Quick Backup

```bash
# Database
docker exec dns-collector-postgres pg_dump -U dns_collector dns_collector > backup.sql

# Restore
cat backup.sql | docker exec -i dns-collector-postgres psql -U dns_collector dns_collector
```

See production README for detailed backup procedures.

## Support

- **Documentation**: [README.md](README.md)
- **Issues**: https://github.com/grom-alex/dns-collector/issues
- **Production Guide**: [deploy/production/README.md](deploy/production/README.md)

## Changelog

See [CHANGELOG.md](CHANGELOG.md) for version history.
