#!/usr/bin/env bash

# Parse addon options
CONFIG_PATH=/data/options.json

MQTT_BROKER=$(jq -r '.mqtt_broker' "$CONFIG_PATH")
MQTT_TOPIC=$(jq -r 'if .mqtt_topic == null then "911/cad/events" else .mqtt_topic end' "$CONFIG_PATH")
MQTT_BASE_TOPIC=$(jq -r 'if .mqtt_base_topic == null then "911/cad" else .mqtt_base_topic end' "$CONFIG_PATH")
PUBLISH_RAW=$(jq -r 'if .publish_raw == null then true else .publish_raw end' "$CONFIG_PATH")
HA_DISCOVERY=$(jq -r 'if .ha_discovery == null then false else .ha_discovery end' "$CONFIG_PATH")
MQTT_USERNAME=$(jq -r 'if .mqtt_username == null then "" else .mqtt_username end' "$CONFIG_PATH")
MQTT_PASSWORD=$(jq -r 'if .mqtt_password == null then "" else .mqtt_password end' "$CONFIG_PATH")
POLL_INTERVAL=$(jq -r 'if .poll_interval == null then 60 else .poll_interval end' "$CONFIG_PATH")

# Set environment variables
export MQTT_BROKER
export MQTT_TOPIC
export MQTT_BASE_TOPIC PUBLISH_RAW HA_DISCOVERY MQTT_USERNAME MQTT_PASSWORD
export POLL_INTERVAL

# Run the application
exec /app/icad2mqtt
