package main

import (
	"context"
	"crypto/sha256"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"icad2mqtt/internal/fetch"
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

func TestPublishIfChangedOnlyPublishesChanges(t *testing.T) {
	p := &fakePublisher{}
	b := &Bridge{client: p, config: Config{MqttTopic: "test/events", PublishRaw: true}}
	r := func(s string) fetch.Response { return fetch.Response{Body: s, Hash: sha256.Sum256([]byte(s))} }
	if changed, err := b.publishIfChanged(r("one")); err != nil || !changed || p.calls != 1 {
		t.Fatalf("first=%v %v calls=%d", changed, err, p.calls)
	}
	if changed, err := b.publishIfChanged(r("one")); err != nil || changed || p.calls != 1 {
		t.Fatalf("duplicate=%v %v calls=%d", changed, err, p.calls)
	}
	if changed, err := b.publishIfChanged(r("two")); err != nil || !changed || p.calls != 2 {
		t.Fatalf("changed=%v %v calls=%d", changed, err, p.calls)
	}
}

func TestRawPublishingCanBeDisabled(t *testing.T) {
	p := &fakePublisher{}
	b := &Bridge{client: p, config: Config{MqttTopic: "events", PublishRaw: false}}
	changed, err := b.publishIfChanged(fetch.Response{Body: "x", Hash: sha256.Sum256([]byte("x"))})
	if err != nil || changed || p.calls != 0 {
		t.Fatalf("disabled=%v %v calls=%d", changed, err, p.calls)
	}
}

func TestFetchEventsUsesUserAgent(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.UserAgent() != "test-agent" {
			t.Errorf("UA=%q", r.UserAgent())
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("events"))
	}))
	defer s.Close()
	b := &Bridge{eventURL: s.URL, config: Config{UserAgent: "test-agent", PollInterval: time.Minute}}
	got, err := b.fetchEvents(context.Background())
	if err != nil || got.Body != "events" {
		t.Fatalf("fetch=%q %v", got.Body, err)
	}
}

func TestRedactBroker(t *testing.T) {
	got := redactBroker("mqtt://alice:secret@example.com:1883")
	want := "mqtt://%2A%2A%2A:%2A%2A%2A@example.com:1883"
	if got != want {
		t.Fatalf("redact=%q want=%q", got, want)
	}
}

func TestEmbeddedTimezoneData(t *testing.T) {
	if _, err := time.LoadLocation("America/New_York"); err != nil {
		t.Fatalf("LoadLocation failed: %v", err)
	}
}

func TestRunFailureUsesFlooredBackoffE(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusBadGateway) }))
	defer s.Close()
	var requested time.Duration
	b := &Bridge{config: Config{PollInterval: time.Minute}, fetcher: &fetch.Fetcher{Client: s.Client(), URL: s.URL, PollInterval: time.Minute, Rand: func(n int64) int64 { return 0 }}, sleep: func(ctx context.Context, delay time.Duration) bool { requested = delay; return false }}
	b.Run(context.Background())
	if requested < time.Minute || requested != b.fetcher.BackoffDelay() {
		t.Fatalf("failure delay=%s, expected=%s", requested, b.fetcher.BackoffDelay())
	}
}

func TestRunSuccessUsesPollIntervalAndCancelE(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("ok")) }))
	defer s.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var requested time.Duration
	b := &Bridge{config: Config{PollInterval: time.Minute}, fetcher: &fetch.Fetcher{Client: s.Client(), URL: s.URL, PollInterval: time.Minute}, sleep: func(ctx context.Context, delay time.Duration) bool {
		requested = delay
		cancel()
		select {
		case <-ctx.Done():
			return false
		default:
			return true
		}
	}}
	b.Run(ctx)
	if requested != time.Minute {
		t.Fatalf("success delay=%s", requested)
	}
}

func TestMQTTOptionsF(t *testing.T) {
	withAuth := mqttOptions(Config{MqttBroker: "tcp://broker:1883", ClientID: "client", MqttUsername: "alice", MqttPassword: "secret"})
	if withAuth.ClientID != "client" || withAuth.Username != "alice" || withAuth.Password != "secret" || len(withAuth.Servers) != 1 || withAuth.Servers[0].String() != "tcp://broker:1883" {
		t.Fatalf("options=%+v", withAuth)
	}
	withoutAuth := mqttOptions(Config{MqttBroker: "tcp://broker:1883", ClientID: "client"})
	if withoutAuth.Username != "" || withoutAuth.Password != "" {
		t.Fatalf("empty auth options: username=%q password=%q", withoutAuth.Username, withoutAuth.Password)
	}
}

func TestLoadConfigDelegates(t *testing.T) {
	t.Setenv("POLL_INTERVAL", "60")
	if got, err := loadConfig(); err != nil || got.PollInterval != time.Minute {
		t.Fatalf("config=%+v err=%v", got, err)
	}
}

func TestConnectMQTTRejectsMissingBroker(t *testing.T) {
	if _, err := connectMQTT(Config{ClientID: "test"}); err == nil {
		t.Fatal("expected missing broker error")
	}
}

func TestMainHelpPath(t *testing.T) {
	old := os.Args
	os.Args = []string{"icad2mqtt", "--help"}
	t.Cleanup(func() { os.Args = old })
	main()
}
