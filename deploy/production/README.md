# DNS Collector Production Deployment

Production-ready configuration for DNS Collector system.

## Version

**v2.0.1** - Environment variable configuration support

## Docker Images

- `registry.gromas.ru/apps/dns-collector/dns-collector:2.0.1`
- `registry.gromas.ru/apps/dns-collector/web-api:2.0.1`

## Quick Start

### 1. Prerequisites

- Docker Engine 20.10+
- Docker Compose 2.0+
- At least 2GB RAM
- 10GB free disk space

### 2. Choose Data Storage Method

**Option A: Docker Volume (Recommended)**
- Managed by Docker
- Easier backup/restore with Docker commands
- Portable across systems
- No manual permission management
- Default behavior (no configuration needed)

**Option B: Custom Host Path**
- Direct access to data on host filesystem
- Easier integration with host backup tools
- Better control over data location
- Useful for NFS/network storage
- Requires setting `POSTGRES_DATA_PATH` in `.env`

**Recommendation**: Use Docker Volume unless you have specific requirements for host path access.

### 3. Configuration

```bash
# Copy environment template
cp .env.example .env

# Edit .env file with your settings
nano .env
```

**Note**: Starting from v2.0.1, database password and SSL mode are configured via environment variables in `.env` file. You no longer need to edit the YAML config files for these settings.

### 4. Important Security Settings

**MUST CHANGE before deployment:**

1. **Database Password** in `.env`:
   ```
   POSTGRES_PASSWORD=YourStrongPasswordHere
   ```

2. **PostgreSQL SSL Mode** in `.env`:
   ```
   # Use "disable" if PostgreSQL doesn't have SSL configured
   POSTGRES_SSL_MODE=disable

   # Use "require" for production with SSL enabled
   POSTGRES_SSL_MODE=require
   ```

3. **Data Storage Path** (optional) in `.env`:
   ```
   # Leave empty to use Docker volume (recommended)
   POSTGRES_DATA_PATH=

   # Or specify custom path on host
   POSTGRES_DATA_PATH=/var/lib/dns-collector/postgres
   ```

   **Note**: If using custom path, ensure directory exists and has proper permissions:
   ```bash
   sudo mkdir -p /var/lib/dns-collector/postgres
   sudo chown -R 999:999 /var/lib/dns-collector/postgres
   ```

4. **CORS Origins** in `config/web-api.yaml`:
   ```yaml
   cors:
     allowed_origins:
       - "https://your-actual-domain.com"
   ```

### 5. Deploy

```bash
# Start all services
docker-compose up -d

# Check status
docker-compose ps

# View logs
docker-compose logs -f

# Stop services
docker-compose down
```

### 6. Health Checks

```bash
# PostgreSQL
docker exec dns-collector-postgres pg_isready -U dns_collector

# Web API
curl http://localhost:8080/health

# DNS Collector (send test request)
echo '{"client_ip": "10.0.0.1", "domain": "google.com", "qtype": "A", "rtype": "dns"}' | nc -u -w1 localhost 5353
```

## Architecture

```
┌─────────────────┐
│   DNS Clients   │
└────────┬────────┘
         │ UDP 5353
         ▼
┌─────────────────┐     ┌──────────────┐
│ dns-collector   │────▶│  PostgreSQL  │
└─────────────────┘     └──────┬───────┘
                               │
┌─────────────────┐            │
│    web-api      │────────────┘
└────────┬────────┘
         │ HTTP 8080
         ▼
┌─────────────────┐
│   Web Browser   │
└─────────────────┘
```

## Resource Limits

### PostgreSQL
- CPU: 0.5-2.0 cores
- Memory: 256M-1G

### dns-collector
- CPU: 0.5-2.0 cores
- Memory: 128M-512M

### web-api
- CPU: 0.5-2.0 cores
- Memory: 256M-512M

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

Access web interface at: `http://your-server:8080`

## Backup

### Database Backup

```bash
# Create backup
docker exec dns-collector-postgres pg_dump -U dns_collector dns_collector > backup_$(date +%Y%m%d_%H%M%S).sql

# Restore backup
cat backup_file.sql | docker exec -i dns-collector-postgres psql -U dns_collector dns_collector
```

### Volume Backup

**For Docker Volume (default):**

```bash
# Stop services
docker-compose down

# Backup postgres volume
docker run --rm -v dns-collector_postgres_data:/data -v $(pwd):/backup alpine tar czf /backup/postgres_data_backup.tar.gz /data

# Start services
docker-compose up -d
```

**For Custom Path (POSTGRES_DATA_PATH set):**

```bash
# Stop services
docker-compose down

# Backup data directory
sudo tar czf postgres_data_backup_$(date +%Y%m%d_%H%M%S).tar.gz -C /var/lib/dns-collector postgres

# Start services
docker-compose up -d

# Restore from backup
sudo tar xzf postgres_data_backup.tar.gz -C /var/lib/dns-collector
sudo chown -R 999:999 /var/lib/dns-collector/postgres
```

## Troubleshooting

### PostgreSQL connection issues

```bash
# Check PostgreSQL is running
docker-compose ps postgres

# Check PostgreSQL logs
docker-compose logs postgres

# Test connection
docker exec dns-collector-postgres psql -U dns_collector -d dns_collector -c "SELECT 1;"
```

### DNS Collector not receiving requests

```bash
# Check UDP port
sudo netstat -ulnp | grep 5353

# Check service logs
docker-compose logs dns-collector

# Test UDP connectivity
echo '{"domain": "test.com"}' | nc -u -w1 localhost 5353
```

### Web API not accessible

```bash
# Check service status
docker-compose ps web-api

# Check logs
docker-compose logs web-api

# Test health endpoint
curl http://localhost:8080/health
```

## Upgrade

```bash
# Pull new images
docker-compose pull

# Recreate containers
docker-compose up -d

# Check status
docker-compose ps
```

## Security Recommendations

1. **Use strong passwords** for database
2. **Enable SSL/TLS** for PostgreSQL connections
3. **Configure firewall** to restrict access:
   - UDP 5353: Only from trusted DNS clients
   - TCP 8080: Only from authorized users/networks
   - TCP 5432: Only from application containers (use internal network)
4. **Regular backups** of database
5. **Monitor logs** for suspicious activity
6. **Keep images updated** with security patches
7. **Use secrets management** for sensitive data
8. **Enable authentication** for web interface if exposing to internet

## Production Checklist

- [ ] Changed default passwords
- [ ] Updated CORS origins
- [ ] Enabled SSL for database
- [ ] Configured firewall rules
- [ ] Set up backup schedule
- [ ] Configured log rotation
- [ ] Set appropriate timezone
- [ ] Tested health checks
- [ ] Verified DNS resolution
- [ ] Tested web interface
- [ ] Set up monitoring alerts
- [ ] Documented custom configurations

## Support

For issues and questions:
- GitHub: https://github.com/grom-alex/dns-collector
- Documentation: See main README.md
