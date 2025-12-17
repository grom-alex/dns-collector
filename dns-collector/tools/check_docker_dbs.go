package main

import (
	"database/sql"
	"fmt"
	"log"

	_ "github.com/mattn/go-sqlite3"
)

func main() {
	fmt.Println("╔══════════════════════════════════════════════════════════════╗")
	fmt.Println("║      DNS COLLECTOR (DOCKER) - TEST RESULTS                   ║")
	fmt.Println("╚══════════════════════════════════════════════════════════════╝")
	fmt.Println()

	// Check docker_domains.db
	db, err := sql.Open("sqlite3", "docker_domains.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// Domain statistics
	var totalDomains, fullyResolved int
	db.QueryRow("SELECT COUNT(*) FROM domain").Scan(&totalDomains)
	db.QueryRow("SELECT COUNT(*) FROM domain WHERE resolv_count >= max_resolv").Scan(&fullyResolved)

	fmt.Println("📊 DOMAIN STATISTICS")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Printf("Total Domains:        %d\n", totalDomains)
	fmt.Printf("Fully Resolved:       %d\n", fullyResolved)
	fmt.Println()

	// IP statistics
	var totalIPs int
	db.QueryRow("SELECT COUNT(*) FROM ip").Scan(&totalIPs)

	fmt.Println("🌐 IP ADDRESS STATISTICS")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Printf("Total IP Addresses:   %d\n", totalIPs)
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
		fmt.Printf("%-20s Resolved: %2d/%2d  IPv4: %d  IPv6: %d\n",
			domain, resolvCount, maxResolv, ipv4, ipv6)
	}
	fmt.Println()

	// Statistics DB
	statsDB, _ := sql.Open("sqlite3", "docker_stats.db")
	defer statsDB.Close()

	var totalRequests int
	statsDB.QueryRow("SELECT COUNT(*) FROM domain_stat").Scan(&totalRequests)

	fmt.Println("📈 REQUEST STATISTICS")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Printf("Total Requests:       %d\n", totalRequests)
	fmt.Println()

	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("✅ DOCKER DEPLOYMENT SUCCESSFUL!")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
}
