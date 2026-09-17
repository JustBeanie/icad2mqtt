# Build stage. Keep the build toolchain out of the runtime image.
FROM --platform=$BUILDPLATFORM golang:1.27-alpine AS builder

ARG TARGETOS
ARG TARGETARCH

WORKDIR /app

# Copy module metadata first so dependency downloads are cached.
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build -trimpath -o icad2mqtt .

# Final stage
FROM alpine:3.24.1

# Install ca-certificates for HTTPS
RUN apk add --no-cache ca-certificates \
    && addgroup -S -g 10001 appgroup \
    && adduser -S -D -u 10001 -G appgroup -h /nonexistent -s /sbin/nologin appuser

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/icad2mqtt .

# The service only needs outbound HTTPS and MQTT connections.
ENV GODEBUG=netdns=go

# Run the application
ENTRYPOINT ["/app/icad2mqtt"]
