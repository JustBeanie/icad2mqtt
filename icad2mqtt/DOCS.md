# ICAD to MQTT Bridge

This Home Assistant add-on uses the published multi-architecture image
`ghcr.io/justbeanie/icad2mqtt:2.0.0`. The same binary is used by Docker and by
the add-on; it reads `/data/options.json` directly when that file exists.

The add-on options keep the `mqtt_broker`, `mqtt_topic`, `mqtt_base_topic`,
`publish_raw`, `ha_discovery`, `mqtt_username`, `mqtt_password`, and
`poll_interval` names. Options-file values take precedence over environment
variables. A missing or null option uses the documented default, while an
explicit false or zero is preserved and then validated by the application.

Home Assistant stores `options.json` as mode 0600. The image therefore starts
as root only long enough to load and validate the options, then clears
supplementary groups and drops permanently to fixed UID/GID `10001` before any
MQTT or HTTP activity. The service itself runs unprivileged after startup.

For local Docker use, `docker-compose.yml` sets `user: "10001:10001"` as a
hardening option. In that mode `/data/options.json` is not relevant and the
binary skips the already-non-root drop.
