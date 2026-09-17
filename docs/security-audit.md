# OWASP SAMM / DSOMM audit

This is a lightweight, repository-scoped review, not a formal certification.
It records controls visible in this repository and does not claim an external
audit.

## Current architecture and controls

- Docker and the Home Assistant add-on use one statically linked Go binary and
  one multi-architecture image. The add-on reads `/data/options.json` natively;
  it no longer carries a second module or shell/JQ configuration path.
- The runtime image is based on pinned Alpine 3.24.1 and installs CA
  certificates. It starts as root only to read Home Assistant's mode-0600
  `/data/options.json`, then clears supplementary groups and drops permanently
  to fixed UID/GID 10001 before MQTT or HTTP activity. No extra file-bypass
  capability is requested. Compose demonstrates direct non-root execution.
- Configuration validation rejects invalid topics and intervals, and password
  values are redacted from `Config.String` and option parse errors.
- HTTP requests have timeouts, status checks, size limits, and cancellation;
  MQTT availability and reconnect behavior are explicit.
- CI runs formatting, vet, builds, module verification, race-enabled tests, and
  a multi-architecture Docker build. The release workflow is tag-only,
  validates the tag against the add-on version, and grants only
  `contents: read` and `packages: write` to its release job.

## Residual risks

- MQTT transport security is deployment-configured. Use TLS broker URLs and
  authentication in production.
- The upstream document is forwarded as data. Consumers should treat it as
  untrusted input.
- This repository contains tests and checks but no independent penetration test
  or signed release review; none is claimed here.
