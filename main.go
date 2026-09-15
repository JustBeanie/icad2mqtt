package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"icad2mqtt/internal/config"
	"icad2mqtt/internal/fetch"
	_ "time/tzdata" // Scratch/non-root images may not contain host zoneinfo.

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

const cadEventURL = "https://911events.ongov.net/CADInet/app/events.jsp"

type Config = config.Config
type mqttPublisher interface {
	Publish(topic string, qos byte, retained bool, payload interface{}) mqtt.Token
}

type Bridge struct {
	client     mqttPublisher
	config     Config
	fetcher    *fetch.Fetcher
	eventURL   string
	lastHash   [32]byte
	lastUpdate string
	hasUpdate  bool
	sleep      func(context.Context, time.Duration) bool
}

func main() {
	if len(os.Args) > 1 && (os.Args[1] == "--help" || os.Args[1] == "-h") {
		fmt.Println("icad2mqtt: fetch CAD events and publish changes over MQTT")
		fmt.Println("configuration: MQTT_BROKER, MQTT_BASE_TOPIC, MQTT_TOPIC, PUBLISH_RAW, HA_DISCOVERY, MQTT_USERNAME, MQTT_PASSWORD, CLIENT_ID, POLL_INTERVAL, HTTP_TIMEOUT, HTTP_USER_AGENT")
		return
	}
	c, err := loadConfig()
	if err != nil {
		log.Fatalf("invalid configuration: %v", err)
	}
	log.Printf("starting ICAD to MQTT bridge (broker=%s topic=%s interval=%s)", redactBroker(c.MqttBroker), c.MqttTopic, c.PollInterval)
	client, err := connectMQTT(c)
	if err != nil {
		log.Fatalf("failed to connect to MQTT broker: %v", err)
	}
	defer client.Disconnect(250)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	b := &Bridge{client: client, config: c, eventURL: cadEventURL, fetcher: &fetch.Fetcher{Client: &http.Client{Timeout: c.RequestTimeout}, URL: cadEventURL, UserAgent: c.UserAgent, PollInterval: c.PollInterval}}
	b.Run(ctx)
}

func loadConfig() (Config, error) { return config.Load() }

func connectMQTT(c Config) (mqtt.Client, error) {
	opts := mqttOptions(c)
	client := mqtt.NewClient(opts)
	token := client.Connect()
	if !token.WaitTimeout(15 * time.Second) {
		return nil, fmt.Errorf("connection timeout")
	}
	if token.Error() != nil {
		return nil, token.Error()
	}
	return client, nil
}

func mqttOptions(c Config) *mqtt.ClientOptions {
	opts := mqtt.NewClientOptions().AddBroker(c.MqttBroker).SetClientID(c.ClientID)
	if c.MqttUsername != "" {
		opts.SetUsername(c.MqttUsername)
	}
	if c.MqttPassword != "" {
		opts.SetPassword(c.MqttPassword)
	}
	opts.SetAutoReconnect(true).SetConnectRetry(true).SetConnectRetryInterval(2 * time.Second)
	opts.OnConnect = func(mqtt.Client) { log.Println("connected to MQTT broker") }
	opts.OnConnectionLost = func(_ mqtt.Client, err error) { log.Printf("MQTT connection lost: %v", err) }
	return opts
}

func (b *Bridge) Run(ctx context.Context) {
	sleep := b.sleep
	if sleep == nil {
		sleep = func(ctx context.Context, delay time.Duration) bool {
			timer := time.NewTimer(delay)
			defer timer.Stop()
			select {
			case <-ctx.Done():
				return false
			case <-timer.C:
				return true
			}
		}
	}
	for {
		b.poll(ctx)
		delay := b.config.PollInterval
		if b.fetcher.Failures > 0 {
			delay = b.fetcher.BackoffDelay()
		}
		if !sleep(ctx, delay) {
			return
		}
	}
}

func (b *Bridge) poll(ctx context.Context) {
	r, err := b.fetchEvents(ctx)
	if err != nil {
		log.Printf("failed to fetch CAD events: %v", err)
		return
	}
	changed, err := b.publishIfChanged(r)
	if err != nil {
		log.Printf("failed to publish CAD events: %v", err)
		return
	}
	if changed {
		log.Printf("published CAD update (%d bytes)", len(r.Body))
	}
}

func (b *Bridge) fetchEvents(ctx context.Context) (fetch.Response, error) {
	if b.fetcher == nil {
		b.fetcher = &fetch.Fetcher{URL: b.eventURL, UserAgent: b.config.UserAgent, PollInterval: b.config.PollInterval}
	}
	if b.fetcher.URL == "" {
		b.fetcher.URL = b.eventURL
	}
	return b.fetcher.Fetch(ctx)
}

func (b *Bridge) publishIfChanged(r fetch.Response) (bool, error) {
	if b.hasUpdate && r.Hash == b.lastHash {
		return false, nil
	}
	if b.config.PublishRaw {
		token := b.client.Publish(b.config.MqttTopic, 1, false, r.Body)
		if !token.WaitTimeout(5 * time.Second) {
			return false, fmt.Errorf("MQTT publish timeout")
		}
		if err := token.Error(); err != nil {
			return false, err
		}
	}
	b.lastHash = r.Hash
	b.lastUpdate = r.Body
	b.hasUpdate = true
	return b.config.PublishRaw, nil
}

func redactBroker(raw string) string {
	return config.RedactBroker(raw)
}
