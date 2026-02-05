#!/usr/bin/env bash

# Parse addon options
CONFIG_PATH=/data/options.json

MQTT_BROKER=$(jq -r '.mqtt_broker' "$CONFIG_PATH")
MQTT_TOPIC=$(jq -r '.mqtt_topic // "911/cad/events"' "$CONFIG_PATH")
POLL_INTERVAL=$(jq -r '.poll_interval // 30' "$CONFIG_PATH")

# Set environment variables
export MQTT_BROKER
export MQTT_TOPIC
export POLL_INTERVAL

# Run the application
exec /app/icad2mqtt
