# Add-on output limitation

This directory is a separate Go module and intentionally publishes the legacy
raw HTML topic only. It cannot import the root module's `internal/` packages.
Structured schema v1 topics, health, counts, and Home Assistant discovery
require running the main icad2mqtt binary (for example the Docker image).
Setting `HA_DISCOVERY=true` emits a startup warning. Add-on support is planned.
The add-on's raw topic behavior remains unchanged.
