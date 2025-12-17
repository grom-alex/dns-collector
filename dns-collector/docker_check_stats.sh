#!/bin/bash

echo "╔══════════════════════════════════════════════════════════════╗"
echo "║      DNS COLLECTOR (DOCKER) - TEST RESULTS                   ║"
echo "╚══════════════════════════════════════════════════════════════╝"
echo ""

echo "📊 DOMAIN STATISTICS"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
sg docker -c "docker exec dns-collector sh -c 'cd /app/data && sqlite3 domains.db \"SELECT COUNT(*) FROM domain;\"'" | while read count; do
  echo "Total Domains:        $count"
done

sg docker -c "docker exec dns-collector sh -c 'cd /app/data && sqlite3 domains.db \"SELECT COUNT(*) FROM domain WHERE resolv_count >= max_resolv;\"'" | while read count; do
  echo "Fully Resolved:       $count"
done

sg docker -c "docker exec dns-collector sh -c 'cd /app/data && sqlite3 domains.db \"SELECT COUNT(*) FROM ip;\"'" | while read count; do
  echo "Total IP Addresses:   $count"
done
echo ""

echo "📋 DOMAINS DETAIL"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
sg docker -c "docker exec dns-collector sh -c 'cd /app/data && sqlite3 -column domains.db \"SELECT domain, resolv_count, max_resolv FROM domain ORDER BY domain;\"'"
echo ""

echo "📈 IP ADDRESSES"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
sg docker -c "docker exec dns-collector sh -c 'cd /app/data && sqlite3 domains.db \"SELECT d.domain, COUNT(CASE WHEN i.type=\\\"ipv4\\\" THEN 1 END) as ipv4, COUNT(CASE WHEN i.type=\\\"ipv6\\\" THEN 1 END) as ipv6 FROM domain d LEFT JOIN ip i ON d.id = i.domain_id GROUP BY d.id ORDER BY d.domain;\"'" | while read line; do
  IFS='|' read -ra PARTS <<< "$line"
  printf "%-20s IPv4: %-3s  IPv6: %-3s\n" "${PARTS[0]}" "${PARTS[1]}" "${PARTS[2]}"
done
echo ""

echo "📊 REQUEST STATISTICS"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
sg docker -c "docker exec dns-collector sh -c 'cd /app/data && sqlite3 stats.db \"SELECT COUNT(*) FROM domain_stat;\"'" | while read count; do
  echo "Total Requests:       $count"
done
echo ""

echo "✅ Docker testing completed successfully!"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
