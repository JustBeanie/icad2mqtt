# Build arguments - must be declared before first FROM
ARG BUILD_FROM=ghcr.io/home-assistant/amd64-base:latest

# Build stage
FROM golang:1.21-alpine AS builder

WORKDIR /app

# Copy module metadata and source. The add-on is built from this directory.
COPY go.mod ./
COPY *.go ./

# Download dependencies without changing the module definition during a build.
RUN go mod download

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o icad2mqtt .

# Final stage - use Home Assistant base image
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
