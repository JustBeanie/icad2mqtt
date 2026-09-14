package normalize

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"icad2mqtt/internal/diff"
	"icad2mqtt/internal/model"
	"icad2mqtt/internal/parse"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var updateGoldens = flag.Bool("update", false, "update fixture golden JSON")

func readFixture(t *testing.T, name string) string {
	t.Helper()
	b, e := os.ReadFile(filepath.Join("..", "..", "testdata", name))
	if e != nil {
		t.Fatal(e)
	}
	return string(b)
}
func pageFixture(t *testing.T, name string) (model.Snapshot, Stats) {
	t.Helper()
	r, e := parse.Parse(readFixture(t, name))
	if e != nil {
		t.Fatal(e)
	}
	return Page(r, time.Date(2026, 9, 13, 4, 11, 0, 0, time.UTC))
}
func TestFixturePages(t *testing.T) {
	tests := []struct {
		name                     string
		rows, incidents, skipped int
		warning                  bool
		drop                     string
		dropCount, duplicates    int
	}{
		{"events_all.html", 11, 10, 0, false, "invalid_time", 1, 0}, {"events_empty.html", 0, 0, 0, true, "", 0, 0}, {"events_short_row.html", 10, 9, 1, false, "invalid_time", 1, 0}, {"events_entities.html", 11, 10, 0, false, "invalid_time", 1, 0}, {"events_spring_gap.html", 11, 9, 0, false, "nonexistent_local_time", 1, 0}, {"events_fall_ambiguous.html", 11, 10, 0, false, "invalid_time", 1, 0}, {"events_duplicate.html", 12, 10, 0, false, "invalid_time", 1, 1}, {"events_type_changed.html", 11, 10, 0, false, "invalid_time", 1, 0}}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s, stats := pageFixture(t, tc.name)
			r, _ := parse.Parse(readFixture(t, tc.name))
			if len(r.Rows) != tc.rows || len(s.Incidents) != tc.incidents || stats.SkippedRows != tc.skipped {
				t.Fatalf("rows=%d incidents=%d stats=%+v", len(r.Rows), len(s.Incidents), stats)
			}
			if tc.warning != (len(stats.Warnings) == 1) {
				t.Fatalf("warnings=%v", stats.Warnings)
			}
			if tc.drop != "" && stats.Dropped[tc.drop] != tc.dropCount {
				t.Fatalf("drops=%v", stats.Dropped)
			}
			if stats.Duplicates != tc.duplicates {
				t.Fatalf("duplicates=%d", stats.Duplicates)
			}
			if tc.name == "events_all.html" && (s.PageUpdatedAt == nil || *s.PageUpdatedAt != "2026-09-13T00:10:00-04:00") {
				t.Fatalf("updated=%v", s.PageUpdatedAt)
			}
			if tc.name == "events_fall_ambiguous.html" {
				found := false
				for _, incident := range s.Incidents {
					if incident.ReceivedAtRaw == "11/01/26 01:30" {
						found = incident.ReceivedAt == "2026-11-01T01:30:00-04:00"
					}
				}
				if !found {
					t.Fatalf("ambiguous row not earlier EDT: %+v", s.Incidents)
				}
			}
			if tc.name == "events_entities.html" {
				found := false
				for _, incident := range s.Incidents {
					if strings.Contains(incident.Agency.Name, "&") {
						found = true
					}
				}
				if !found {
					t.Fatalf("entity decode missing: %+v", s.Incidents)
				}
			}
		})
	}
}
func TestMissingHeaderAndInvalidUpdated(t *testing.T) {
	if _, err := parse.Parse(readFixture(t, "events_missing_header.html")); err == nil || err.(*parse.PageError).Reason != parse.NoHeader {
		t.Fatalf("err=%v", err)
	}
	r, err := parse.Parse(strings.Replace(readFixture(t, "events_all.html"), "Sunday, September 13, 2026 12:10 AM", "not a timestamp", 1))
	if err != nil {
		t.Fatal(err)
	}
	s, stats := Page(r, time.Unix(0, 0))
	if s.PageUpdatedAt != nil || len(stats.Warnings) != 1 || stats.Warnings[0] != "page_updated_at_unparsable" {
		t.Fatalf("snapshot=%+v stats=%+v", s, stats)
	}
	if s.Incidents[0].Municipality.Name != nil {
		t.Fatal("municipality name must be null")
	}
}
func TestGoldens(t *testing.T) {
	for _, tc := range []struct{ fixture, golden string }{{"events_all.html", "golden_normal.json"}, {"events_empty.html", "golden_snapshot.json"}} {
		s, _ := pageFixture(t, tc.fixture)
		got, err := json.Marshal(s)
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join("..", "..", "testdata", tc.golden)
		if *updateGoldens {
			if err := os.WriteFile(path, got, 0644); err != nil {
				t.Fatal(err)
			}
		}
		want, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != string(want) {
			t.Fatalf("%s differs; run with -update", tc.golden)
		}
	}
	incident := model.Incident{ID: "0123456789abcdef", Agency: Agency("Example County Fire"), ReceivedAt: "2026-09-12T08:39:00-04:00", ReceivedAtRaw: "09/12/26 08:39", Type: Type("STRUCTURE FIRE Residential"), AddressRaw: "100 INVENTED STREET", AddressClean: "100 INVENTED STREET", Municipality: model.Municipality{Raw: "XA1"}, CrossStreetsRaw: "FICTION AVE & SAMPLE RD", CrossStreets: []string{"FICTION AVE", "SAMPLE RD"}, Status: "active"}
	got, _ := json.Marshal(model.Event{Schema: 1, Event: model.New, Incident: incident})
	path := filepath.Join("..", "..", "testdata", "golden_event_new.json")
	if *updateGoldens {
		if err := os.WriteFile(path, got, 0644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) {
		t.Fatal("golden_event_new.json differs")
	}
}
func TestCategoriesAndID(t *testing.T) {
	for _, tc := range []struct {
		name     string
		category model.Category
	}{{"Syracuse Fire Department", model.Fire}, {"Geddes Police", model.Police}, {"Syracuse Police", model.Police}, {"GBAC Ambulance", model.EMS}, {"Dewitt Police", model.Police}, {"EMS", model.EMS}, {"Medical", model.EMS}, {"State Police", model.Police}, {"Sheriff", model.Police}, {"Firestone Plaza Security", model.Unknown}, {"Emsworth Shop", model.Unknown}, {"Fire & EMS", model.Fire}, {"Fire Police", model.Fire}} {
		if got := Agency(tc.name); got.Category != tc.category {
			t.Errorf("%s=%s", tc.name, got.Category)
		}
	}
	row := []string{"Syracuse Police", "09/12/26 23:18", "MOTOR VEHICLE ACCIDENT", "22 PLACEHOLDER LN", "XB2", "TEST RD"}
	inc, err := Incident(row)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256([]byte("syracuse police|2026-09-12T23:18:00-04:00|22 PLACEHOLDER LN|XB2"))
	if inc.ID != hex.EncodeToString(sum[:])[:16] {
		t.Fatalf("id=%s", inc.ID)
	}
}
func TestTimezoneLoaderFailure(t *testing.T) {
	restore := SetLocationLoaderForTest(func(string) (*time.Location, error) { return nil, errors.New("missing zone") })
	defer restore()
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("panic=%v", r)
		}
	}()
	if _, err := ParseTime("09/12/26 23:18"); err == nil {
		t.Fatal("expected typed error")
	} else if _, ok := err.(*LocationError); !ok {
		t.Fatalf("%T", err)
	}
}
func TestDiffUsesPages(t *testing.T) {
	prev, _ := pageFixture(t, "events_all.html")
	next, _ := pageFixture(t, "events_type_changed.html")
	events := diff.Diff(&prev, &next)
	if len(events) != 1 || events[0].Event != model.Updated {
		t.Fatalf("events=%+v", events)
	}
	changed := next
	changed.Incidents[0].CrossStreetsRaw = "ANOTHER RD"
	events = diff.Diff(&prev, &changed)
	if len(events) != 1 || events[0].Event != model.Updated {
		t.Fatalf("cross=%+v", events)
	}
	left := prev
	left.Incidents = left.Incidents[1:]
	events = diff.Diff(&prev, &left)
	if len(events) != 1 || events[0].Event != model.Closed || events[0].Incident.Status != "closed" {
		t.Fatalf("closed=%+v", events)
	}
	if events = diff.Diff(&left, &left); len(events) != 0 {
		t.Fatalf("repeated close=%+v", events)
	}
	if _, err := parse.Parse(readFixture(t, "events_missing_header.html")); err == nil {
		t.Fatal("failed page must not be diffed")
	}
}
