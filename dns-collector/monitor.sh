#!/usr/bin/env bash
# Мониторинг DNS Collector через psql.
#
# Требуется psql (клиент PostgreSQL) и доступ к БД.
# Параметры подключения — стандартные PG*:
#   PGHOST, PGPORT, PGUSER, PGPASSWORD, PGDATABASE
# либо DATABASE_URL.
#
# Пример:
#   PGHOST=localhost PGUSER=dns_collector PGDATABASE=dns_collector ./monitor.sh
#   DATABASE_URL=postgres://dns_collector:pass@localhost:5432/dns_collector ./monitor.sh

set -uo pipefail

PSQL="${PSQL:-psql}"
DB_ARGS=()
if [[ -n "${DATABASE_URL:-}" ]]; then
    DB_ARGS=("${DATABASE_URL}")
fi

if ! command -v "${PSQL}" >/dev/null 2>&1; then
    echo "psql not found. Install postgresql-client or use:" >&2
    echo "  docker compose exec -T postgres psql -U dns_collector -d dns_collector" >&2
    exit 1
fi

query() {
    "${PSQL}" -X -q -A -F ' | ' -P pager=off ${DB_ARGS[@]+"${DB_ARGS[@]}"} -c "$1"
}

if ! query "SELECT 1" >/dev/null 2>&1; then
    echo "Cannot connect to PostgreSQL. Check PG* env / DATABASE_URL." >&2
    exit 1
fi

echo "======================================"
echo "   DNS Collector Monitoring"
echo "======================================"
echo

echo "=== Общая статистика ==="
query "SELECT
         (SELECT count(*) FROM domain) AS domains,
         (SELECT count(*) FROM ip) AS ips,
         (SELECT count(*) FROM domain_stat) AS requests;"
echo

echo "=== Размеры таблиц ==="
query "SELECT relname,
              pg_size_pretty(pg_total_relation_size(relid)) AS total,
              pg_size_pretty(pg_relation_size(relid)) AS heap
       FROM pg_stat_user_tables
       ORDER BY pg_total_relation_size(relid) DESC;"
echo

echo "=== Запись: UPDATE / HOT ==="
query "SELECT relname,
              n_tup_upd,
              n_tup_hot_upd,
              round(100.0 * n_tup_hot_upd / NULLIF(n_tup_upd, 0), 1) AS hot_pct
       FROM pg_stat_user_tables
       WHERE n_tup_upd > 0
       ORDER BY n_tup_upd DESC;"
echo

echo "=== Dead tuples / autovacuum ==="
query "SELECT relname, n_live_tup, n_dead_tup, autovacuum_count, last_autovacuum
       FROM pg_stat_user_tables
       ORDER BY n_dead_tup DESC;"
echo

echo "=== Типы IP ==="
query "SELECT type, count(*) AS count, count(DISTINCT domain_id) AS domains
       FROM ip GROUP BY type ORDER BY count DESC;"
echo

echo "=== Топ-10 доменов по IP ==="
query "SELECT d.domain, count(i.id) AS ips
       FROM domain d LEFT JOIN ip i ON i.domain_id = d.id
       GROUP BY d.id, d.domain
       ORDER BY ips DESC LIMIT 10;"
echo

echo "=== Топ-10 запрашиваемых доменов за 24 часа ==="
query "SELECT domain, count(*) AS requests
       FROM domain_stat
       WHERE timestamp >= now() - interval '24 hours'
       GROUP BY domain ORDER BY requests DESC LIMIT 10;"
echo

echo "=== Топ-10 клиентов за 24 часа ==="
query "SELECT client_ip, count(*) AS requests, count(DISTINCT domain) AS domains
       FROM domain_stat
       WHERE timestamp >= now() - interval '24 hours'
       GROUP BY client_ip ORDER BY requests DESC LIMIT 10;"
echo

echo "=== Активность за 24 часа ==="
query "SELECT
         count(*) AS requests,
         count(DISTINCT domain) AS domains,
         count(DISTINCT client_ip) AS clients
       FROM domain_stat
       WHERE timestamp >= now() - interval '24 hours';"
echo

echo "=== Ожидают резолва ==="
query "SELECT count(*) AS pending
       FROM domain WHERE resolv_count < max_resolv;"
echo

echo "======================================"
echo "   Monitoring Complete"
echo "======================================"
