# Build stage
FROM golang:1.21-alpine AS builder

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY *.go ./

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o icad2mqtt .

# Final stage
FROM alpine:latest

# Install ca-certificates and jq for config parsing
RUN apk --no-cache add ca-certificates jq

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/icad2mqtt .

# Copy run script
COPY addon/run.sh .
RUN chmod +x run.sh

# Create non-root user
RUN addgroup -D appgroup && adduser -D appuser -G appgroup
USER appuser

# Run the application
CMD ["./run.sh"]
