#!/usr/bin/env bash
# 24-hour PostgreSQL/collector monitoring sampler.
#
# Usage:
#   sudo OUT_DIR=/opt/dns-collector/monitoring/2026-11-23 ./pg_monitor_24h.sh
#   DURATION_SECONDS=10 INTERVAL_SECONDS=2 OUT_DIR=/tmp/test ./pg_monitor_24h.sh
#
# Requires: docker, docker compose services (postgres, collector, web-api), psql inside postgres container.
# Does not store secrets: psql uses local socket/trust inside the container.

set -uo pipefail

DURATION_HOURS="${DURATION_HOURS:-24}"
DURATION_SECONDS="${DURATION_SECONDS:-}"
INTERVAL_SECONDS="${INTERVAL_SECONDS:-300}"
OUT_DIR="${OUT_DIR:-/opt/dns-collector/monitoring/$(date -u +%Y%m%dT%H%M%SZ)}"
PG_CONTAINER="${PG_CONTAINER:-dns-collector-postgres}"
PG_USER="${PG_USER:-dns_collector}"
PG_DB="${PG_DB:-dns_collector}"
COLLECTOR_URL="${COLLECTOR_URL:-http://localhost:8081/metrics}"
WEBAPI_URL="${WEBAPI_URL:-http://localhost:8080/metrics}"

mkdir -p "$OUT_DIR" || exit 1
cd "$OUT_DIR" || exit 1

now_utc() { date -u +%Y-%m-%dT%H:%M:%SZ; }

cat > meta.txt <<EOF
start_utc=$(now_utc)
host=$(hostname)
duration_hours=$DURATION_HOURS
duration_seconds=$DURATION_SECONDS
interval_seconds=$INTERVAL_SECONDS
out_dir=$OUT_DIR
pg_container=$PG_CONTAINER
pg_user=$PG_USER
pg_db=$PG_DB
collector_url=$COLLECTOR_URL
webapi_url=$WEBAPI_URL
EOF

psql_exec() {
  docker exec "$PG_CONTAINER" psql -U "$PG_USER" -d "$PG_DB" -X -v ON_ERROR_STOP=1 "$@"
}

append_csv() {
  local file="$1"
  local header_sql="$2"
  local data_sql="$3"
  local tmp
  tmp="$(mktemp)"
  if ! psql_exec -c "COPY ($data_sql) TO STDOUT WITH CSV" >"$tmp" 2>>errors.log; then
    echo "$(now_utc),ERROR" >> "$file"
    rm -f "$tmp"
    return 1
  fi
  if [ ! -s "$file" ]; then
    psql_exec -c "COPY ($header_sql) TO STDOUT WITH CSV HEADER" >> "$file" 2>>errors.log || true
  fi
  cat "$tmp" >> "$file"
  rm -f "$tmp"
}

sample() {
  local TS
  TS="$(now_utc)"

  append_csv "pg_stat_statements.csv" \
    "SELECT '$TS'::text AS ts, queryid, calls, rows, total_exec_time, mean_exec_time, wal_records, wal_fpi, wal_bytes, shared_blks_hit, shared_blks_read, shared_blks_dirtied, shared_blks_written, temp_blks_read, temp_blks_written, blk_read_time, blk_write_time, left(query, 1000) AS query FROM pg_stat_statements WHERE false" \
    "SELECT '$TS'::text AS ts, queryid, calls, rows, total_exec_time, mean_exec_time, wal_records, wal_fpi, wal_bytes, shared_blks_hit, shared_blks_read, shared_blks_dirtied, shared_blks_written, temp_blks_read, temp_blks_written, blk_read_time, blk_write_time, left(query, 1000) AS query FROM pg_stat_statements"

  append_csv "tables.csv" \
    "SELECT '$TS'::text AS ts, relname, n_live_tup, n_dead_tup, n_mod_since_analyze, n_tup_ins, n_tup_upd, n_tup_hot_upd, n_tup_del, autovacuum_count, vacuum_count, analyze_count, last_autovacuum, last_analyze FROM pg_stat_user_tables WHERE false" \
    "SELECT '$TS'::text AS ts, relname, n_live_tup, n_dead_tup, n_mod_since_analyze, n_tup_ins, n_tup_upd, n_tup_hot_upd, n_tup_del, autovacuum_count, vacuum_count, analyze_count, last_autovacuum, last_analyze FROM pg_stat_user_tables"

  append_csv "pg_stat_io.csv" \
    "SELECT '$TS'::text AS ts, backend_type, object, context, reads, read_time, writes, write_time, writebacks, extends, hits, evictions, reuses, fsyncs FROM pg_stat_io WHERE false" \
    "SELECT '$TS'::text AS ts, backend_type, object, context, reads, read_time, writes, write_time, writebacks, extends, hits, evictions, reuses, fsyncs FROM pg_stat_io"

  if [ ! -s system.csv ]; then
    echo "ts,checkpoints_timed,checkpoints_req,checkpoint_write_time,checkpoint_sync_time,buffers_checkpoint,buffers_clean,buffers_backend,buffers_alloc,wal_records,wal_fpi,wal_bytes,wal_buffers_full,wal_write,wal_sync,blks_read,blks_hit,xact_commit,xact_rollback,tup_inserted,tup_updated,tup_deleted,pgss_dealloc" > system.csv
  fi
  psql_exec -At -F ',' -c "SELECT '$TS',
    (SELECT checkpoints_timed FROM pg_stat_bgwriter),
    (SELECT checkpoints_req FROM pg_stat_bgwriter),
    (SELECT checkpoint_write_time FROM pg_stat_bgwriter),
    (SELECT checkpoint_sync_time FROM pg_stat_bgwriter),
    (SELECT buffers_checkpoint FROM pg_stat_bgwriter),
    (SELECT buffers_clean FROM pg_stat_bgwriter),
    (SELECT buffers_backend FROM pg_stat_bgwriter),
    (SELECT buffers_alloc FROM pg_stat_bgwriter),
    (SELECT wal_records FROM pg_stat_wal),
    (SELECT wal_fpi FROM pg_stat_wal),
    (SELECT wal_bytes FROM pg_stat_wal),
    (SELECT wal_buffers_full FROM pg_stat_wal),
    (SELECT wal_write FROM pg_stat_wal),
    (SELECT wal_sync FROM pg_stat_wal),
    (SELECT blks_read FROM pg_stat_database WHERE datname=current_database()),
    (SELECT blks_hit FROM pg_stat_database WHERE datname=current_database()),
    (SELECT xact_commit FROM pg_stat_database WHERE datname=current_database()),
    (SELECT xact_rollback FROM pg_stat_database WHERE datname=current_database()),
    (SELECT tup_inserted FROM pg_stat_database WHERE datname=current_database()),
    (SELECT tup_updated FROM pg_stat_database WHERE datname=current_database()),
    (SELECT tup_deleted FROM pg_stat_database WHERE datname=current_database()),
    (SELECT dealloc FROM pg_stat_statements_info);" >> system.csv 2>>errors.log

  if [ ! -s disk.csv ]; then
    echo "ts,dev,reads_completed,sectors_read,writes_completed,sectors_written" > disk.csv
  fi
  awk -v ts="$TS" '$3=="sda1"||$3=="sdb1"{print ts","$3","$4","$6","$8","$10}' /proc/diskstats >> disk.csv

  if [ ! -s docker.csv ]; then
    echo "ts,container,cpu,mem,block_io" > docker.csv
  fi
  docker stats --no-stream --format '{{.Name}}|{{.CPUPerc}}|{{.MemUsage}}|{{.BlockIO}}' 2>>errors.log | while IFS='|' read -r name cpu mem bio; do
    echo "$TS,$name,$cpu,$mem,$bio" >> docker.csv
  done

  {
    echo "### $TS"
    curl -sS --max-time 5 "$COLLECTOR_URL" || echo "# collector metrics error"
  } >> collector-metrics.log 2>&1

  {
    echo "### $TS"
    curl -sS --max-time 5 "$WEBAPI_URL" || echo "# web-api metrics error"
  } >> web-api-metrics.log 2>&1
}

START="$(date +%s)"
if [ -n "$DURATION_SECONDS" ]; then
  END=$((START + DURATION_SECONDS))
else
  END=$((START + DURATION_HOURS * 3600))
fi
SAMPLE=0

echo "monitor started: $OUT_DIR" >> monitor.log
while [ "$(date +%s)" -lt "$END" ]; do
  SAMPLE=$((SAMPLE + 1))
  echo "$(now_utc) sample $SAMPLE" >> monitor.log
  sample
  sleep "$INTERVAL_SECONDS"
done

cat >> meta.txt <<EOF
end_utc=$(now_utc)
samples=$SAMPLE
EOF
echo "monitor finished: $(now_utc)" >> monitor.log
