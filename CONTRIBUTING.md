# Contributing to ICAD to MQTT Bridge

Thank you for your interest in contributing! This document provides guidelines for contributing to this project.

## Getting Started

1. Fork the repository
2. Clone your fork: `git clone https://github.com/YOUR_USERNAME/icad2mqtt.git`
3. Create a branch: `git checkout -b feature/your-feature-name`
4. Make your changes
5. Test your changes
6. Commit your changes: `git commit -m "Add some feature"`
7. Push to your branch: `git push origin feature/your-feature-name`
8. Open a Pull Request

## Development Setup

### Prerequisites
- Go 1.21 or later
- Docker (optional, for testing with docker-compose)
- MQTT broker (for testing)

### Building

```bash
go mod download
go build -o icad2mqtt
```

### Testing

```bash
go test ./...
```

### Running Locally

```bash
export MQTT_BROKER=tcp://localhost:1883
export MQTT_TOPIC=911/cad/events
export POLL_INTERVAL=30
./icad2mqtt
```

## Code Style

- Follow Go conventions and best practices
- Use `gofmt` to format your code
- Add comments for exported functions and types
- Keep functions focused and small

## Commit Messages

- Use clear, descriptive commit messages
- Reference issues when applicable (e.g., "Fix #123")

## Pull Request Process

1. Ensure your code builds and tests pass
2. Update documentation if needed
3. Add tests for new features
4. Ensure all CI checks pass
5. Request review from maintainers

## Questions?

Feel free to open an issue for questions or discussions.