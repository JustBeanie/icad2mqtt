# Build stage. Keep the build toolchain out of the runtime image.
FROM golang:1.23-alpine AS builder

WORKDIR /app

# Copy module metadata first so dependency downloads are cached.
COPY go.mod ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o icad2mqtt .

# Final stage
FROM alpine:3.20

# Install ca-certificates for HTTPS
RUN apk add --no-cache ca-certificates \
    && addgroup -S appgroup \
    && adduser -S -G appgroup -h /nonexistent -s /sbin/nologin appuser

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/icad2mqtt .

USER appuser

# The service only needs outbound HTTPS and MQTT connections.
ENV GODEBUG=netdns=go

# Run the application
CMD ["./icad2mqtt"]
