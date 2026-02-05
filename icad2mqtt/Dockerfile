# Build stage
FROM golang:1.21-alpine AS builder

WORKDIR /app

# Copy go mod files
COPY go.mod ./
COPY go.sum* ./

# Download dependencies
RUN go mod download

# Copy source code
COPY *.go ./

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o icad2mqtt .

# Final stage - use Home Assistant base image
ARG BUILD_FROM
FROM ${BUILD_FROM}

# Install ca-certificates and jq for config parsing
RUN apk add --no-cache ca-certificates jq

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/icad2mqtt .

# Copy run script
COPY run.sh /run.sh
RUN chmod a+x /run.sh

# Run the application
CMD [ "/run.sh" ]
