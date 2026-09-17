package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"icad2mqtt/internal/config"
	"icad2mqtt/internal/fetch"
	"icad2mqtt/internal/model"
	"icad2mqtt/internal/normalize"
	"icad2mqtt/internal/parse"
	"icad2mqtt/internal/publish"
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
	err     error
}

type published struct {
	topic    string
	qos      byte
	retained bool
	payload  []byte
}
type recordingPublisher struct {
	calls []published
	err   error
}

func (p *recordingPublisher) Publish(topic string, qos byte, retained bool, payload interface{}) mqtt.Token {
	b, _ := payload.([]byte)
	if b == nil {
		if s, ok := payload.(string); ok {
			b = []byte(s)
		}
	}
	p.calls = append(p.calls, published{topic, qos, retained, b})
	return fakeToken{err: p.err}
}

type recordingOutput struct{ p *recordingPublisher }

func (o recordingOutput) Publish(topic string, qos byte, retained bool, payload []byte) error {
	o.p.calls = append(o.p.calls, published{topic, qos, retained, append([]byte(nil), payload...)})
	return nil
}

func (p *fakePublisher) Publish(topic string, _ byte, _ bool, payload interface{}) mqtt.Token {
	p.topic, p.payload, p.calls = topic, payload, p.calls+1
	return fakeToken{err: p.err}
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

func TestMQTTOutputPublishesBytes(t *testing.T) {
	p := &fakePublisher{}
	if err := (mqttOutput{p}).Publish("topic", 1, true, []byte("payload")); err != nil {
		t.Fatal(err)
	}
	if p.topic != "topic" || p.calls != 1 {
		t.Fatalf("publisher=%+v", p)
	}
}

func TestMQTTOutputReturnsPublishError(t *testing.T) {
	want := errors.New("publish failed")
	if err := (mqttOutput{&fakePublisher{err: want}}).Publish("topic", 1, false, []byte("payload")); !errors.Is(err, want) {
		t.Fatalf("err=%v", err)
	}
}

func TestPollStructuredFixtureCycle(t *testing.T) {
	first, err := os.ReadFile("testdata/events_all.html")
	if err != nil {
		t.Fatal(err)
	}
	second, err := os.ReadFile("testdata/events_type_changed.html")
	if err != nil {
		t.Fatal(err)
	}
	requests := 0
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests++
		if requests == 1 {
			_, _ = w.Write(first)
		} else {
			_, _ = w.Write(second)
		}
	}))
	defer s.Close()
	p := &fakePublisher{}
	b := &Bridge{client: p, config: Config{MqttTopic: "raw", MqttBaseTopic: "base", PublishRaw: true, ClientID: "client", HADiscovery: true}, eventURL: s.URL, fetcher: &fetch.Fetcher{Client: s.Client(), URL: s.URL, PollInterval: time.Minute}, structured: publish.New(publish.Config{BaseTopic: "base", RawTopic: "raw", ClientID: "client", HADiscovery: true}, mqttOutput{p})}
	b.poll(context.Background())
	b.poll(context.Background())
	if requests != 2 || p.calls < 4 {
		t.Fatalf("requests=%d calls=%d", requests, p.calls)
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
	withoutAuth.OnConnectionLost(nil, errors.New("test connection loss"))
}

func TestMQTTOptionsWillAndOnConnect(t *testing.T) {
	opts := mqttOptions(Config{MqttBaseTopic: "base", ClientID: "client"})
	if !opts.WillEnabled || opts.WillTopic != "base/availability" || string(opts.WillPayload) != "offline" || opts.WillQos != 1 || !opts.WillRetained {
		t.Fatalf("will not configured: %+v", opts)
	}
	if opts.OnConnect == nil {
		t.Fatal("missing OnConnect")
	}
	p := &recordingPublisher{}
	publishOnline(p, "base")
	if len(p.calls) != 1 || p.calls[0].topic != "base/availability" || p.calls[0].qos != 1 || !p.calls[0].retained || string(p.calls[0].payload) != "online" {
		t.Fatalf("online=%+v", p.calls)
	}
}

func TestPollPageErrorPublishesNothingStructured(t *testing.T) {
	bad, _ := os.ReadFile("testdata/events_missing_header.html")
	good, _ := os.ReadFile("testdata/events_all.html")
	n := 0
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		n++
		if n == 1 {
			_, _ = w.Write(bad)
		} else {
			_, _ = w.Write(good)
		}
	}))
	defer s.Close()
	p := &recordingPublisher{}
	b := &Bridge{client: p, config: Config{MqttTopic: "raw", MqttBaseTopic: "base", PublishRaw: true, ClientID: "c", HADiscovery: true}, eventURL: s.URL, fetcher: &fetch.Fetcher{Client: s.Client(), URL: s.URL, PollInterval: time.Minute}, structured: publish.New(publish.Config{BaseTopic: "base", RawTopic: "raw", ClientID: "c", HADiscovery: true}, recordingOutput{p})}
	b.poll(context.Background())
	var health publish.Health
	for _, c := range p.calls {
		if c.topic == "base/health" {
			_ = json.Unmarshal(c.payload, &health)
		}
		if c.topic == "base/incidents" || c.topic == "base/incident" || c.topic == "base/counts" {
			t.Fatalf("unexpected structured call %+v", c)
		}
	}
	if health.PageErrorsTotal != 1 || len(p.calls) != 2 {
		t.Fatalf("calls=%+v health=%+v", p.calls, health)
	}
	p.calls = nil
	b.poll(context.Background())
	for _, c := range p.calls {
		if c.topic == "base/incident" {
			t.Fatal("page error changed diff state")
		}
	}
}

func TestLogsContainNoIncidentContent(t *testing.T) {
	var buf bytes.Buffer
	oldOut := log.Writer()
	oldFlags := log.Flags()
	log.SetOutput(&buf)
	log.SetFlags(0)
	t.Cleanup(func() { log.SetOutput(oldOut); log.SetFlags(oldFlags) })
	files, _ := filepath.Glob("testdata/events_*.html")
	for _, name := range files {
		body, _ := os.ReadFile(name)
		s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(body) }))
		p := &recordingPublisher{}
		b := &Bridge{client: p, config: Config{MqttTopic: "raw", MqttBaseTopic: "base", PublishRaw: true, ClientID: "c", HADiscovery: true}, eventURL: s.URL, fetcher: &fetch.Fetcher{Client: s.Client(), URL: s.URL, PollInterval: time.Minute}, structured: publish.New(publish.Config{BaseTopic: "base", RawTopic: "raw", ClientID: "c", HADiscovery: true}, recordingOutput{p})}
		b.poll(context.Background())
		s.Close()
		if result, err := parse.Parse(string(body)); err == nil {
			snapshot, _ := normalize.Page(result, time.Now())
			for _, i := range snapshot.Incidents {
				for _, v := range []string{i.AddressRaw, i.AddressClean, i.CrossStreetsRaw, i.Type.Raw} {
					if v != "" && strings.Contains(buf.String(), v) {
						t.Fatalf("logged fixture content %q", v)
					}
				}
			}
		}
	}
	if buf.Len() == 0 {
		t.Fatal("log capture empty")
	}
}

func TestRunEndToEndOrderedPublishes(t *testing.T) {
	first, _ := os.ReadFile("testdata/events_all.html")
	second, _ := os.ReadFile("testdata/events_type_changed.html")
	n := 0
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		n++
		if n == 1 {
			_, _ = w.Write(first)
		} else {
			_, _ = w.Write(second)
		}
	}))
	defer s.Close()
	p := &recordingPublisher{}
	sleeps := 0
	b := &Bridge{client: p, config: Config{MqttTopic: "raw", MqttBaseTopic: "base", PublishRaw: true, ClientID: "c", HADiscovery: true, PollInterval: time.Minute}, eventURL: s.URL, fetcher: &fetch.Fetcher{Client: s.Client(), URL: s.URL, PollInterval: time.Minute}, structured: publish.New(publish.Config{BaseTopic: "base", RawTopic: "raw", ClientID: "c", HADiscovery: true}, recordingOutput{p}), sleep: func(context.Context, time.Duration) bool { sleeps++; return sleeps < 2 }}
	b.Run(context.Background())
	want := []string{"raw", "base/incidents", "base/counts", "base/health", "raw", "base/incidents", "base/incident", "base/health"}
	if len(p.calls) != len(want) {
		t.Fatalf("calls=%d want=%d", len(p.calls), len(want))
	}
	for i, w := range want {
		if p.calls[i].topic != w || p.calls[i].qos != 1 {
			t.Fatalf("%d got=%+v want=%s", i, p.calls[i], w)
		}
	}
	var e model.Event
	if err := json.Unmarshal(p.calls[6].payload, &e); err != nil || e.Event != model.Updated {
		t.Fatalf("event=%+v err=%v", e, err)
	}
}

func TestLoadConfigDelegates(t *testing.T) {
	t.Setenv("POLL_INTERVAL", "60")
	if got, err := loadConfig(); err != nil || got.PollInterval != time.Minute {
		t.Fatalf("config=%+v err=%v", got, err)
	}
}

func TestLoadConfigWiresOptionsC(t *testing.T) {
	path := filepath.Join(t.TempDir(), "options.json")
	if err := os.WriteFile(path, []byte(`{"mqtt_broker":"tcp://file:1883","mqtt_base_topic":"file/base","publish_raw":false,"poll_interval":75,"mqtt_password":"wire-secret"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	oldPath := config.OptionsPath
	config.OptionsPath = path
	t.Cleanup(func() { config.OptionsPath = oldPath })
	t.Setenv("MQTT_BROKER", "tcp://env:1883")
	got, err := loadConfig()
	if err != nil || got.MqttBroker != "tcp://file:1883" || got.MqttBaseTopic != "file/base" || got.PublishRaw || got.PollInterval != 75*time.Second {
		t.Fatalf("config=%+v err=%v", got, err)
	}
	if strings.Contains(got.String(), "wire-secret") {
		t.Fatalf("config string leaked password: %s", got.String())
	}
}

func TestPrivilegeDropOrderingC(t *testing.T) {
	var events []string
	stop := errors.New("stop after connect")
	err := runWithConfig(
		func() (Config, error) { events = append(events, "config"); return Config{}, nil },
		func() error { events = append(events, "drop"); return nil },
		func(Config) (mqtt.Client, error) { events = append(events, "mqtt"); return nil, stop },
	)
	if !errors.Is(err, stop) || strings.Join(events, ",") != "config,drop,mqtt" {
		t.Fatalf("err=%v events=%v", err, events)
	}
}

func TestPrivilegeDropFailureStopsNetworkC(t *testing.T) {
	var events []string
	failure := errors.New("drop failed")
	err := runWithConfig(
		func() (Config, error) { events = append(events, "config"); return Config{}, nil },
		func() error { events = append(events, "drop"); return failure },
		func(Config) (mqtt.Client, error) { events = append(events, "mqtt"); return nil, nil },
	)
	if !errors.Is(err, failure) || strings.Join(events, ",") != "config,drop" {
		t.Fatalf("err=%v events=%v", err, events)
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
	oldLoad := loadConfigAndDropFn
	loadConfigAndDropFn = func() (Config, error) {
		t.Fatal("help must return before config loading and privilege drop")
		return Config{}, nil
	}
	t.Cleanup(func() { loadConfigAndDropFn = oldLoad })
	main()
}
