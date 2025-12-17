#!/bin/bash

# Скрипт мониторинга DNS Collector

DOMAINS_DB="domains.db"
STATS_DB="stats.db"

# Цвета для вывода
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
RED='\033[0;31m'
NC='\033[0m' # No Color

echo -e "${BLUE}======================================${NC}"
echo -e "${BLUE}   DNS Collector Monitoring${NC}"
echo -e "${BLUE}======================================${NC}"
echo ""

# Проверка существования баз данных
if [ ! -f "$DOMAINS_DB" ]; then
    echo -e "${RED}Database $DOMAINS_DB not found!${NC}"
    exit 1
fi

if [ ! -f "$STATS_DB" ]; then
    echo -e "${RED}Database $STATS_DB not found!${NC}"
    exit 1
fi

# Общая статистика по доменам
echo -e "${GREEN}=== Domains Database Statistics ===${NC}"
echo ""

TOTAL_DOMAINS=$(sqlite3 $DOMAINS_DB "SELECT COUNT(*) FROM domain;")
TOTAL_IPS=$(sqlite3 $DOMAINS_DB "SELECT COUNT(*) FROM ip;")
FULLY_RESOLVED=$(sqlite3 $DOMAINS_DB "SELECT COUNT(*) FROM domain WHERE resolv_count >= max_resolv;")
PENDING=$(sqlite3 $DOMAINS_DB "SELECT COUNT(*) FROM domain WHERE resolv_count < max_resolv;")

echo -e "Total Domains:       ${YELLOW}$TOTAL_DOMAINS${NC}"
echo -e "Total IP Addresses:  ${YELLOW}$TOTAL_IPS${NC}"
echo -e "Fully Resolved:      ${GREEN}$FULLY_RESOLVED${NC}"
echo -e "Pending Resolution:  ${YELLOW}$PENDING${NC}"
echo ""

# Статистика по типам IP
echo -e "${GREEN}=== IP Address Types ===${NC}"
sqlite3 -column -header $DOMAINS_DB "
SELECT
    type as 'Type',
    COUNT(*) as 'Count',
    COUNT(DISTINCT domain_id) as 'Unique Domains'
FROM ip
GROUP BY type;
"
echo ""

# Топ-10 доменов по количеству IP
echo -e "${GREEN}=== Top 10 Domains by IP Count ===${NC}"
sqlite3 -column -header $DOMAINS_DB "
SELECT
    d.domain as 'Domain',
    COUNT(i.id) as 'IPs'
FROM domain d
LEFT JOIN ip i ON d.id = i.domain_id
GROUP BY d.id
ORDER BY COUNT(i.id) DESC
LIMIT 10;
"
echo ""

# Последние добавленные домены
echo -e "${GREEN}=== Recently Added Domains (Last 10) ===${NC}"
sqlite3 -column -header $DOMAINS_DB "
SELECT
    domain as 'Domain',
    datetime(time_insert) as 'Added At',
    resolv_count as 'Resolved'
FROM domain
ORDER BY time_insert DESC
LIMIT 10;
"
echo ""

# Статистика запросов
echo -e "${GREEN}=== Request Statistics ===${NC}"
echo ""

TOTAL_REQUESTS=$(sqlite3 $STATS_DB "SELECT COUNT(*) FROM domain_stat;")
UNIQUE_DOMAINS_STAT=$(sqlite3 $STATS_DB "SELECT COUNT(DISTINCT domain) FROM domain_stat;")
UNIQUE_CLIENTS=$(sqlite3 $STATS_DB "SELECT COUNT(DISTINCT client_ip) FROM domain_stat;")

echo -e "Total Requests:      ${YELLOW}$TOTAL_REQUESTS${NC}"
echo -e "Unique Domains:      ${YELLOW}$UNIQUE_DOMAINS_STAT${NC}"
echo -e "Unique Clients:      ${YELLOW}$UNIQUE_CLIENTS${NC}"
echo ""

# Статистика по типам резолвинга
echo -e "${GREEN}=== Resolution Type Statistics ===${NC}"
sqlite3 -column -header $STATS_DB "
SELECT
    rtype as 'Type',
    COUNT(*) as 'Count',
    ROUND(COUNT(*) * 100.0 / (SELECT COUNT(*) FROM domain_stat), 2) as 'Percentage'
FROM domain_stat
GROUP BY rtype;
"
echo ""

# Топ-10 запрашиваемых доменов
echo -e "${GREEN}=== Top 10 Requested Domains ===${NC}"
sqlite3 -column -header $STATS_DB "
SELECT
    domain as 'Domain',
    COUNT(*) as 'Requests',
    COUNT(DISTINCT client_ip) as 'Clients'
FROM domain_stat
GROUP BY domain
ORDER BY COUNT(*) DESC
LIMIT 10;
"
echo ""

# Топ-10 активных клиентов
echo -e "${GREEN}=== Top 10 Active Clients ===${NC}"
sqlite3 -column -header $STATS_DB "
SELECT
    client_ip as 'Client IP',
    COUNT(*) as 'Requests',
    COUNT(DISTINCT domain) as 'Domains'
FROM domain_stat
GROUP BY client_ip
ORDER BY COUNT(*) DESC
LIMIT 10;
"
echo ""

# Активность за последние 24 часа
echo -e "${GREEN}=== Activity Last 24 Hours ===${NC}"
REQUESTS_24H=$(sqlite3 $STATS_DB "SELECT COUNT(*) FROM domain_stat WHERE timestamp >= datetime('now', '-24 hours');")
DOMAINS_24H=$(sqlite3 $STATS_DB "SELECT COUNT(DISTINCT domain) FROM domain_stat WHERE timestamp >= datetime('now', '-24 hours');")
CLIENTS_24H=$(sqlite3 $STATS_DB "SELECT COUNT(DISTINCT client_ip) FROM domain_stat WHERE timestamp >= datetime('now', '-24 hours');")

echo -e "Requests:       ${YELLOW}$REQUESTS_24H${NC}"
echo -e "Unique Domains: ${YELLOW}$DOMAINS_24H${NC}"
echo -e "Unique Clients: ${YELLOW}$CLIENTS_24H${NC}"
echo ""

# Последние 10 запросов
echo -e "${GREEN}=== Last 10 Requests ===${NC}"
sqlite3 -column -header $STATS_DB "
SELECT
    domain as 'Domain',
    client_ip as 'Client',
    rtype as 'Type',
    datetime(timestamp) as 'Time'
FROM domain_stat
ORDER BY timestamp DESC
LIMIT 10;
"
echo ""

# Размер баз данных
echo -e "${GREEN}=== Database Sizes ===${NC}"
DOMAINS_SIZE=$(du -h $DOMAINS_DB | cut -f1)
STATS_SIZE=$(du -h $STATS_DB | cut -f1)
echo -e "Domains DB: ${YELLOW}$DOMAINS_SIZE${NC}"
echo -e "Stats DB:   ${YELLOW}$STATS_SIZE${NC}"
echo ""

echo -e "${BLUE}======================================${NC}"
echo -e "${BLUE}   Monitoring Complete${NC}"
echo -e "${BLUE}======================================${NC}"
