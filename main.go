package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

const (
	cadEventURL           = "https://911events.ongov.net/CADInet/app/events.jsp"
	defaultMqttBroker     = "tcp://localhost:1883"
	defaultMqttTopic      = "911/cad/events"
	defaultClientID       = "icad2mqtt"
	defaultPollInterval   = 30 * time.Second
	defaultRequestTimeout = 10 * time.Second
	maxResponseSize       = 5 << 20
)

type Config struct {
	MqttBroker     string
	MqttTopic      string
	ClientID       string
	PollInterval   time.Duration
	RequestTimeout time.Duration
	UserAgent      string
}

type mqttPublisher interface {
	Publish(topic string, qos byte, retained bool, payload interface{}) mqtt.Token
}

type Bridge struct {
	client     mqttPublisher
	httpClient *http.Client
	config     Config
	eventURL   string
	lastUpdate string
	hasUpdate  bool
}

func main() {
	if len(os.Args) > 1 && (os.Args[1] == "--help" || os.Args[1] == "-h") {
		fmt.Println("icad2mqtt: fetch CAD events and publish changes over MQTT")
		fmt.Println("configuration: MQTT_BROKER, MQTT_TOPIC, CLIENT_ID, POLL_INTERVAL, HTTP_USER_AGENT")
		return
	}

	config, err := loadConfig()
	if err != nil {
		log.Fatalf("invalid configuration: %v", err)
	}

	log.Printf("starting ICAD to MQTT bridge (broker=%s topic=%s interval=%s)",
		redactBroker(config.MqttBroker), config.MqttTopic, config.PollInterval)

	client, err := connectMQTT(config)
	if err != nil {
		log.Fatalf("failed to connect to MQTT broker: %v", err)
	}
	defer client.Disconnect(250)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	bridge := &Bridge{
		client:     client,
		httpClient: &http.Client{Timeout: config.RequestTimeout},
		config:     config,
		eventURL:   cadEventURL,
	}
	bridge.Run(ctx)
	log.Println("ICAD to MQTT bridge stopped")
}

func loadConfig() (Config, error) {
	pollSeconds, err := positiveIntEnv("POLL_INTERVAL", int(defaultPollInterval/time.Second))
	if err != nil {
		return Config{}, err
	}

	return Config{
		MqttBroker:     getEnv("MQTT_BROKER", defaultMqttBroker),
		MqttTopic:      getEnv("MQTT_TOPIC", defaultMqttTopic),
		ClientID:       getEnv("CLIENT_ID", defaultClientID),
		PollInterval:   time.Duration(pollSeconds) * time.Second,
		RequestTimeout: defaultRequestTimeout,
		UserAgent:      getEnv("HTTP_USER_AGENT", "icad2mqtt/1.0"),
	}, nil
}

func positiveIntEnv(key string, fallback int) (int, error) {
	raw := getEnv(key, strconv.Itoa(fallback))
	value, err := strconv.Atoi(raw)
	if err != nil || value < 1 {
		return 0, fmt.Errorf("%s must be a positive integer, got %q", key, raw)
	}
	return value, nil
}

func getEnv(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func connectMQTT(config Config) (mqtt.Client, error) {
	opts := mqtt.NewClientOptions()
	opts.AddBroker(config.MqttBroker)
	opts.SetClientID(config.ClientID)
	opts.SetAutoReconnect(true)
	opts.SetConnectRetry(true)
	opts.SetConnectRetryInterval(2 * time.Second)
	opts.OnConnect = func(mqtt.Client) { log.Println("connected to MQTT broker") }
	opts.OnConnectionLost = func(_ mqtt.Client, err error) { log.Printf("MQTT connection lost: %v", err) }

	client := mqtt.NewClient(opts)
	if token := client.Connect(); !token.WaitTimeout(15 * time.Second) {
		return nil, fmt.Errorf("connection timeout")
	} else if token.Error() != nil {
		return nil, token.Error()
	}
	return client, nil
}

func (b *Bridge) Run(ctx context.Context) {
	b.poll(ctx)
	ticker := time.NewTicker(b.config.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			b.poll(ctx)
		}
	}
}

func (b *Bridge) poll(ctx context.Context) {
	data, err := b.fetchEvents(ctx)
	if err != nil {
		log.Printf("failed to fetch CAD events: %v", err)
		return
	}
	changed, err := b.publishIfChanged(data)
	if err != nil {
		log.Printf("failed to publish CAD events: %v", err)
		return
	}
	if changed {
		log.Printf("published CAD update (%d bytes)", len(data))
	}
}

func (b *Bridge) fetchEvents(ctx context.Context) (string, error) {
	eventURL := b.eventURL
	if eventURL == "" {
		eventURL = cadEventURL
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, eventURL, nil)
	if err != nil {
		return "", fmt.Errorf("create HTTP request: %w", err)
	}
	request.Header.Set("User-Agent", b.config.UserAgent)

	response, err := b.httpClient.Do(request)
	if err != nil {
		return "", fmt.Errorf("HTTP request failed: %w", err)
	}
	defer func() {
		if err := response.Body.Close(); err != nil {
			log.Printf("failed to close HTTP response body: %v", err)
		}
	}()
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected HTTP status: %s", response.Status)
	}

	body, err := io.ReadAll(io.LimitReader(response.Body, maxResponseSize+1))
	if err != nil {
		return "", fmt.Errorf("read HTTP response: %w", err)
	}
	if len(body) > maxResponseSize {
		return "", fmt.Errorf("HTTP response exceeds %d bytes", maxResponseSize)
	}
	return string(body), nil
}

func (b *Bridge) publishIfChanged(data string) (bool, error) {
	if b.hasUpdate && data == b.lastUpdate {
		return false, nil
	}
	token := b.client.Publish(b.config.MqttTopic, 1, false, data)
	if !token.WaitTimeout(5 * time.Second) {
		return false, fmt.Errorf("MQTT publish timeout")
	}
	if err := token.Error(); err != nil {
		return false, err
	}
	b.lastUpdate = data
	b.hasUpdate = true
	return true, nil
}

func redactBroker(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.User == nil {
		return raw
	}
	parsed.User = url.UserPassword("***", "***")
	return parsed.String()
}
