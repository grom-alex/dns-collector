.PHONY: build run clean test deps

# Название бинарника
BINARY_NAME=dns-collector
BUILD_DIR=build

# Сборка программы
build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	go build -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/dns-collector
	@echo "Build complete: $(BUILD_DIR)/$(BINARY_NAME)"

# Запуск программы
run: build
	@echo "Starting $(BINARY_NAME)..."
	./$(BUILD_DIR)/$(BINARY_NAME)

# Запуск с указанием конфига
run-config: build
	@echo "Starting $(BINARY_NAME) with custom config..."
	./$(BUILD_DIR)/$(BINARY_NAME) -config $(CONFIG)

# Установка зависимостей
deps:
	@echo "Downloading dependencies..."
	go mod download
	go mod tidy

# Очистка
clean:
	@echo "Cleaning..."
	rm -rf $(BUILD_DIR)
	rm -f *.db
	@echo "Clean complete"

# Тест отправки сообщений
test:
	@echo "Running test client..."
	python3 test_client.py

# Сборка для разных платформ
build-linux:
	@echo "Building for Linux..."
	@mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=amd64 go build -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 ./cmd/dns-collector

build-windows:
	@echo "Building for Windows..."
	@mkdir -p $(BUILD_DIR)
	GOOS=windows GOARCH=amd64 go build -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe ./cmd/dns-collector

build-macos:
	@echo "Building for macOS..."
	@mkdir -p $(BUILD_DIR)
	GOOS=darwin GOARCH=amd64 go build -o $(BUILD_DIR)/$(BINARY_NAME)-macos-amd64 ./cmd/dns-collector

build-all: build-linux build-windows build-macos
	@echo "All builds complete"

# Помощь
help:
	@echo "Available targets:"
	@echo "  make build        - Build the application"
	@echo "  make run          - Build and run the application"
	@echo "  make run-config   - Run with custom config (use CONFIG=/path/to/config.yaml)"
	@echo "  make deps         - Download dependencies"
	@echo "  make clean        - Clean build artifacts and databases"
	@echo "  make test         - Run test client"
	@echo "  make build-all    - Build for all platforms"
	@echo "  make help         - Show this help"
