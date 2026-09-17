package config

import (
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// OptionsPath is the Home Assistant add-on options file. It is a variable so
// tests can use a temporary file without changing the production path.
var OptionsPath = "/data/options.json"

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
	opts, err := loadOptions(OptionsPath)
	if err != nil {
		return Config{}, err
	}
	poll, err := secondsValue("POLL_INTERVAL", int64(DefaultPollInterval/time.Second), opts.pollInterval)
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
	publishRaw, err := booleanValue("PUBLISH_RAW", true, opts.publishRaw)
	if err != nil {
		return Config{}, err
	}
	discovery, err := booleanValue("HA_DISCOVERY", false, opts.haDiscovery)
	if err != nil {
		return Config{}, err
	}
	base := rawStringValue("MQTT_BASE_TOPIC", DefaultBaseTopic, opts.baseTopic)
	raw := rawStringValue("MQTT_TOPIC", base+"/events", opts.topic)
	if err := validTopic(base); err != nil {
		return Config{}, fmt.Errorf("MQTT_BASE_TOPIC: %w", err)
	}
	if err := validTopic(raw); err != nil {
		return Config{}, fmt.Errorf("MQTT_TOPIC: %w", err)
	}
	return Config{MqttBroker: stringValue("MQTT_BROKER", DefaultBroker, opts.broker), MqttTopic: raw,
		MqttBaseTopic: base, PublishRaw: publishRaw, HADiscovery: discovery,
		MqttUsername: strings.TrimSpace(stringValue("MQTT_USERNAME", "", opts.username)), MqttPassword: stringValue("MQTT_PASSWORD", "", opts.password),
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
	return secondsValue(key, int64(fallback), nil)
}

func secondsValue(key string, fallback int64, option *int64) (int64, error) {
	raw := strconv.FormatInt(fallback, 10)
	if option != nil {
		return *option, validateSeconds(key, *option)
	}
	raw = env(key, raw)
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || value < 1 {
		return 0, fmt.Errorf("%s must be a positive integer, got %q", key, raw)
	}
	return value, nil
}

func validateSeconds(key string, value int64) error {
	if value < 1 {
		return fmt.Errorf("%s must be a positive integer, got %d", key, value)
	}
	return nil
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

func booleanValue(key string, fallback bool, option *bool) (bool, error) {
	raw := strconv.FormatBool(fallback)
	if option != nil {
		return *option, nil
	}
	raw = env(key, raw)
	v, err := strconv.ParseBool(raw)
	if err != nil || (raw != "true" && raw != "false") {
		return false, fmt.Errorf("%s must be true or false, got %q", key, raw)
	}
	return v, nil
}

func stringValue(key, fallback string, option *string) string {
	if option != nil {
		return *option
	}
	return env(key, fallback)
}

func rawStringValue(key, fallback string, option *string) string {
	if option != nil {
		return *option
	}
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

type addonOptions struct {
	broker       *string
	baseTopic    *string
	topic        *string
	publishRaw   *bool
	haDiscovery  *bool
	username     *string
	password     *string
	pollInterval *int64
}

type addonOptionsJSON struct {
	Broker       *string `json:"mqtt_broker"`
	BaseTopic    *string `json:"mqtt_base_topic"`
	Topic        *string `json:"mqtt_topic"`
	PublishRaw   *bool   `json:"publish_raw"`
	HADiscovery  *bool   `json:"ha_discovery"`
	Username     *string `json:"mqtt_username"`
	Password     *string `json:"mqtt_password"`
	PollInterval *int64  `json:"poll_interval"`
}

func loadOptions(path string) (addonOptions, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return addonOptions{}, nil
	}
	if err != nil {
		return addonOptions{}, fmt.Errorf("read options file: %w", err)
	}
	var raw addonOptionsJSON
	if err := json.Unmarshal(data, &raw); err != nil {
		return addonOptions{}, fmt.Errorf("options file contains malformed JSON")
	}
	broker := DefaultBroker
	baseTopic := DefaultBaseTopic
	topic := DefaultRawTopic
	publishRaw := true
	haDiscovery := false
	username := ""
	password := ""
	pollInterval := int64(DefaultPollInterval / time.Second)
	if raw.Broker != nil {
		broker = *raw.Broker
	}
	if raw.BaseTopic != nil {
		baseTopic = *raw.BaseTopic
	}
	if raw.Topic != nil {
		topic = *raw.Topic
	}
	if raw.PublishRaw != nil {
		publishRaw = *raw.PublishRaw
	}
	if raw.HADiscovery != nil {
		haDiscovery = *raw.HADiscovery
	}
	if raw.Username != nil {
		username = *raw.Username
	}
	if raw.Password != nil {
		password = *raw.Password
	}
	if raw.PollInterval != nil {
		pollInterval = *raw.PollInterval
	}
	return addonOptions{broker: &broker, baseTopic: &baseTopic, topic: &topic,
		publishRaw: &publishRaw, haDiscovery: &haDiscovery, username: &username,
		password: &password, pollInterval: &pollInterval}, nil
}

func validTopic(topic string) error {
	if strings.TrimSpace(topic) == "" || strings.ContainsAny(topic, "+#") {
		return fmt.Errorf("must be non-empty and contain neither + nor #")
	}
	return nil
}
