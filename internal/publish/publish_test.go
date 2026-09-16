package publish

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"icad2mqtt/internal/model"
	"icad2mqtt/internal/normalize"
)

type call struct {
	topic    string
	qos      byte
	retained bool
	payload  []byte
}
type fake struct{ calls []call }

func (f *fake) Publish(topic string, qos byte, retained bool, payload []byte) error {
	f.calls = append(f.calls, call{topic, qos, retained, append([]byte(nil), payload...)})
	return nil
}
func invented() model.Snapshot {
	updated := time.Date(2026, 9, 14, 14, 0, 0, 0, time.UTC)
	return model.NewSnapshot(&updated, time.Date(2026, 9, 14, 14, 0, 0, 0, time.UTC), []model.Incident{{ID: "abc", Agency: model.Agency{Name: "Invented Fire", Category: model.Fire}, Type: model.Type{Raw: "FIRE-1"}, Status: "active"}})
}
func TestPublishContractAndSuppression(t *testing.T) {
	f := &fake{}
	m := New(Config{BaseTopic: "x", ClientID: "c", HADiscovery: true, Version: "test"}, f)
	f.calls = nil
	s := invented()
	if err := m.Valid(s, normalize.Stats{Dropped: map[string]int{}}, time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := m.Valid(s, normalize.Stats{Dropped: map[string]int{}}, time.Now().Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	var snapshots, events int
	for _, c := range f.calls {
		if c.topic == "x/incidents" {
			snapshots++
		}
		if c.topic == "x/incident" {
			events++
		}
		if c.qos != 1 || !c.retained && c.topic == "x/incidents" {
			t.Fatalf("flags: %+v", c)
		}
	}
	if snapshots != 1 || events != 0 {
		t.Fatalf("snapshot=%d events=%d", snapshots, events)
	}
	if err := m.Valid(s, normalize.Stats{Dropped: map[string]int{}}, time.Now().Add(2*time.Minute)); err != nil {
		t.Fatal(err)
	}
}
func TestSeedNewUpdatedAndCounts(t *testing.T) {
	f := &fake{}
	m := New(Config{BaseTopic: "x", HADiscovery: true, ClientID: "c"}, f)
	f.calls = nil
	s := invented()
	_ = m.Valid(s, normalize.Stats{Dropped: map[string]int{}}, time.Now())
	nBytes, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	var n model.Snapshot
	if err := json.Unmarshal(nBytes, &n); err != nil {
		t.Fatal(err)
	}
	n.Incidents[0].Type.Raw = "FIRE-2"
	_ = m.Valid(n, normalize.Stats{Dropped: map[string]int{}}, time.Now())
	seen := false
	for _, c := range f.calls {
		if c.topic == "x/incident" {
			seen = true
			var e model.Event
			_ = json.Unmarshal(c.payload, &e)
			if e.Event != model.Updated {
				t.Fatal(e)
			}
		}
	}
	if !seen {
		t.Fatal("missing updated event")
	}
}
func TestHealthTransitionsAndDiscoveryClear(t *testing.T) {
	f := &fake{}
	m := New(Config{BaseTopic: "x", ClientID: "c"}, f)
	if len(f.calls) != 6 {
		t.Fatalf("clear calls=%d", len(f.calls))
	}
	s := invented()
	_ = m.Valid(s, normalize.Stats{SkippedRows: 1, Dropped: map[string]int{"invalid_time": 1}}, time.Now())
	if m.Health().Status != "degraded" {
		t.Fatal(m.Health())
	}
	for i := 0; i < 3; i++ {
		_ = m.Failure("fetch_error", time.Now())
	}
	if m.Health().Status != "failing" {
		t.Fatal(m.Health())
	}
	_ = m.Valid(s, normalize.Stats{Dropped: map[string]int{}}, time.Now())
	if m.Health().Status != "ok" {
		t.Fatal(m.Health())
	}
}

func TestPageErrorsEscalateToFailing(t *testing.T) {
	f := &fake{}
	m := New(Config{BaseTopic: "x", HADiscovery: true}, f)
	for i := 0; i < 3; i++ {
		if err := m.PageError("no_header", time.Now()); err != nil {
			t.Fatal(err)
		}
	}
	if m.Health().ConsecutiveFailures != 3 || m.Health().Status != "failing" {
		t.Fatalf("health=%+v", m.Health())
	}
	s := invented()
	if err := m.Valid(s, normalize.Stats{Dropped: map[string]int{}}, time.Now()); err != nil {
		t.Fatal(err)
	}
	if m.Health().ConsecutiveFailures != 0 || m.Health().Status != "ok" {
		t.Fatalf("recovered health=%+v", m.Health())
	}
}
func TestDiscoveryContainsNoIncidentContent(t *testing.T) {
	f := &fake{}
	m := New(Config{BaseTopic: "x", ClientID: "c", HADiscovery: true, Version: "v"}, f)
	if err := m.PublishDiscovery(); err != nil {
		t.Fatal(err)
	}
	for _, c := range f.calls {
		if strings.Contains(string(c.payload), "Invented") || strings.Contains(string(c.payload), "address") || strings.Contains(string(c.payload), "cross") {
			t.Fatal(string(c.payload))
		}
	}
}
func TestExamples(t *testing.T) {
	f := &fake{}
	m := New(Config{BaseTopic: "x", ClientID: "c", HADiscovery: true, Version: "v"}, f)
	f.calls = nil
	_ = m.Valid(invented(), normalize.Stats{Dropped: map[string]int{}}, time.Date(2026, 9, 14, 14, 0, 0, 0, time.UTC))
	next := invented()
	next.Incidents = append([]model.Incident(nil), next.Incidents...)
	next.Incidents[0].Type.Raw = "FIRE-2 Invented Update"
	if err := m.Valid(next, normalize.Stats{Dropped: map[string]int{}}, time.Date(2026, 9, 14, 14, 1, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	_ = m.PublishDiscovery()
	seenEvent := false
	for _, c := range f.calls {
		t.Logf("%s %s", c.topic, string(c.payload))
		if c.topic == "x/incident" {
			seenEvent = true
		}
	}
	if !seenEvent {
		t.Fatal("asserting example event was not published")
	}
}

func TestOutputHelpersAndCategories(t *testing.T) {
	f := &fake{}
	m := New(Config{BaseTopic: "x", RawTopic: "raw", ClientID: "c", HADiscovery: true}, f)
	if err := m.Availability(true); err != nil {
		t.Fatal(err)
	}
	if err := m.Availability(false); err != nil {
		t.Fatal(err)
	}
	if err := m.Raw([]byte("html")); err != nil {
		t.Fatal(err)
	}
	if err := m.PageError("no_header", time.Now()); err != nil {
		t.Fatal(err)
	}
	for _, cat := range []model.Category{model.EMS, model.Police, model.Other, model.Unknown} {
		s := invented()
		s.Incidents[0].ID = string(cat)
		s.Incidents[0].Agency.Category = cat
		if err := m.Valid(s, normalize.Stats{Dropped: map[string]int{}}, time.Now()); err != nil {
			t.Fatal(err)
		}
	}
	if len(m.discovery()) != 6 {
		t.Fatal("discovery count")
	}
	if icon("fire") != "mdi:fire" || icon("ems") != "mdi:ambulance" || icon("police") != "mdi:police-badge" || icon("other") != "mdi:alert" {
		t.Fatal("icons")
	}
	countsSnapshot := model.Snapshot{Incidents: []model.Incident{
		{Agency: model.Agency{Category: model.Fire}}, {Agency: model.Agency{Category: model.EMS}},
		{Agency: model.Agency{Category: model.Police}}, {Agency: model.Agency{Category: model.Other}},
		{Agency: model.Agency{Category: model.Unknown}},
	}}
	if count(countsSnapshot).Total != 5 {
		t.Fatal("counts")
	}
}

func TestClosedAndNewEvents(t *testing.T) {
	f := &fake{}
	m := New(Config{BaseTopic: "x", HADiscovery: true}, f)
	s := invented()
	_ = m.Valid(s, normalize.Stats{Dropped: map[string]int{}}, time.Now())
	n := model.NewSnapshot(nil, time.Now(), nil)
	_ = m.Valid(n, normalize.Stats{Dropped: map[string]int{}}, time.Now())
	var closed bool
	for _, c := range f.calls {
		if c.topic != "x/incident" {
			continue
		}
		var e model.Event
		if err := json.Unmarshal(c.payload, &e); err != nil {
			t.Fatal(err)
		}
		closed = e.Event == model.Closed && e.Incident.Status == "closed"
	}
	if !closed {
		t.Fatal("missing closed event")
	}
}
