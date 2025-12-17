package main

import (
	"database/sql"
	"fmt"
	"log"

	_ "github.com/mattn/go-sqlite3"
)

func main() {
	// Check domains.db
	fmt.Println("=== Checking domains.db ===")
	db, err := sql.Open("sqlite3", "domains.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// Check domain table
	fmt.Println("\n--- Domain Table ---")
	rows, err := db.Query("SELECT id, domain, datetime(time_insert), resolv_count, max_resolv FROM domain")
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	for rows.Next() {
		var id int
		var domain, timeInsert string
		var resolvCount, maxResolv int
		rows.Scan(&id, &domain, &timeInsert, &resolvCount, &maxResolv)
		fmt.Printf("ID: %d, Domain: %s, Inserted: %s, Count: %d/%d\n",
			id, domain, timeInsert, resolvCount, maxResolv)
	}

	// Check IP table
	fmt.Println("\n--- IP Table ---")
	rows2, err := db.Query(`
		SELECT d.domain, i.ip, i.type, datetime(i.time)
		FROM domain d
		JOIN ip i ON d.id = i.domain_id
		ORDER BY d.domain, i.type
	`)
	if err != nil {
		log.Fatal(err)
	}
	defer rows2.Close()

	count := 0
	for rows2.Next() {
		var domain, ip, ipType, time string
		rows2.Scan(&domain, &ip, &ipType, &time)
		fmt.Printf("%s -> %s (%s) at %s\n", domain, ip, ipType, time)
		count++
	}

	if count == 0 {
		fmt.Println("No IP addresses resolved yet")
	}

	// Check stats.db
	fmt.Println("\n=== Checking stats.db ===")
	statsDB, err := sql.Open("sqlite3", "stats.db")
	if err != nil {
		log.Fatal(err)
	}
	defer statsDB.Close()

	fmt.Println("\n--- Statistics ---")
	var totalStats int
	statsDB.QueryRow("SELECT COUNT(*) FROM domain_stat").Scan(&totalStats)
	fmt.Printf("Total requests: %d\n", totalStats)

	rows3, err := statsDB.Query(`
		SELECT domain, COUNT(*) as count
		FROM domain_stat
		GROUP BY domain
		ORDER BY count DESC
	`)
	if err != nil {
		log.Fatal(err)
	}
	defer rows3.Close()

	fmt.Println("\nRequests per domain:")
	for rows3.Next() {
		var domain string
		var count int
		rows3.Scan(&domain, &count)
		fmt.Printf("  %s: %d requests\n", domain, count)
	}
}
