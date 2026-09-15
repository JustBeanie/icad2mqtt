package config

import (
	"fmt"
	"log"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	DefaultBroker       = "tcp://localhost:1883"
	DefaultBaseTopic    = "911/cad"
	DefaultRawTopic     = "911/cad/events"
	DefaultClientID     = "icad2mqtt"
	DefaultPollInterval = 60 * time.Second
	DefaultHTTPTimeout  = 15 * time.Second
)

type Config struct {
	MqttBroker     string
	MqttTopic      string
	MqttBaseTopic  string
	PublishRaw     bool
	HADiscovery    bool
	MqttUsername   string
	MqttPassword   string
	ClientID       string
	PollInterval   time.Duration
	RequestTimeout time.Duration
	UserAgent      string
}

func (c Config) String() string {
	return fmt.Sprintf("Config{MqttBroker:%q MqttTopic:%q MqttBaseTopic:%q PublishRaw:%t HADiscovery:%t MqttUsername:%q ClientID:%q PollInterval:%s RequestTimeout:%s UserAgent:%q MqttPassword:<redacted>}", RedactBroker(c.MqttBroker), c.MqttTopic, c.MqttBaseTopic, c.PublishRaw, c.HADiscovery, c.MqttUsername, c.ClientID, c.PollInterval, c.RequestTimeout, c.UserAgent)
}

func (c Config) GoString() string { return c.String() }

func RedactBroker(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.User == nil {
		return raw
	}
	parsed.User = url.UserPassword("***", "***")
	return parsed.String()
}

func Load() (Config, error) {
	poll, err := seconds("POLL_INTERVAL", DefaultPollInterval/time.Second)
	if err != nil {
		return Config{}, err
	}
	if poll < 60 {
		log.Printf("POLL_INTERVAL below 60 seconds; clamping to 60 seconds")
		poll = 60
	}
	timeout, err := secondsInRange("HTTP_TIMEOUT", DefaultHTTPTimeout/time.Second, 5, 60)
	if err != nil {
		return Config{}, err
	}
	publishRaw, err := boolean("PUBLISH_RAW", true)
	if err != nil {
		return Config{}, err
	}
	discovery, err := boolean("HA_DISCOVERY", false)
	if err != nil {
		return Config{}, err
	}
	base := os.Getenv("MQTT_BASE_TOPIC")
	if base == "" {
		base = DefaultBaseTopic
	}
	raw := os.Getenv("MQTT_TOPIC")
	if raw == "" {
		raw = base + "/events"
	}
	if err := validTopic(base); err != nil {
		return Config{}, fmt.Errorf("MQTT_BASE_TOPIC: %w", err)
	}
	if err := validTopic(raw); err != nil {
		return Config{}, fmt.Errorf("MQTT_TOPIC: %w", err)
	}
	return Config{MqttBroker: env("MQTT_BROKER", DefaultBroker), MqttTopic: raw,
		MqttBaseTopic: base, PublishRaw: publishRaw, HADiscovery: discovery,
		MqttUsername: strings.TrimSpace(os.Getenv("MQTT_USERNAME")), MqttPassword: os.Getenv("MQTT_PASSWORD"),
		ClientID: env("CLIENT_ID", DefaultClientID), PollInterval: time.Duration(poll) * time.Second,
		RequestTimeout: time.Duration(timeout) * time.Second, UserAgent: env("HTTP_USER_AGENT", "icad2mqtt/1.0")}, nil
}

func env(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func seconds(key string, fallback time.Duration) (int64, error) {
	raw := env(key, strconv.FormatInt(int64(fallback), 10))
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || value < 1 {
		return 0, fmt.Errorf("%s must be a positive integer, got %q", key, raw)
	}
	return value, nil
}

func secondsInRange(key string, fallback time.Duration, min, max int64) (int64, error) {
	v, err := seconds(key, fallback)
	if err != nil {
		return 0, err
	}
	if v < min || v > max {
		return 0, fmt.Errorf("%s must be between %d and %d seconds, got %d", key, min, max, v)
	}
	return v, nil
}

func boolean(key string, fallback bool) (bool, error) {
	raw := env(key, strconv.FormatBool(fallback))
	v, err := strconv.ParseBool(raw)
	if err != nil || (raw != "true" && raw != "false") {
		return false, fmt.Errorf("%s must be true or false, got %q", key, raw)
	}
	return v, nil
}

func validTopic(topic string) error {
	if strings.TrimSpace(topic) == "" || strings.ContainsAny(topic, "+#") {
		return fmt.Errorf("must be non-empty and contain neither + nor #")
	}
	return nil
}
