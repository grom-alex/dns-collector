.PHONY: help dev build clean install test docker-build docker-run

help:
	@echo "DNS Collector Web API - Makefile commands:"
	@echo ""
	@echo "Development:"
	@echo "  make dev              - Run backend in development mode"
	@echo "  make dev-frontend     - Run frontend dev server"
	@echo "  make install          - Install all dependencies"
	@echo ""
	@echo "Build:"
	@echo "  make build            - Build backend binary"
	@echo "  make build-frontend   - Build frontend for production"
	@echo "  make build-all        - Build both backend and frontend"
	@echo ""
	@echo "Docker:"
	@echo "  make docker-build     - Build Docker image"
	@echo "  make docker-run       - Run Docker container"
	@echo ""
	@echo "Other:"
	@echo "  make clean            - Clean build artifacts"
	@echo "  make test             - Run tests"

# Development
dev:
	go run cmd/main.go -config config/config.yaml

dev-frontend:
	cd frontend && npm run dev

# Installation
install:
	go mod download
	cd frontend && npm install

# Build
build:
	CGO_ENABLED=1 go build -o bin/web-api ./cmd/main.go

build-frontend:
	cd frontend && npm run build

build-all: build-frontend build

# Docker
docker-build:
	docker build -t dns-collector-webapi:latest .

docker-run:
	docker run -d \
		-p 8080:8080 \
		-v $$(pwd)/../data:/app/data:ro \
		--name dns-collector-webapi \
		dns-collector-webapi:latest

# Clean
clean:
	rm -rf bin/
	rm -rf frontend/dist/
	rm -rf frontend/node_modules/
	go clean

# Test
test:
	go test ./...
	cd frontend && npm run test
