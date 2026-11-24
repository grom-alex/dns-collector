#!/usr/bin/env bash
# Быстрая проверка статистики DNS Collector через PostgreSQL.
# Запуск из любой директории: ./docker_check_stats.sh
# Требуется docker compose и запущенный сервис postgres.

set -euo pipefail

cd "$(dirname "$0")/.."

DB_USER="${POSTGRES_USER:-dns_collector}"
DB_NAME="${POSTGRES_DB:-dns_collector}"

psql_cmd() {
    docker compose exec -T postgres \
        psql -X -q -A -F ' | ' -P pager=off -U "${DB_USER}" -d "${DB_NAME}" -c "$1"
}

echo "=== Domains ==="
psql_cmd "SELECT count(*) AS domains FROM domain;"
psql_cmd "SELECT count(*) AS pending FROM domain WHERE resolv_count < max_resolv;"
psql_cmd "SELECT count(*) AS ips FROM ip;"
echo

echo "=== Domains list ==="
psql_cmd "SELECT domain, resolv_count, max_resolv, last_resolv_time
          FROM domain ORDER BY domain LIMIT 50;"
echo

echo "=== IP per domain (IPv4 / IPv6) ==="
psql_cmd "SELECT d.domain,
                 count(*) FILTER (WHERE i.type = 'ipv4') AS ipv4,
                 count(*) FILTER (WHERE i.type = 'ipv6') AS ipv6
          FROM domain d
          LEFT JOIN ip i ON i.domain_id = d.id
          GROUP BY d.id, d.domain
          ORDER BY d.domain
          LIMIT 50;"
echo

echo "=== Requests ==="
psql_cmd "SELECT count(*) AS requests FROM domain_stat;"
psql_cmd "SELECT count(*) AS requests_24h
          FROM domain_stat
          WHERE timestamp >= now() - interval '24 hours';"
