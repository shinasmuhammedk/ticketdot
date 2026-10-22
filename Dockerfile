# ==========================================
# Multi-stage Dockerfile for TicketDot Backend
# ==========================================

# 1. Build Stage
FROM golang:1.22-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Cache Go modules
COPY go.mod go.sum ./
RUN go mod download

# Copy backend source code
COPY . .

# Build statically linked binary with stripped debug info
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /bin/api ./cmd/api/main.go
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /bin/seed ./cmd/seed/main.go

# 2. Production Runtime Stage (Minimal Alpine with Non-Root User)
FROM alpine:3.19 AS runtime

# Install certificates for outbound HTTPS requests (e.g. payment gateway)
RUN apk --no-cache add ca-certificates tzdata

# Create dedicated non-root application user and group
RUN addgroup -S appgroup && adduser -S appuser -G appgroup

WORKDIR /home/appuser

# Copy compiled binaries from builder stage
COPY --from=builder /bin/api /usr/local/bin/api
COPY --from=builder /bin/seed /usr/local/bin/seed

# Change ownership to non-root user
RUN chown -R appuser:appgroup /home/appuser /usr/local/bin/api /usr/local/bin/seed

# Switch to non-root user
USER appuser

EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/api"]
