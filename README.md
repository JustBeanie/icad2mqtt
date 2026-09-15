# ICAD to MQTT Bridge

A small Go service that fetches the current CAD events document and publishes
only changed responses over MQTT. It is designed to run as a root Docker
container build or a Home Assistant add-on.

## Quick Start

```bash
git clone https://github.com/JustBeanie/icad2mqtt.git
cd icad2mqtt
docker-compose up
```

## Features

- Polls `https://911events.ongov.net/CADInet/app/events.jsp` for active calls
- Publishes updates to an MQTT broker
- Only sends updates when data changes
- Automatic MQTT reconnection with retry backoff
- Configurable polling interval
- Graceful shutdown on SIGINT/SIGTERM
- Bounded HTTP requests and response size
- Non-root Docker runtime
- Docker and Home Assistant add-on support

## Configuration

Via environment variables:
| Variable | Default | Description |
|---|---|---|
| `MQTT_BROKER` | `tcp://localhost:1883` | MQTT broker address |
| `MQTT_BASE_TOPIC` | `911/cad` | Base for structured MQTT topics |
| `MQTT_TOPIC` | `911/cad/events` | Raw HTML topic |
| `PUBLISH_RAW` | `true` | Publish raw HTML on change |
| `HA_DISCOVERY` | `false` | Enable Home Assistant discovery (reserved for publishing integration) |
| `MQTT_USERNAME` | empty | Optional MQTT username |
| `MQTT_PASSWORD` | empty | Optional MQTT password; never logged or printed |
| `CLIENT_ID` | `icad2mqtt` | MQTT client ID |
| `POLL_INTERVAL` | `60` seconds | Poll interval; values below 60 are clamped to 60 with one warning |
| `HTTP_TIMEOUT` | `15` seconds | HTTP timeout, restricted to 5–60 seconds |
| `HTTP_USER_AGENT` | `icad2mqtt/1.0` | HTTP User-Agent |

## Running Standalone

### Prerequisites
- Go 1.21 or later
- MQTT broker (e.g., Mosquitto)

```bash
go mod download
go build -trimpath -o icad2mqtt
./icad2mqtt
```

Run the same checks used by CI with `go test -race ./...`, `go vet ./...`, and
`gofmt -l .` (which must produce no output).

## Running with Docker

### Quick Start
```bash
docker-compose up
```

This starts both the app and Mosquitto MQTT broker.

### Build Custom Image
```bash
docker build --pull -t icad2mqtt:local .
docker run -e MQTT_BROKER=tcp://your-broker:1883 icad2mqtt
```

The Dockerfile and Docker build context are at the repository root. The image
runs as a non-root user and contains only the compiled service and CA
certificates.

### Environment Variables in Docker
```bash
docker run \
  -e MQTT_BROKER=tcp://broker.example.com:1883 \
  -e MQTT_TOPIC=911/events \
  -e POLL_INTERVAL=60 \
  icad2mqtt
```

## Home Assistant Add-on

### Installation

1. Add the repository to Home Assistant:
   - Settings → Add-ons → Create add-on repository
   - URL: `https://github.com/JustBeanie/icad2mqtt`

2. Install ICAD to MQTT Bridge from the add-on store

3. Configure in the add-on options:
   ```json
   {
     "mqtt_broker": "tcp://localhost:1883",
     "mqtt_topic": "911/cad/events",
     "mqtt_base_topic": "911/cad",
     "publish_raw": true,
     "ha_discovery": false,
     "mqtt_username": "",
     "mqtt_password": "",
     "poll_interval": 60
   }
   ```

4. Start the add-on

### Notes
- Requires MQTT broker running (built-in or separate)
- Publishes to configured topic with QoS 1
- Raw publishing is enabled by default and can be disabled with `publish_raw`.
- Add-on options map to the environment variables above; the password is never logged.
- Automatically restarts on failure

## Building Home Assistant Add-on

To build and test locally:
```bash
docker build -f icad2mqtt/Dockerfile -t icad2mqtt-addon .
```

To test add-on option extraction on a POSIX host with `jq`, run
`sh icad2mqtt/run_test.sh`.

## Project Structure

```
icad2mqtt/
├── main.go              # Main application
├── go.mod/go.sum        # Go dependencies
├── Dockerfile           # Docker image for standalone
├── docker-compose.yml   # Docker Compose with MQTT
├── repository.yaml      # Home Assistant repository config
├── icad2mqtt/           # Home Assistant add-on
│   ├── config.yaml      # Add-on configuration
│   ├── run.sh           # Add-on startup script
│   └── Dockerfile       # Add-on-specific Dockerfile
└── README.md            # This file
```

## Dependencies

- `github.com/eclipse/paho.mqtt.golang` - MQTT client library

## Logging

The application outputs logs to stdout showing:
- Connection status
- Configuration details
- Fetch errors
- MQTT publish events

Broker credentials are not logged. Do not put secrets in committed `.env` files.

## Security

See [SECURITY.md](SECURITY.md) for reporting guidance and
[SECURITY_AUDIT.md](SECURITY_AUDIT.md) for the current OWASP SAMM/DSOMM-oriented
repository audit and residual risks.

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

MIT License - see [LICENSE](LICENSE) file for details
