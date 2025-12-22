.PHONY: test test-dns-collector test-web-api test-coverage test-verbose clean build help

# Default target
help:
	@echo "Available targets:"
	@echo "  make test              - Run all tests"
	@echo "  make test-dns-collector - Run dns-collector tests only"
	@echo "  make test-web-api      - Run web-api tests only"
	@echo "  make test-coverage     - Run tests with coverage report"
	@echo "  make test-verbose      - Run tests with verbose output"
	@echo "  make build             - Build all services"
	@echo "  make clean             - Clean build artifacts"

# Run all tests
test:
	@echo "Running dns-collector tests..."
	@cd dns-collector && go test ./internal/... ./cmd/... -timeout 30s
	@echo "\nRunning web-api tests..."
	@cd web-api && go test ./internal/... ./cmd/... -timeout 30s
	@echo "\n✅ All tests passed!"

# Run dns-collector tests
test-dns-collector:
	@echo "Running dns-collector tests..."
	@cd dns-collector && go test ./internal/... ./cmd/... -v -timeout 30s

# Run web-api tests
test-web-api:
	@echo "Running web-api tests..."
	@cd web-api && go test ./internal/... ./cmd/... -v -timeout 30s

# Run tests with coverage
test-coverage:
	@echo "Running dns-collector tests with coverage..."
	@cd dns-collector && go test ./internal/... ./cmd/... -coverprofile=coverage.out -timeout 30s
	@cd dns-collector && go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: dns-collector/coverage.html"
	@echo "\nRunning web-api tests with coverage..."
	@cd web-api && go test ./internal/... ./cmd/... -coverprofile=coverage.out -timeout 30s
	@cd web-api && go tool cover -html=coverage.html -o coverage.html
	@echo "Coverage report: web-api/coverage.html"

# Run tests with verbose output
test-verbose:
	@echo "Running dns-collector tests (verbose)..."
	@cd dns-collector && go test ./internal/... ./cmd/... -v -timeout 30s
	@echo "\nRunning web-api tests (verbose)..."
	@cd web-api && go test ./internal/... ./cmd/... -v -timeout 30s

# Build all services
build:
	@echo "Building dns-collector..."
	@cd dns-collector && go build -o bin/dns-collector ./cmd/dns-collector
	@echo "Building web-api..."
	@cd web-api && go build -o bin/web-api ./cmd/main.go
	@echo "\n✅ Build completed!"

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	@rm -rf dns-collector/bin dns-collector/coverage.out dns-collector/coverage.html
	@rm -rf web-api/bin web-api/coverage.out web-api/coverage.html
	@echo "✅ Clean completed!"
