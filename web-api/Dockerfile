# Build stage for frontend
FROM node:20-alpine AS frontend-builder

WORKDIR /app/frontend
COPY frontend/package*.json ./
RUN npm install
COPY frontend/ ./
RUN npm run build

# Build stage for backend
FROM golang:1.21-alpine AS backend-builder

RUN apk add --no-cache gcc musl-dev sqlite-dev

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=1 GOOS=linux go build -tags="sqlite_omit_load_extension" -ldflags="-s -w" -o web-api ./cmd/main.go

# Final stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates sqlite wget

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
