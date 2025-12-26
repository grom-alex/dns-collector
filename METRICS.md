# DNS Collector Metrics

DNS Collector provides comprehensive metrics for monitoring via Prometheus and InfluxDB2.

## Endpoints

| Service | Endpoint | Port | Description |
|---------|----------|------|-------------|
| dns-collector | `/metrics` | 9090 | Prometheus metrics |
| web-api | `/metrics` | 8080 | Prometheus metrics (same port as API) |

## Configuration

### Prometheus (scraping)

Metrics are always available at `/metrics` when enabled.

```yaml
metrics:
  enabled: true
  port: 9090        # dns-collector only
  path: "/metrics"
```

### InfluxDB2 (push)

Optional push to InfluxDB2 for time-series storage.

```yaml
metrics:
  influxdb:
    enabled: true
    url: "https://influxdb:8086"
    token: ""  # Use INFLUXDB_TOKEN env var
    organization: "my-org"
    bucket: "dns-metrics"
    interval_seconds: 10
    insecure_skip_verify: false  # true for self-signed certs
```

Environment variable `INFLUXDB_TOKEN` overrides config file token.

---

## dns-collector Metrics

### Resolver Metrics

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `dns_resolver_domains_processed_total` | Counter | `status` | Total domains processed (success/error) |
| `dns_resolver_lookups_total` | Counter | `ip_version`, `status` | DNS lookups by IP version (ipv4/ipv6) and status |
| `dns_resolver_lookup_duration_seconds` | Histogram | `ip_version` | DNS lookup duration |
| `dns_resolver_batch_size` | Gauge | - | Current batch size being resolved |
| `dns_resolver_active_workers` | Gauge | - | Number of active resolver workers |

### UDP Server Metrics

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `dns_server_messages_received_total` | Counter | `status` | Messages received (valid/invalid) |
| `dns_server_domains_received_total` | Counter | `rtype` | Domains received by record type (cache/dns) |
| `dns_server_new_domains_total` | Counter | - | New unique domains registered |
| `dns_server_processing_duration_seconds` | Histogram | - | Message processing time |

### Cleanup Metrics

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `dns_cleanup_stats_deleted_total` | Counter | - | Old stats records deleted |
| `dns_cleanup_ips_deleted_total` | Counter | - | Expired IP addresses deleted |
| `dns_cleanup_duration_seconds` | Histogram | - | Cleanup operation duration |
| `dns_cleanup_runs_total` | Counter | - | Total cleanup runs |

### Database Metrics

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `dns_db_domains_total` | Gauge | - | Total domains in database |
| `dns_db_ips_total` | Gauge | - | Total IP addresses in database |
| `dns_db_connections_open` | Gauge | - | Open database connections |
| `dns_db_connections_idle` | Gauge | - | Idle database connections |
| `dns_db_queries_total` | Counter | `type`, `status` | Database queries by type and status |
| `dns_db_query_duration_seconds` | Histogram | `type` | Query duration by type |

---

## web-api Metrics

### HTTP Metrics

Automatically collected via Gin middleware.

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `http_requests_total` | Counter | `method`, `path`, `status` | Total HTTP requests |
| `http_request_duration_seconds` | Histogram | `method`, `path` | Request duration |
| `http_requests_in_flight` | Gauge | - | Currently processing requests |
| `http_response_size_bytes` | Histogram | `method`, `path` | Response body size |

### API Metrics

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `api_stats_queries_total` | Counter | - | Queries to `/api/stats` |
| `api_domains_queries_total` | Counter | - | Queries to `/api/domains` |

### Database Metrics

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `db_domains_total` | Gauge | - | Total domains in database |
| `db_ips_total` | Gauge | - | Total IP addresses in database |
| `db_connections_open` | Gauge | - | Open database connections |
| `db_connections_idle` | Gauge | - | Idle database connections |

---

## Go Runtime Metrics

Both services expose Go runtime metrics (automatically collected):

| Metric | Type | Description |
|--------|------|-------------|
| `go_goroutines` | Gauge | Number of goroutines |
| `go_threads` | Gauge | Number of OS threads |
| `go_gc_duration_seconds` | Summary | GC pause duration |
| `go_memstats_alloc_bytes` | Gauge | Allocated heap bytes |
| `go_memstats_heap_objects` | Gauge | Number of heap objects |
| `go_info` | Gauge | Go version info |
| `process_cpu_seconds_total` | Counter | CPU time consumed |
| `process_resident_memory_bytes` | Gauge | Resident memory size |
| `process_open_fds` | Gauge | Open file descriptors |

---

## Prometheus Scrape Config Example

```yaml
scrape_configs:
  - job_name: 'dns-collector'
    static_configs:
      - targets: ['dns-collector:9090']
    scrape_interval: 15s

  - job_name: 'dns-collector-webapi'
    static_configs:
      - targets: ['web-api:8080']
    metrics_path: /metrics
    scrape_interval: 15s
```

---

## Grafana Dashboard Queries

### Request Rate
```promql
rate(http_requests_total{job="dns-collector-webapi"}[5m])
```

### DNS Lookup Latency (p95)
```promql
histogram_quantile(0.95, rate(dns_resolver_lookup_duration_seconds_bucket[5m]))
```

### Active Workers
```promql
dns_resolver_active_workers
```

### Error Rate
```promql
rate(dns_resolver_lookups_total{status="error"}[5m])
/ rate(dns_resolver_lookups_total[5m])
```

### Database Size
```promql
dns_db_domains_total + dns_db_ips_total
```
