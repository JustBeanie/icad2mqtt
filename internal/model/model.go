package model

import (
	"encoding/json"
	"sort"
	"time"
)

type Category string

const (
	Fire    Category = "fire"
	EMS     Category = "ems"
	Police  Category = "police"
	Other   Category = "other"
	Unknown Category = "unknown"
)

type Agency struct {
	Name           string   `json:"name"`
	Key            string   `json:"key"`
	Category       Category `json:"category"`
	CategorySource string   `json:"category_source"`
}
type Type struct {
	Raw  string  `json:"raw"`
	Key  string  `json:"key"`
	Code *string `json:"code,omitempty"`
}
type Municipality struct {
	Raw  string  `json:"raw"`
	Name *string `json:"name"`
}
type Incident struct {
	ID              string       `json:"id"`
	Agency          Agency       `json:"agency"`
	ReceivedAt      string       `json:"received_at"`
	ReceivedAtRaw   string       `json:"received_at_raw"`
	Type            Type         `json:"type"`
	AddressRaw      string       `json:"address_raw"`
	AddressClean    string       `json:"address_clean"`
	Municipality    Municipality `json:"municipality"`
	CrossStreetsRaw string       `json:"cross_streets_raw"`
	CrossStreets    []string     `json:"cross_streets"`
	Status          string       `json:"status"`
}
type Snapshot struct {
	Schema        int        `json:"schema"`
	Source        string     `json:"source"`
	PageUpdatedAt *string    `json:"page_updated_at"`
	FetchedAt     string     `json:"fetched_at"`
	Incidents     []Incident `json:"incidents"`
}
type EventKind string

const (
	New     EventKind = "new"
	Updated EventKind = "updated"
	Closed  EventKind = "closed"
)

type Event struct {
	Schema   int       `json:"schema"`
	Event    EventKind `json:"event"`
	Incident Incident  `json:"incident"`
}

func SortIncidents(s *Snapshot) {
	sort.SliceStable(s.Incidents, func(i, j int) bool {
		if s.Incidents[i].ReceivedAt != s.Incidents[j].ReceivedAt {
			return s.Incidents[i].ReceivedAt < s.Incidents[j].ReceivedAt
		}
		return s.Incidents[i].ID < s.Incidents[j].ID
	})
}
func NewSnapshot(updated *time.Time, fetched time.Time, incidents []Incident) Snapshot {
	if incidents == nil {
		incidents = []Incident{}
	}
	var u *string
	if updated != nil {
		x := updated.Format(time.RFC3339)
		u = &x
	}
	s := Snapshot{1, "ongov-911events", u, fetched.UTC().Format(time.RFC3339), incidents}
	SortIncidents(&s)
	return s
}
func JSON(v any) ([]byte, error) { return json.Marshal(v) }
