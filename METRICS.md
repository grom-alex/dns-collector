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

### Security

**Metrics Endpoint Protection:**

By default, metrics endpoints are **unauthenticated** (standard for Prometheus). Consider these security measures:

1. **Firewall Rules**: Restrict access to metrics ports (9090, 8080) to trusted networks
   ```bash
   # Example: iptables rule to allow only monitoring server
   iptables -A INPUT -p tcp --dport 9090 -s 192.168.1.100 -j ACCEPT
   iptables -A INPUT -p tcp --dport 9090 -j DROP
   ```

2. **Reverse Proxy with Authentication**: Use nginx/Caddy with basic auth
   ```nginx
   location /metrics {
       auth_basic "Metrics";
       auth_basic_user_file /etc/nginx/.htpasswd;
       proxy_pass http://dns-collector:9090/metrics;
   }
   ```

3. **VPN/Private Network**: Deploy metrics endpoints on internal network only

4. **InfluxDB TLS**: Always use TLS in production
   - Set `insecure_skip_verify: false` (default)
   - Use valid certificates or internal CA
   - Rotate tokens regularly

**Best Practices:**
- Never expose metrics ports to public internet
- Use environment variables for sensitive credentials (`INFLUXDB_TOKEN`)
- Enable TLS for InfluxDB connections
- Monitor metrics endpoint access logs
- Implement network segmentation

---

## dns-collector Metrics

### Resolver Metrics

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `dns_resolver_domains_processed_total` | Counter | `status` | Total domains processed (success/no_results) |
| `dns_resolver_lookups_total` | Counter | `ip_version`, `status` | DNS lookups by IP version (ipv4/ipv6) and status |
| `dns_resolver_lookup_duration_seconds` | Histogram | `ip_version` | DNS lookup duration |
| `dns_resolver_batch_size` | Gauge | - | Current batch size being resolved |
| `dns_resolver_active_workers` | Gauge | - | Number of active resolver workers |

### UDP Server Metrics

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `dns_server_messages_received_total` | Counter | `status` | Messages received (valid/invalid) |
| `dns_server_domains_received_total` | Counter | `rtype` | Domains received by record type |
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
| `dns_db_domains_total` | Gauge | - | Total domains in database (updated every 30s) |
| `dns_db_ips_total` | Gauge | - | Total IP addresses in database (updated every 30s) |

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
| `api_export_generated_total` | Counter | `type` | Exports generated (stats_excel/domains_excel) |

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

### New Domains Rate
```promql
rate(dns_server_new_domains_total[5m])
```

### Export Count by Type
```promql
sum by (type) (api_export_generated_total)
```

---

## Prometheus Alerting Rules

### Critical Alerts

#### High DNS Error Rate
```yaml
- alert: HighDNSErrorRate
  expr: |
    rate(dns_resolver_lookups_total{status="error"}[5m])
    / rate(dns_resolver_lookups_total[5m]) > 0.10
  for: 5m
  labels:
    severity: critical
  annotations:
    summary: "High DNS resolution error rate"
    description: "DNS error rate is {{ $value | humanizePercentage }} (threshold: 10%)"
```

#### DNS Collector Down
```yaml
- alert: DNSCollectorDown
  expr: up{job="dns-collector"} == 0
  for: 1m
  labels:
    severity: critical
  annotations:
    summary: "DNS Collector service is down"
    description: "DNS Collector has been down for more than 1 minute"
```

#### Web API Down
```yaml
- alert: WebAPIDown
  expr: up{job="dns-collector-webapi"} == 0
  for: 1m
  labels:
    severity: critical
  annotations:
    summary: "Web API service is down"
    description: "Web API has been down for more than 1 minute"
```

### Warning Alerts

#### High DNS Lookup Latency
```yaml
- alert: HighDNSLookupLatency
  expr: |
    histogram_quantile(0.95,
      rate(dns_resolver_lookup_duration_seconds_bucket[5m])
    ) > 2
  for: 10m
  labels:
    severity: warning
  annotations:
    summary: "High DNS lookup latency"
    description: "P95 DNS lookup latency is {{ $value }}s (threshold: 2s)"
```

#### No New Domains Received
```yaml
- alert: NoNewDomainsReceived
  expr: |
    rate(dns_server_new_domains_total[10m]) == 0
  for: 30m
  labels:
    severity: warning
  annotations:
    summary: "No new domains received"
    description: "No new domains have been received in the last 30 minutes"
```

#### High HTTP Error Rate
```yaml
- alert: HighHTTPErrorRate
  expr: |
    sum(rate(http_requests_total{status=~"5.."}[5m]))
    / sum(rate(http_requests_total[5m])) > 0.05
  for: 5m
  labels:
    severity: warning
  annotations:
    summary: "High HTTP 5xx error rate on Web API"
    description: "HTTP 5xx error rate is {{ $value | humanizePercentage }} (threshold: 5%)"
```

#### Slow HTTP Requests
```yaml
- alert: SlowHTTPRequests
  expr: |
    histogram_quantile(0.95,
      rate(http_request_duration_seconds_bucket{path="/api/stats"}[5m])
    ) > 1
  for: 10m
  labels:
    severity: warning
  annotations:
    summary: "Slow HTTP requests detected"
    description: "P95 latency for {{ $labels.path }} is {{ $value }}s (threshold: 1s)"
```

### Performance Alerts

#### High Memory Usage
```yaml
- alert: HighMemoryUsage
  expr: |
    go_memstats_alloc_bytes / 1024 / 1024 > 512
  for: 15m
  labels:
    severity: warning
  annotations:
    summary: "High memory usage"
    description: "Memory usage is {{ $value }}MB (threshold: 512MB)"
```

#### High Goroutine Count
```yaml
- alert: HighGoroutineCount
  expr: go_goroutines > 1000
  for: 15m
  labels:
    severity: warning
  annotations:
    summary: "High goroutine count"
    description: "Goroutine count is {{ $value }} (threshold: 1000)"
```

#### Cleanup Job Not Running
```yaml
- alert: CleanupJobNotRunning
  expr: |
    time() - (dns_cleanup_runs_total * 0 + time()) > 86400
  for: 1h
  labels:
    severity: warning
  annotations:
    summary: "Cleanup job hasn't run recently"
    description: "Cleanup job hasn't run in over 24 hours"
```

### Database Alerts

#### Database Growing Too Fast
```yaml
- alert: DatabaseGrowingTooFast
  expr: |
    rate(dns_db_domains_total[1h]) > 1000
  for: 2h
  labels:
    severity: warning
  annotations:
    summary: "Database growing rapidly"
    description: "Database is growing at {{ $value }} domains/hour (threshold: 1000/hour)"
```

#### No Active Resolver Workers
```yaml
- alert: NoActiveResolverWorkers
  expr: dns_resolver_active_workers == 0
  for: 5m
  labels:
    severity: critical
  annotations:
    summary: "No active DNS resolver workers"
    description: "All resolver workers are inactive - DNS resolution has stopped"
```

### Example Alert Manager Configuration

```yaml
route:
  receiver: 'default'
  group_by: ['alertname', 'severity']
  group_wait: 10s
  group_interval: 5m
  repeat_interval: 4h
  routes:
    - match:
        severity: critical
      receiver: 'pagerduty'
      continue: true
    - match:
        severity: warning
      receiver: 'slack'

receivers:
  - name: 'default'
    email_configs:
      - to: 'ops@example.com'
  - name: 'slack'
    slack_configs:
      - api_url: 'https://hooks.slack.com/services/YOUR/WEBHOOK/URL'
        channel: '#alerts'
        title: '{{ .GroupLabels.alertname }}'
        text: '{{ range .Alerts }}{{ .Annotations.description }}{{ end }}'
  - name: 'pagerduty'
    pagerduty_configs:
      - service_key: 'YOUR_PAGERDUTY_KEY'
```

---

## Monitoring Best Practices

### Retention Policies

**Prometheus:**
- Default retention: 15 days
- High-traffic deployments: 7 days (reduce storage)
- Low-traffic/analysis: 30 days
- Configure: `--storage.tsdb.retention.time=15d`

**InfluxDB2:**
- Create retention policies per use case:
  ```influxql
  # Short-term high-resolution data (10s interval)
  CREATE RETENTION POLICY "short_term" ON "dns-metrics" DURATION 7d REPLICATION 1 DEFAULT

  # Long-term downsampled data (5m aggregates)
  CREATE RETENTION POLICY "long_term" ON "dns-metrics" DURATION 365d REPLICATION 1
  ```

### Cardinality Management

**Monitor label cardinality:**
```promql
# Check unique label combinations
count by (__name__) (count by (__name__, instance, job) (up))
```

**Avoid high-cardinality labels:**
- ❌ Don't use domain names as labels
- ❌ Don't use timestamps or UUIDs as labels
- ✅ Use status codes, IP versions, record types
- ✅ Aggregate before storing in InfluxDB

### Dashboard Organization

**Recommended dashboard structure:**
1. **Overview** - System health, error rates, traffic
2. **DNS Resolver** - Lookup latency, error rates, worker status
3. **Web API** - HTTP metrics, endpoint performance
4. **Database** - Growth trends, cleanup statistics
5. **Infrastructure** - CPU, memory, goroutines, GC

### Grafana Variables

Use variables for flexible dashboards:
```
$job = dns-collector, dns-collector-webapi
$instance = dns-collector:9090, web-api:8080
$quantile = 0.50, 0.75, 0.95, 0.99
```

---

## Troubleshooting

### High Memory Usage

**Diagnose:**
```promql
go_memstats_alloc_bytes / 1024 / 1024  # Current allocation in MB
rate(go_memstats_alloc_bytes_total[5m])  # Allocation rate
go_memstats_heap_objects  # Object count
```

**Solutions:**
- Check for memory leaks with pprof
- Reduce batch sizes in resolver
- Adjust cleanup intervals
- Scale horizontally instead of vertically

### Metrics Not Updating

**Check:**
1. Service is running: `docker ps`
2. Metrics endpoint accessible: `curl http://localhost:9090/metrics`
3. Prometheus scraping: Check Targets page
4. InfluxDB connection: Check logs for TLS errors
5. Firewall rules allowing scraping

### InfluxDB Write Failures

**Common issues:**
- TLS certificate errors → Set `insecure_skip_verify: true` (dev only)
- Authentication failures → Check `INFLUXDB_TOKEN` environment variable
- Bucket doesn't exist → Create bucket in InfluxDB UI
- Quota exceeded → Check InfluxDB Cloud limits

**Debug:**
```bash
# Check InfluxDB health
curl -k https://influxdb:8086/health

# Test write access
influx write \
  --bucket dns-metrics \
  --org my-org \
  --token $INFLUXDB_TOKEN \
  --precision s \
  "test_metric value=1"
```
