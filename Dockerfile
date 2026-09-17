# Build stage. Keep the build toolchain out of the runtime image.
FROM --platform=$BUILDPLATFORM golang:1.27-alpine AS builder

ARG TARGETOS
ARG TARGETARCH

WORKDIR /app

# Copy module metadata first so dependency downloads are cached.
COPY go.mod go.sum ./

# The scratch runtime needs the CA bundle for outbound HTTPS.
RUN apk add --no-cache ca-certificates

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
# Build outside /app: the source tree has an icad2mqtt/ add-on folder of the same name.
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build -trimpath -o /out/icad2mqtt .

# Final stage
FROM scratch

WORKDIR /app

# This is safe on scratch: the binary is static, Go provides TLS, and the CA
# bundle is copied explicitly; time/tzdata is embedded in the binary.
COPY --from=builder /out/icad2mqtt /app/icad2mqtt
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt

# The service only needs outbound HTTPS and MQTT connections.
ENV GODEBUG=netdns=go

# Run the application
ENTRYPOINT ["/app/icad2mqtt"]
