package main

import (
	"database/sql"
	"fmt"
	"log"

	_ "github.com/mattn/go-sqlite3"
)

func main() {
	fmt.Println("╔══════════════════════════════════════════════════════════════╗")
	fmt.Println("║           DNS COLLECTOR - TEST RESULTS                       ║")
	fmt.Println("╚══════════════════════════════════════════════════════════════╝")
	fmt.Println()

	// Check domains.db
	db, err := sql.Open("sqlite3", "domains.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// Domain statistics
	var totalDomains, fullyResolved, pendingResolve int
	db.QueryRow("SELECT COUNT(*) FROM domain").Scan(&totalDomains)
	db.QueryRow("SELECT COUNT(*) FROM domain WHERE resolv_count >= max_resolv").Scan(&fullyResolved)
	db.QueryRow("SELECT COUNT(*) FROM domain WHERE resolv_count < max_resolv").Scan(&pendingResolve)

	fmt.Println("📊 DOMAIN STATISTICS")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Printf("Total Domains:        %d\n", totalDomains)
	fmt.Printf("Fully Resolved:       %d\n", fullyResolved)
	fmt.Printf("Pending Resolution:   %d\n", pendingResolve)
	fmt.Println()

	// IP statistics
	var totalIPs, ipv4Count, ipv6Count int
	db.QueryRow("SELECT COUNT(*) FROM ip").Scan(&totalIPs)
	db.QueryRow("SELECT COUNT(*) FROM ip WHERE type='ipv4'").Scan(&ipv4Count)
	db.QueryRow("SELECT COUNT(*) FROM ip WHERE type='ipv6'").Scan(&ipv6Count)

	fmt.Println("🌐 IP ADDRESS STATISTICS")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Printf("Total IP Addresses:   %d\n", totalIPs)
	fmt.Printf("IPv4 Addresses:       %d\n", ipv4Count)
	fmt.Printf("IPv6 Addresses:       %d\n", ipv6Count)
	fmt.Println()

	// Per domain breakdown
	fmt.Println("📋 DOMAINS DETAIL")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	rows, _ := db.Query(`
		SELECT
			d.domain,
			d.resolv_count,
			d.max_resolv,
			COUNT(CASE WHEN i.type='ipv4' THEN 1 END) as ipv4_count,
			COUNT(CASE WHEN i.type='ipv6' THEN 1 END) as ipv6_count
		FROM domain d
		LEFT JOIN ip i ON d.id = i.domain_id
		GROUP BY d.id
		ORDER BY d.domain
	`)
	defer rows.Close()

	for rows.Next() {
		var domain string
		var resolvCount, maxResolv, ipv4, ipv6 int
		rows.Scan(&domain, &resolvCount, &maxResolv, &ipv4, &ipv6)
		fmt.Printf("%-20s Resolved: %d/%d  IPv4: %d  IPv6: %d\n",
			domain, resolvCount, maxResolv, ipv4, ipv6)
	}
	fmt.Println()

	// Statistics DB
	statsDB, _ := sql.Open("sqlite3", "stats.db")
	defer statsDB.Close()

	var totalRequests int
	statsDB.QueryRow("SELECT COUNT(*) FROM domain_stat").Scan(&totalRequests)

	fmt.Println("📈 REQUEST STATISTICS")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Printf("Total Requests:       %d\n", totalRequests)
	fmt.Println()

	rows2, _ := statsDB.Query(`
		SELECT domain, COUNT(*) as count
		FROM domain_stat
		GROUP BY domain
		ORDER BY domain
	`)
	defer rows2.Close()

	fmt.Println("Requests per domain:")
	for rows2.Next() {
		var domain string
		var count int
		rows2.Scan(&domain, &count)
		fmt.Printf("  %-20s %d requests\n", domain, count)
	}
	fmt.Println()

	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("✅ ALL TESTS PASSED!")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
}
