package publish

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"icad2mqtt/internal/model"
	"icad2mqtt/internal/normalize"
	"icad2mqtt/internal/parse"
)

func TestManagerFixtureSequence(t *testing.T) {
	read := func(name string) parse.Result {
		b, err := os.ReadFile("../../testdata/" + name)
		if err != nil {
			t.Fatal(err)
		}
		r, err := parse.Parse(string(b))
		if err != nil {
			t.Fatal(err)
		}
		return r
	}
	all := read("events_all.html")
	changed := read("events_type_changed.html")
	allPage, allStats := normalize.Page(all, time.Now())
	changedPage, changedStats := normalize.Page(changed, time.Now())
	changedID := ""
	for _, i := range allPage.Incidents {
		for _, j := range changedPage.Incidents {
			if i.ID == j.ID && i.Type.Raw != j.Type.Raw {
				changedID = i.ID
			}
		}
	}
	if changedID == "" {
		t.Fatal("fixture has no type change")
	}
	removed := all
	copied := make([][]string, len(all.Rows))
	copy(copied, all.Rows)
	removed.Rows = copied
	for i, row := range removed.Rows {
		inc, err := normalize.Incident(row)
		if err == nil && inc.ID == changedID {
			removed.Rows = append(removed.Rows[:i], removed.Rows[i+1:]...)
			break
		}
	}
	removedPage, removedStats := normalize.Page(removed, time.Now())
	f := &fake{}
	m := New(Config{BaseTopic: "x", HADiscovery: true, ClientID: "c"}, f)
	valid := func(s model.Snapshot, st normalize.Stats) {
		if err := m.Valid(s, st, time.Now()); err != nil {
			t.Fatal(err)
		}
	}
	valid(allPage, allStats)
	if events(f) != 0 || countTopic(f, "x/incidents") != 1 {
		t.Fatalf("seed calls=%+v", f.calls)
	}
	f.calls = nil
	valid(changedPage, changedStats)
	if events(f) != 1 || eventKind(f) != model.Updated || eventID(f) != changedID {
		t.Fatalf("update calls=%+v", f.calls)
	}
	f.calls = nil
	valid(removedPage, removedStats)
	if events(f) != 1 || eventKind(f) != model.Closed || eventID(f) != changedID || eventStatus(f) != "closed" {
		t.Fatalf("close calls=%+v", f.calls)
	}
	f.calls = nil
	valid(removedPage, removedStats)
	if events(f) != 0 {
		t.Fatalf("repeat calls=%+v", f.calls)
	}
}
func countTopic(f *fake, topic string) int {
	n := 0
	for _, c := range f.calls {
		if c.topic == topic {
			n++
		}
	}
	return n
}
func events(f *fake) int { return countTopic(f, "x/incident") }
func decodedEvent(f *fake) model.Event {
	for _, c := range f.calls {
		if c.topic == "x/incident" {
			var e model.Event
			_ = json.Unmarshal(c.payload, &e)
			return e
		}
	}
	return model.Event{}
}
func eventKind(f *fake) model.EventKind { return decodedEvent(f).Event }
func eventID(f *fake) string            { return decodedEvent(f).Incident.ID }
func eventStatus(f *fake) string        { return decodedEvent(f).Incident.Status }
