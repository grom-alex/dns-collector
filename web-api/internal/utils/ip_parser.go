package utils

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"strings"
)

// ParseIPsFromFile reads a file and extracts valid IPv4 and IPv6 addresses
// File format: one IP address per line, empty lines and comments (#) are ignored
func ParseIPsFromFile(filepath string) ([]string, []string, error) {
	file, err := os.Open(filepath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			fmt.Printf("Warning: failed to close file %s: %v\n", filepath, closeErr)
		}
	}()

	var ipv4List []string
	var ipv6List []string
	lineNum := 0

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse and validate IP address
		ip := net.ParseIP(line)
		if ip == nil {
			// Log warning but continue processing
			fmt.Printf("Warning: invalid IP address on line %d: %s\n", lineNum, line)
			continue
		}

		// Determine IP type
		if ip.To4() != nil {
			// IPv4 address
			ipv4List = append(ipv4List, line)
		} else {
			// IPv6 address
			ipv6List = append(ipv6List, line)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, nil, fmt.Errorf("error reading file: %w", err)
	}

	return ipv4List, ipv6List, nil
}

// ValidateIPAddress checks if a string is a valid IP address
func ValidateIPAddress(ip string) bool {
	return net.ParseIP(ip) != nil
}

// IsIPv4 checks if an IP address is IPv4
func IsIPv4(ip string) bool {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return false
	}
	return parsed.To4() != nil
}

// IsIPv6 checks if an IP address is IPv6
func IsIPv6(ip string) bool {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return false
	}
	return parsed.To4() == nil
}
