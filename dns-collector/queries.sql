-- Полезные SQL-запросы для DNS Collector (PostgreSQL)
-- Запуск:
--   docker compose exec -T postgres psql -U dns_collector -d dns_collector -f - < queries.sql
-- или:
--   psql "$DATABASE_URL" -f queries.sql

\pset pager off

-- =====================================================
-- Домены и IP
-- =====================================================

-- 1. Все домены с состоянием резолвинга
SELECT
    id,
    domain,
    time_insert,
    resolv_count,
    max_resolv,
    last_resolv_time,
    last_seen
FROM domain
ORDER BY last_resolv_time DESC
LIMIT 100;

-- 2. Домены, ожидающие резолва
SELECT
    domain,
    resolv_count,
    max_resolv,
    (max_resolv - resolv_count) AS remaining,
    last_resolv_time
FROM domain
WHERE resolv_count < max_resolv
ORDER BY last_resolv_time ASC
LIMIT 100;

-- 3. IP-адреса конкретного домена
SELECT
    d.domain,
    i.ip,
    i.type,
    i.time AS resolved_at
FROM domain d
JOIN ip i ON i.domain_id = d.id
WHERE d.domain = 'example.com'
ORDER BY i.type, i.ip;

-- 4. Топ-20 доменов по количеству IP
SELECT
    d.domain,
    COUNT(i.id) AS ip_count,
    string_agg(i.ip, ', ' ORDER BY i.ip) AS ip_addresses
FROM domain d
LEFT JOIN ip i ON i.domain_id = d.id
GROUP BY d.id, d.domain
ORDER BY ip_count DESC
LIMIT 20;

-- 5. Домены с IPv4 и IPv6
SELECT
    d.domain,
    COUNT(*) FILTER (WHERE i.type = 'ipv4') AS ipv4_count,
    COUNT(*) FILTER (WHERE i.type = 'ipv6') AS ipv6_count
FROM domain d
JOIN ip i ON i.domain_id = d.id
GROUP BY d.id, d.domain
HAVING COUNT(*) FILTER (WHERE i.type = 'ipv4') > 0
   AND COUNT(*) FILTER (WHERE i.type = 'ipv6') > 0
ORDER BY (COUNT(*)) DESC
LIMIT 50;

-- 6. Домены, которые давно не запрашивали
SELECT domain, last_seen, resolv_count
FROM domain
WHERE last_seen IS NOT NULL
  AND last_seen < now() - interval '30 days'
ORDER BY last_seen
LIMIT 100;

-- =====================================================
-- Статистика запросов
-- =====================================================

-- 7. Запросы за последние 24 часа
SELECT COUNT(*) AS requests
FROM domain_stat
WHERE timestamp >= now() - interval '24 hours';

-- 8. Уникальные клиенты за сутки
SELECT COUNT(DISTINCT client_ip) AS clients
FROM domain_stat
WHERE timestamp >= now() - interval '24 hours';

-- 9. Топ-20 доменов за сутки
SELECT domain, COUNT(*) AS requests
FROM domain_stat
WHERE timestamp >= now() - interval '24 hours'
GROUP BY domain
ORDER BY requests DESC
LIMIT 20;

-- 10. Топ-20 клиентов за сутки
SELECT client_ip, COUNT(*) AS requests, COUNT(DISTINCT domain) AS domains
FROM domain_stat
WHERE timestamp >= now() - interval '24 hours'
GROUP BY client_ip
ORDER BY requests DESC
LIMIT 20;

-- 11. Распределение по rtype
SELECT
    rtype,
    COUNT(*) AS requests,
    round(100.0 * COUNT(*) / SUM(COUNT(*)) OVER (), 2) AS pct
FROM domain_stat
WHERE timestamp >= now() - interval '24 hours'
GROUP BY rtype
ORDER BY requests DESC;

-- 12. Активность по часам за сутки
SELECT
    date_trunc('hour', timestamp) AS hour,
    COUNT(*) AS requests,
    COUNT(DISTINCT client_ip) AS clients
FROM domain_stat
WHERE timestamp >= now() - interval '24 hours'
GROUP BY hour
ORDER BY hour;

-- 13. Домены, запрошенные более чем с 10 клиентов
SELECT domain, COUNT(DISTINCT client_ip) AS clients
FROM domain_stat
WHERE timestamp >= now() - interval '7 days'
GROUP BY domain
HAVING COUNT(DISTINCT client_ip) > 10
ORDER BY clients DESC
LIMIT 50;

-- =====================================================
-- Retention / обслуживание
-- =====================================================

-- 14. Сколько строк попадает под retention (не удалять вручную!)
SELECT COUNT(*) AS rows_to_delete
FROM domain_stat
WHERE timestamp < now() - interval '30 days';

-- 15. Размеры таблиц
SELECT
    relname,
    pg_size_pretty(pg_total_relation_size(relid)) AS total,
    pg_size_pretty(pg_relation_size(relid)) AS heap
FROM pg_stat_user_tables
ORDER BY pg_total_relation_size(relid) DESC;

-- 16. Dead tuples и autovacuum
SELECT
    relname,
    n_live_tup,
    n_dead_tup,
    last_vacuum,
    last_autovacuum,
    autovacuum_count
FROM pg_stat_user_tables
ORDER BY n_dead_tup DESC;

-- 17. HOT-обновления (ключевая метрика после оптимизации)
SELECT
    relname,
    n_tup_upd,
    n_tup_hot_upd,
    round(100.0 * n_tup_hot_upd / NULLIF(n_tup_upd, 0), 1) AS hot_pct
FROM pg_stat_user_tables
WHERE n_tup_upd > 0
ORDER BY n_tup_upd DESC;

-- 18. Буферы и checkpoints
SELECT * FROM pg_stat_bgwriter;

-- 19. Крупнейшие индексы
SELECT
    relname,
    indexrelname,
    idx_scan,
    pg_size_pretty(pg_relation_size(indexrelid)) AS size
FROM pg_stat_user_indexes
ORDER BY pg_relation_size(indexrelid) DESC
LIMIT 20;
