# Build stage for frontend
FROM --platform=$BUILDPLATFORM node:20-alpine AS frontend-builder

WORKDIR /app/frontend
COPY frontend/package*.json ./
RUN npm install
COPY frontend/ ./
RUN npm run build

# Build stage for backend
FROM --platform=$BUILDPLATFORM golang:1.24-bookworm AS backend-builder

ARG TARGETOS=linux
ARG TARGETARCH=amd64

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -ldflags="-s -w" -o web-api ./cmd/main.go

# Final stage
FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    wget \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

# Copy backend binary
COPY --from=backend-builder /app/web-api .

# Copy frontend build
COPY --from=frontend-builder /app/frontend/dist ./frontend/dist

# Copy config
COPY config/config.yaml ./config/

# Create data directory
RUN mkdir -p /app/data

EXPOSE 8080

CMD ["./web-api", "-config", "config/config.yaml"]
