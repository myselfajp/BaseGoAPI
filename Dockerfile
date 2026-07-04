# ---- Build stage ----
FROM golang:1.23-alpine AS builder

WORKDIR /app

# Cache dependencies first.
COPY go.mod go.sum ./
RUN go mod download

# Build the statically-linked binary (migrations are embedded via go:embed).
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /app/server ./cmd/server

# ---- Runtime stage ----
FROM alpine:3.20

# Non-root user and TLS roots + curl for the health check.
RUN apk add --no-cache ca-certificates curl \
    && adduser -D -h /app app

WORKDIR /app
COPY --from=builder /app/server /app/server

USER app

EXPOSE 8000

HEALTHCHECK --interval=30s --timeout=30s --start-period=5s --retries=3 \
    CMD curl -f http://localhost:8000/v1/health || exit 1

CMD ["/app/server"]
