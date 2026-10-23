# Build stage 1: Frontend
FROM node:22-alpine AS frontend-builder
WORKDIR /app/web
COPY web/package*.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

# Build stage 2: Go binary
FROM golang:1.27-alpine AS backend-builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/ ./cmd/
COPY internal/ ./internal/
COPY --from=frontend-builder /app/web/dist ./web/dist
COPY web/embed.go ./web/embed.go
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o uptime-mon ./cmd/uptime-mon

# Final stage: Minimal runtime image
FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=backend-builder /app/uptime-mon /app/uptime-mon

# Expose HTTP port
EXPOSE 8080

# Volumes for declarative YAML configurations and persistent SQLite state
VOLUME ["/app/config", "/app/data"]

ENTRYPOINT ["/app/uptime-mon"]
CMD ["serve", "--config-dir", "/app/config", "--data-dir", "/app/data", "--port", "8080"]
