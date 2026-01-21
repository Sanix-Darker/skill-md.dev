# Build stage
FROM golang:1.23-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o skillmd ./cmd/skillmd

# Runtime stage
FROM alpine:3.19

WORKDIR /app

# Install ca-certificates for HTTPS
RUN apk add --no-cache ca-certificates

# Copy binary from builder
COPY --from=builder /app/skillmd /usr/local/bin/skillmd

# Create non-root user and data directory
RUN addgroup -g 1000 skillmd && \
    adduser -u 1000 -G skillmd -s /sbin/nologin -D skillmd && \
    mkdir -p /data && \
    chown -R skillmd:skillmd /data

# Expose port
EXPOSE 8080

# Set environment variables
ENV SKILLMD_DB=/data/skill-md.db
ENV SKILLMD_PORT=8080

# Switch to non-root user
USER skillmd:skillmd

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/ || exit 1

# Run server
ENTRYPOINT ["skillmd"]
CMD ["serve", "--port", "8080", "--db", "/data/skill-md.db"]
