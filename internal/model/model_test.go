package model

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestGoldenShapeAndOrder(t *testing.T) {
	s := NewSnapshot(nil, time.Unix(0, 0), []Incident{{ID: "b", ReceivedAt: "2026-01-02T00:00:00Z"}, {ID: "a", ReceivedAt: "2026-01-01T00:00:00Z"}})
	b, e := json.Marshal(s)
	if e != nil || !strings.Contains(string(b), `"page_updated_at":null`) || s.Incidents[0].ID != "a" {
		t.Fatal(string(b), e)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	if m["schema"].(float64) != 1 {
		t.Fatal(m)
	}
	eb, _ := json.Marshal(Event{Schema: 1, Event: New, Incident: s.Incidents[0]})
	if !strings.Contains(string(eb), `"event":"new"`) {
		t.Fatal(string(eb))
	}
}

func TestSortTieJSONAndHelpers(t *testing.T) {
	s := Snapshot{Incidents: []Incident{{ID: "b", ReceivedAt: "2026-01-01T00:00:00Z"}, {ID: "a", ReceivedAt: "2026-01-01T00:00:00Z"}}}
	SortIncidents(&s)
	if s.Incidents[0].ID != "a" {
		t.Fatal(s.Incidents)
	}
	if _, err := JSON(Event{Schema: 1, Event: Closed, Incident: s.Incidents[0]}); err != nil {
		t.Fatal(err)
	}
	if got := NewSnapshot(nil, time.Unix(0, 0), nil); got.Incidents == nil {
		t.Fatal("nil incident list")
	}
	u := time.Date(2026, 9, 13, 0, 0, 0, 0, time.UTC)
	if got := NewSnapshot(&u, u, nil); got.PageUpdatedAt == nil || *got.PageUpdatedAt != "2026-09-13T00:00:00Z" {
		t.Fatal(got.PageUpdatedAt)
	}
}
