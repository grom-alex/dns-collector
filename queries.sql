-- Полезные SQL запросы для анализа данных DNS Collector

-- =====================================================
-- Запросы к domains.db
-- =====================================================

-- 1. Показать все домены с информацией о резолвинге
SELECT
    id,
    domain,
    datetime(time_insert) as inserted,
    resolv_count,
    max_resolv,
    datetime(last_resolv_time) as last_resolved
FROM domain
ORDER BY last_resolv_time DESC;

-- 2. Показать домены, которые еще нужно резолвить
SELECT
    domain,
    resolv_count,
    max_resolv,
    (max_resolv - resolv_count) as remaining_resolves
FROM domain
WHERE resolv_count < max_resolv
ORDER BY last_resolv_time ASC;

-- 3. Показать все IP адреса для конкретного домена
SELECT
    d.domain,
    i.ip,
    i.type,
    datetime(i.time) as resolved_at
FROM domain d
JOIN ip i ON d.id = i.domain_id
WHERE d.domain = 'google.com'
ORDER BY i.type, i.ip;

-- 4. Показать топ доменов по количеству IP адресов
SELECT
    d.domain,
    COUNT(i.id) as ip_count,
    GROUP_CONCAT(i.ip, ', ') as ip_addresses
FROM domain d
LEFT JOIN ip i ON d.id = i.domain_id
GROUP BY d.id
ORDER BY ip_count DESC
LIMIT 20;

-- 5. Показать домены с IPv4 и IPv6 адресами
SELECT
    d.domain,
    SUM(CASE WHEN i.type = 'ipv4' THEN 1 ELSE 0 END) as ipv4_count,
    SUM(CASE WHEN i.type = 'ipv6' THEN 1 ELSE 0 END) as ipv6_count
FROM domain d
LEFT JOIN ip i ON d.id = i.domain_id
GROUP BY d.id
HAVING ipv4_count > 0 OR ipv6_count > 0
ORDER BY (ipv4_count + ipv6_count) DESC;

-- 6. Показать недавно добавленные домены
SELECT
    domain,
    datetime(time_insert) as added_at,
    resolv_count
FROM domain
ORDER BY time_insert DESC
LIMIT 50;

-- 7. Показать все IP адреса с их доменами
SELECT
    i.ip,
    i.type,
    d.domain,
    datetime(i.time) as last_updated
FROM ip i
JOIN domain d ON i.domain_id = d.id
ORDER BY i.time DESC;

-- 8. Статистика по типам IP адресов
SELECT
    type,
    COUNT(*) as count,
    COUNT(DISTINCT domain_id) as unique_domains
FROM ip
GROUP BY type;

-- 9. Домены, которые достигли максимального количества резолвингов
SELECT
    domain,
    resolv_count,
    max_resolv,
    datetime(last_resolv_time) as last_resolved
FROM domain
WHERE resolv_count >= max_resolv
ORDER BY last_resolv_time DESC;

-- 10. Общая статистика по базе
SELECT
    (SELECT COUNT(*) FROM domain) as total_domains,
    (SELECT COUNT(*) FROM ip) as total_ips,
    (SELECT COUNT(*) FROM domain WHERE resolv_count >= max_resolv) as fully_resolved,
    (SELECT COUNT(*) FROM domain WHERE resolv_count < max_resolv) as pending_resolution;


-- =====================================================
-- Запросы к stats.db
-- =====================================================

-- 11. Показать последние запросы
SELECT
    id,
    domain,
    client_ip,
    rtype,
    datetime(timestamp) as requested_at
FROM domain_stat
ORDER BY timestamp DESC
LIMIT 100;

-- 12. Топ доменов по количеству запросов
SELECT
    domain,
    COUNT(*) as request_count,
    COUNT(DISTINCT client_ip) as unique_clients
FROM domain_stat
GROUP BY domain
ORDER BY request_count DESC
LIMIT 20;

-- 13. Топ клиентов по количеству запросов
SELECT
    client_ip,
    COUNT(*) as request_count,
    COUNT(DISTINCT domain) as unique_domains
FROM domain_stat
GROUP BY client_ip
ORDER BY request_count DESC
LIMIT 20;

-- 14. Статистика по типам резолвинга (cache vs dns)
SELECT
    rtype,
    COUNT(*) as count,
    ROUND(COUNT(*) * 100.0 / (SELECT COUNT(*) FROM domain_stat), 2) as percentage
FROM domain_stat
GROUP BY rtype
ORDER BY count DESC;

-- 15. Активность по часам
SELECT
    strftime('%Y-%m-%d %H:00', timestamp) as hour,
    COUNT(*) as request_count
FROM domain_stat
GROUP BY hour
ORDER BY hour DESC
LIMIT 24;

-- 16. Активность по дням
SELECT
    DATE(timestamp) as day,
    COUNT(*) as request_count,
    COUNT(DISTINCT domain) as unique_domains,
    COUNT(DISTINCT client_ip) as unique_clients
FROM domain_stat
GROUP BY day
ORDER BY day DESC
LIMIT 30;

-- 17. Поиск запросов конкретного клиента
SELECT
    domain,
    rtype,
    datetime(timestamp) as requested_at
FROM domain_stat
WHERE client_ip = '192.168.0.10'
ORDER BY timestamp DESC;

-- 18. Домены, запрашиваемые несколькими клиентами
SELECT
    domain,
    COUNT(DISTINCT client_ip) as client_count,
    COUNT(*) as total_requests
FROM domain_stat
GROUP BY domain
HAVING client_count > 1
ORDER BY client_count DESC;

-- 19. Статистика за последний час
SELECT
    COUNT(*) as requests_last_hour,
    COUNT(DISTINCT domain) as unique_domains,
    COUNT(DISTINCT client_ip) as unique_clients
FROM domain_stat
WHERE timestamp >= datetime('now', '-1 hour');

-- 20. Статистика за последние 24 часа
SELECT
    COUNT(*) as requests_last_24h,
    COUNT(DISTINCT domain) as unique_domains,
    COUNT(DISTINCT client_ip) as unique_clients
FROM domain_stat
WHERE timestamp >= datetime('now', '-24 hours');


-- =====================================================
-- Комбинированные запросы (требуют подключения обеих БД)
-- =====================================================

-- Для выполнения комбинированных запросов используйте:
-- sqlite3 domains.db
-- sqlite> ATTACH DATABASE 'stats.db' AS stats;

-- 21. Сравнение данных: домены в основной БД vs статистике
-- (выполняется в domains.db после ATTACH)
/*
ATTACH DATABASE 'stats.db' AS stats;

SELECT
    d.domain,
    d.resolv_count,
    COUNT(i.id) as ip_count,
    COALESCE(s.stat_count, 0) as request_count
FROM domain d
LEFT JOIN ip i ON d.id = i.domain_id
LEFT JOIN (
    SELECT domain, COUNT(*) as stat_count
    FROM stats.domain_stat
    GROUP BY domain
) s ON d.domain = s.domain
GROUP BY d.id
ORDER BY request_count DESC;
*/

-- =====================================================
-- Служебные запросы
-- =====================================================

-- Очистка старых записей статистики (старше 30 дней)
-- DELETE FROM domain_stat WHERE timestamp < datetime('now', '-30 days');

-- Вакуумирование базы для освобождения места
-- VACUUM;

-- Проверка размера баз данных
-- .databases
