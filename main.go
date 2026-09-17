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
	"icad2mqtt/internal/normalize"
	"icad2mqtt/internal/parse"
	"icad2mqtt/internal/publish"
	_ "time/tzdata" // Scratch/non-root images may not contain host zoneinfo.

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

const cadEventURL = "https://911events.ongov.net/CADInet/app/events.jsp"

type Config = config.Config
type mqttPublisher interface {
	Publish(topic string, qos byte, retained bool, payload interface{}) mqtt.Token
}
type mqttOutput struct{ client mqttPublisher }

func (o mqttOutput) Publish(topic string, qos byte, retained bool, payload []byte) error {
	t := o.client.Publish(topic, qos, retained, payload)
	if !t.WaitTimeout(5 * time.Second) {
		return fmt.Errorf("publish timeout")
	}
	return t.Error()
}

type Bridge struct {
	client     mqttPublisher
	config     Config
	fetcher    *fetch.Fetcher
	eventURL   string
	lastHash   [32]byte
	lastUpdate string
	hasUpdate  bool
	structured *publish.Manager
	sleep      func(context.Context, time.Duration) bool
}

func main() {
	if len(os.Args) > 1 && (os.Args[1] == "--help" || os.Args[1] == "-h") {
		fmt.Println("icad2mqtt: fetch CAD events and publish changes over MQTT")
		fmt.Println("configuration: MQTT_BROKER, MQTT_BASE_TOPIC, MQTT_TOPIC, PUBLISH_RAW, HA_DISCOVERY, MQTT_USERNAME, MQTT_PASSWORD, CLIENT_ID, POLL_INTERVAL, HTTP_TIMEOUT, HTTP_USER_AGENT")
		return
	}
	c, err := loadConfigAndDrop()
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
	b := &Bridge{client: client, config: c, eventURL: cadEventURL, fetcher: &fetch.Fetcher{Client: &http.Client{Timeout: c.RequestTimeout}, URL: cadEventURL, UserAgent: c.UserAgent, PollInterval: c.PollInterval}, structured: publish.New(publish.Config{BaseTopic: c.MqttBaseTopic, RawTopic: c.MqttTopic, ClientID: c.ClientID, PublishRaw: c.PublishRaw, HADiscovery: c.HADiscovery, Version: "2.0.0"}, mqttOutput{client})}
	if err := b.structured.PublishDiscovery(); err != nil {
		log.Printf("structured publish failed (topic=discovery)")
	}
	b.Run(ctx)
}

func loadConfig() (Config, error) { return config.Load() }

var dropPrivilegesFn = dropPrivileges

func loadConfigAndDrop() (Config, error) {
	return loadAndDropConfig(loadConfig, dropPrivilegesFn)
}

func loadAndDropConfig(load func() (Config, error), drop func() error) (Config, error) {
	c, err := load()
	if err != nil {
		return Config{}, err
	}
	if err := drop(); err != nil {
		return Config{}, fmt.Errorf("drop privileges: %w", err)
	}
	return c, nil
}

func runWithConfig(load func() (Config, error), drop func() error, connect func(Config) (mqtt.Client, error)) error {
	c, err := loadAndDropConfig(load, drop)
	if err != nil {
		return err
	}
	_, err = connect(c)
	return err
}

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
	opts.SetWill(c.MqttBaseTopic+"/availability", "offline", 1, true)
	opts.OnConnect = func(client mqtt.Client) {
		log.Println("connected to MQTT broker")
		publishOnline(client, c.MqttBaseTopic)
	}
	opts.OnConnectionLost = func(_ mqtt.Client, err error) { log.Printf("MQTT connection lost: %v", err) }
	return opts
}

func publishOnline(client mqttPublisher, base string) {
	topic := base + "/availability"
	token := client.Publish(topic, 1, true, "online")
	if !token.WaitTimeout(5*time.Second) || token.Error() != nil {
		log.Printf("MQTT publish failed (topic=%s)", topic)
	}
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
		log.Printf("fetch failed (reason=fetch_error)")
		if b.structured != nil {
			_ = b.structured.Failure("fetch_error", time.Now())
		}
		return
	}
	changed, err := b.publishIfChanged(r)
	if err != nil {
		log.Printf("publish failed (topic=%s)", b.config.MqttTopic)
		return
	}
	if b.structured != nil {
		result, parseErr := parse.Parse(r.Body)
		if parseErr != nil {
			reason := "parse_error"
			if pageErr, ok := parseErr.(*parse.PageError); ok {
				reason = string(pageErr.Reason)
			}
			log.Printf("page rejected (reason=%s)", reason)
			_ = b.structured.PageError(reason, time.Now())
			return
		}
		snapshot, stats := normalize.Page(result, time.Now())
		if publishErr := b.structured.Valid(snapshot, stats, time.Now()); publishErr != nil {
			log.Printf("publish failed (topic=%s)", b.config.MqttBaseTopic)
		}
	}
	if changed {
		log.Printf("published raw update (topic=%s)", b.config.MqttTopic)
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
