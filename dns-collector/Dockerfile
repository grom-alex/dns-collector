# Multi-stage build для минимального размера образа

# Stage 1: Build
FROM golang:1.21-bullseye AS builder

# Установка необходимых пакетов для сборки
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /build

# Копируем go.mod и go.sum для кэширования зависимостей
COPY go.mod go.sum ./
RUN go mod download

# Копируем исходный код
COPY . .

# Сборка приложения
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o dns-collector ./cmd/dns-collector

# Stage 2: Runtime
FROM debian:bullseye-slim

# Установка runtime зависимостей
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

# Копируем бинарник из builder stage
COPY --from=builder /build/dns-collector .

# Копируем конфигурационный файл
COPY config/ ./config/

# Создаем пользователя для запуска приложения
RUN groupadd -g 1000 dnscollector && \
    useradd -r -u 1000 -g dnscollector dnscollector && \
    chown -R dnscollector:dnscollector /app

USER dnscollector

# Expose UDP port
EXPOSE 5353/udp

# Запуск приложения
ENTRYPOINT ["./dns-collector"]
CMD ["-config", "/app/config/config.yaml"]
