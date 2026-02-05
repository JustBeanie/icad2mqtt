# ICAD to MQTT Bridge

A lightweight Go application that fetches active calls from the 911 events endpoint and publishes updates over MQTT.

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
- Automatic MQTT reconnection with exponential backoff
- Configurable polling interval
- Docker and Home Assistant add-on support

## Configuration

Via environment variables:
- `MQTT_BROKER`: MQTT broker address (default: `tcp://localhost:1883`)
- `MQTT_TOPIC`: MQTT topic for publishing events (default: `911/cad/events`)
- `CLIENT_ID`: MQTT client ID (default: `icad2mqtt`)
- `POLL_INTERVAL`: Polling interval in seconds (default: `30`)

## Running Standalone

### Prerequisites
- Go 1.21 or later
- MQTT broker (e.g., Mosquitto)

```bash
go mod download
go build -o icad2mqtt
./icad2mqtt
```

## Running with Docker

### Quick Start
```bash
docker-compose up
```

This starts both the app and Mosquitto MQTT broker.

### Build Custom Image
```bash
docker build -t icad2mqtt .
docker run -e MQTT_BROKER=tcp://your-broker:1883 icad2mqtt
```

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
     "poll_interval": 30
   }
   ```

4. Start the add-on

### Notes
- Requires MQTT broker running (built-in or separate)
- Publishes to configured topic with QoS 1
- Automatically restarts on failure

## Building Home Assistant Add-on

To build and test locally:
```bash
docker build -f addon/Dockerfile -t icad2mqtt-addon .
```

## Project Structure

```
icad2mqtt/
├── main.go              # Main application
├── go.mod/go.sum        # Go dependencies
├── Dockerfile           # Docker image for standalone
├── docker-compose.yml   # Docker Compose with MQTT
├── addon/
│   ├── addon.json       # Home Assistant manifest
│   ├── run.sh           # Addon startup script
│   └── Dockerfile       # Addon-specific Dockerfile
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

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

MIT License - see [LICENSE](LICENSE) file for details
