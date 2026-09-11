package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

type fakeToken struct{ err error }

func (t fakeToken) Wait() bool                     { return true }
func (t fakeToken) WaitTimeout(time.Duration) bool { return true }
func (t fakeToken) Error() error                   { return t.err }
func (t fakeToken) Done() <-chan struct{}          { done := make(chan struct{}); close(done); return done }

type fakePublisher struct {
	topic   string
	payload interface{}
	calls   int
}

func (p *fakePublisher) Publish(topic string, _ byte, _ bool, payload interface{}) mqtt.Token {
	p.topic, p.payload, p.calls = topic, payload, p.calls+1
	return fakeToken{}
}

func TestLoadConfigDefaultsAndPollInterval(t *testing.T) {
	t.Setenv("MQTT_BROKER", "")
	t.Setenv("MQTT_TOPIC", "")
	t.Setenv("CLIENT_ID", "")
	t.Setenv("POLL_INTERVAL", "7")

	config, err := loadConfig()
	if err != nil {
		t.Fatalf("loadConfig returned error: %v", err)
	}
	if config.MqttBroker != defaultMqttBroker || config.MqttTopic != defaultMqttTopic {
		t.Fatalf("unexpected defaults: %+v", config)
	}
	if config.PollInterval != 7*time.Second {
		t.Fatalf("poll interval = %s, want 7s", config.PollInterval)
	}
}

func TestLoadConfigRejectsInvalidPollInterval(t *testing.T) {
	t.Setenv("POLL_INTERVAL", "0")
	if _, err := loadConfig(); err == nil {
		t.Fatal("expected invalid poll interval error")
	}
}

func TestRedactBroker(t *testing.T) {
	got := redactBroker("mqtt://alice:secret@example.com:1883")
	want := "mqtt://%2A%2A%2A:%2A%2A%2A@example.com:1883"
	if got != want {
		t.Fatalf("redactBroker = %q, want %q", got, want)
	}
}

func TestFetchEventsUsesUserAgentAndLimitsStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") != "test-agent" {
			t.Errorf("User-Agent = %q, want test-agent", r.Header.Get("User-Agent"))
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("events"))
	}))
	defer server.Close()

	bridge := &Bridge{
		httpClient: server.Client(),
		eventURL:   server.URL,
		config:     Config{UserAgent: "test-agent"},
	}
	got, err := bridge.fetchEvents(context.Background())
	if err != nil || got != "events" {
		t.Fatalf("fetchEvents = %q, %v", got, err)
	}
}

func TestPublishIfChangedOnlyPublishesChanges(t *testing.T) {
	publisher := &fakePublisher{}
	bridge := &Bridge{client: publisher, config: Config{MqttTopic: "test/events"}}

	changed, err := bridge.publishIfChanged("one")
	if err != nil || !changed || publisher.calls != 1 {
		t.Fatalf("first publish = changed %v, err %v, calls %d", changed, err, publisher.calls)
	}
	changed, err = bridge.publishIfChanged("one")
	if err != nil || changed || publisher.calls != 1 {
		t.Fatalf("duplicate publish = changed %v, err %v, calls %d", changed, err, publisher.calls)
	}
	changed, err = bridge.publishIfChanged("two")
	if err != nil || !changed || publisher.calls != 2 {
		t.Fatalf("changed publish = changed %v, err %v, calls %d", changed, err, publisher.calls)
	}
}
