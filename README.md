# ICAD to MQTT Bridge

The service fetches the current CAD events document and publishes changes over
MQTT. It emits structured incident, availability, health, and optional Home
Assistant discovery topics, plus the raw compatibility topic.

## Prerequisites

Go 1.26 or later is required to build and test the project. Docker and an MQTT
broker are optional for local container and integration testing.

## Configuration

Docker uses environment variables. The Home Assistant add-on uses the same
binary and reads `/data/options.json`; options-file values win when both are
present. Missing and null options use the defaults below.

| Variable / option | Default |
| --- | --- |
| `MQTT_BROKER` / `mqtt_broker` | `tcp://localhost:1883` |
| `MQTT_BASE_TOPIC` / `mqtt_base_topic` | `911/cad` |
| `MQTT_TOPIC` / `mqtt_topic` | `911/cad/events` |
| `PUBLISH_RAW` / `publish_raw` | `true` |
| `HA_DISCOVERY` / `ha_discovery` | `false` |
| `MQTT_USERNAME` / `mqtt_username` | empty |
| `MQTT_PASSWORD` / `mqtt_password` | empty |
| `POLL_INTERVAL` / `poll_interval` | `60` seconds |

`CLIENT_ID`, `HTTP_TIMEOUT`, and `HTTP_USER_AGENT` remain environment-only
Docker settings. Passwords are never included in configuration strings or
validation errors.

## Docker

```sh
docker compose up
docker build -t icad2mqtt:local .
docker run -e MQTT_BROKER=tcp://broker.example.com:1883 icad2mqtt:local
```

The root `Dockerfile` builds a static `linux/amd64` or `linux/arm64` image and
uses a minimal `scratch` runtime with an explicitly copied CA bundle. The
binary starts as root only to read add-on options, then drops permanently to
UID/GID 10001 before network activity; Compose shows how to run it directly as
that unprivileged user.

## Home Assistant add-on

Install the add-on from the repository and configure its existing options.
Version 2.0.0 uses `ghcr.io/justbeanie/icad2mqtt`, a multi-architecture image,
for both Docker and Home Assistant. Supported architectures are `aarch64` and
`amd64`; deprecated 32-bit architectures were dropped.

See [icad2mqtt/DOCS.md](icad2mqtt/DOCS.md), [icad2mqtt/CHANGELOG.md](icad2mqtt/CHANGELOG.md),
and the [MQTT contract](docs/contract.md).

## Project structure

```text
main.go                    application wiring
internal/                  configuration, fetch, parse, normalize, publish
Dockerfile                 shared multi-architecture image
docker-compose.yml         local service and Mosquitto broker
icad2mqtt/config.yaml      Home Assistant add-on metadata
docs/contract.md           structured MQTT contract
```

## Checks

Run `just check` for tests, vet, formatting, and the root build. CI also runs
the race-enabled test suite and module verification.

Security reporting is described in [SECURITY.md](SECURITY.md); the repository
audit is [docs/security-audit.md](docs/security-audit.md).
