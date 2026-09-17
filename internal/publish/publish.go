// Package publish contains the schema-v1 MQTT output contract.
package publish

import (
	"bytes"
	"encoding/json"
	"time"

	"icad2mqtt/internal/diff"
	"icad2mqtt/internal/model"
	"icad2mqtt/internal/normalize"
)

// Publisher is deliberately smaller than an MQTT client so publishing can be unit tested.
type Publisher interface {
	Publish(topic string, qos byte, retained bool, payload []byte) error
}

type Config struct {
	BaseTopic, RawTopic, ClientID, Version string
	PublishRaw, HADiscovery                bool
}

type Health struct {
	Schema              int            `json:"schema"`
	Status              string         `json:"status"`
	LastSuccessAt       string         `json:"last_success_at"`
	LastErrorAt         string         `json:"last_error_at"`
	LastErrorReason     string         `json:"last_error_reason"`
	ConsecutiveFailures int            `json:"consecutive_failures"`
	PollsTotal          int            `json:"polls_total"`
	PageErrorsTotal     int            `json:"page_errors_total"`
	RowsSkippedTotal    int            `json:"rows_skipped_total"`
	RowsDroppedTotal    map[string]int `json:"rows_dropped_total"`
	DuplicatesTotal     int            `json:"duplicates_total"`
	IncidentsActive     int            `json:"incidents_active"`
	PageUpdatedAt       *string        `json:"page_updated_at"`
	Version             string         `json:"version"`
}

type Counts struct {
	Fire    int `json:"fire"`
	EMS     int `json:"ems"`
	Police  int `json:"police"`
	Other   int `json:"other"`
	Unknown int `json:"unknown"`
	Total   int `json:"total"`
}

type Manager struct {
	cfg          Config
	out          Publisher
	engine       *diff.Engine
	seeded       bool
	lastSnapshot []byte
	lastCounts   []byte
	health       Health
}

func New(cfg Config, out Publisher) *Manager {
	m := &Manager{cfg: cfg, out: out, engine: diff.New(), health: Health{Schema: 1, Status: "ok", RowsDroppedTotal: map[string]int{}, Version: cfg.Version}}
	if m.cfg.Version == "" {
		m.cfg.Version = "unknown"
		m.health.Version = m.cfg.Version
	}
	if !cfg.HADiscovery {
		m.clearDiscovery()
	}
	return m
}
func (m *Manager) Topic(s string) string { return m.cfg.BaseTopic + "/" + s }
func (m *Manager) publish(topic string, retained bool, payload []byte) error {
	return m.out.Publish(topic, 1, retained, payload)
}
func canonical(s model.Snapshot) ([]byte, error) { s.FetchedAt = ""; return json.Marshal(s) }

func (m *Manager) Valid(s model.Snapshot, stats normalize.Stats, now time.Time) error {
	m.health.PollsTotal++
	m.health.ConsecutiveFailures = 0
	m.health.LastSuccessAt = now.UTC().Format(time.RFC3339)
	m.health.IncidentsActive = len(s.Incidents)
	m.health.PageUpdatedAt = s.PageUpdatedAt
	m.health.RowsSkippedTotal += stats.SkippedRows
	m.health.DuplicatesTotal += stats.Duplicates
	for k, v := range stats.Dropped {
		m.health.RowsDroppedTotal[k] += v
	}
	m.health.Status = "ok"
	if stats.SkippedRows > 0 || len(stats.Dropped) > 0 || s.PageUpdatedAt == nil {
		m.health.Status = "degraded"
	}
	wasSeeded := m.seeded
	if !m.seeded {
		m.engine.Diff(&s, &s)
		m.seeded = true
	}
	events := []model.Event{}
	if wasSeeded {
		events = m.engine.Diff(nil, &s)
	}
	canonicalBytes, err := canonical(s)
	if err != nil {
		return err
	}
	if len(m.lastSnapshot) == 0 || !bytes.Equal(m.lastSnapshot, canonicalBytes) {
		if err = m.publish(m.Topic("incidents"), true, mustJSON(s)); err != nil {
			return err
		}
		m.lastSnapshot = canonicalBytes
	}
	for _, e := range events {
		b, x := json.Marshal(e)
		if x != nil {
			return x
		}
		if err = m.publish(m.Topic("incident"), false, b); err != nil {
			return err
		}
	}
	counts := count(s)
	cb, _ := json.Marshal(counts)
	if !bytes.Equal(m.lastCounts, cb) {
		if m.cfg.HADiscovery {
			if err = m.publish(m.Topic("counts"), true, cb); err != nil {
				return err
			}
		}
		m.lastCounts = cb
	}
	return m.publishHealth()
}
func (m *Manager) Failure(reason string, now time.Time) error {
	m.health.PollsTotal++
	m.health.ConsecutiveFailures++
	m.health.LastErrorAt = now.UTC().Format(time.RFC3339)
	m.health.LastErrorReason = reason
	if m.health.ConsecutiveFailures >= 3 {
		m.health.Status = "failing"
	} else {
		m.health.Status = "degraded"
	}
	return m.publishHealth()
}
func (m *Manager) PageError(reason string, now time.Time) error {
	m.health.PollsTotal++
	m.health.PageErrorsTotal++
	m.health.ConsecutiveFailures++
	m.health.LastErrorAt = now.UTC().Format(time.RFC3339)
	m.health.LastErrorReason = reason
	if m.health.ConsecutiveFailures >= 3 {
		m.health.Status = "failing"
	} else {
		m.health.Status = "degraded"
	}
	return m.publishHealth()
}
func (m *Manager) publishHealth() error {
	b, err := json.Marshal(m.health)
	if err != nil {
		return err
	}
	return m.publish(m.Topic("health"), true, b)
}
func mustJSON(v any) []byte { b, _ := json.Marshal(v); return b }

func count(s model.Snapshot) Counts {
	var c Counts
	for _, i := range s.Incidents {
		switch i.Agency.Category {
		case model.Fire:
			c.Fire++
		case model.EMS:
			c.EMS++
		case model.Police:
			c.Police++
		case model.Other:
			c.Other++
		default:
			c.Unknown++
		}
	}
	c.Total = len(s.Incidents)
	return c
}

type discovery struct {
	Name              string `json:"name"`
	UniqueID          string `json:"unique_id"`
	StateTopic        string `json:"state_topic"`
	ValueTemplate     string `json:"value_template"`
	AvailabilityTopic string `json:"availability_topic"`
	StateClass        string `json:"state_class"`
	Icon              string `json:"icon"`
	Device            device `json:"device"`
}
type device struct {
	Name        string   `json:"name"`
	Identifiers []string `json:"identifiers"`
	SWVersion   string   `json:"sw_version"`
}

func (m *Manager) clearDiscovery() {
	for _, c := range []string{"fire", "ems", "police", "other", "unknown", "total"} {
		_ = m.publish("homeassistant/sensor/icad2mqtt_"+m.cfg.ClientID+"/"+c+"_active/config", true, []byte{})
	}
}
func (m *Manager) discovery() []discovery {
	out := make([]discovery, 0, 6)
	for _, c := range []string{"fire", "ems", "police", "other", "unknown", "total"} {
		out = append(out, discovery{Name: "icad2mqtt " + c + " active", UniqueID: "icad2mqtt_" + m.cfg.ClientID + "_" + c + "_active", StateTopic: m.Topic("counts"), ValueTemplate: "{{ value_json." + c + " }}", AvailabilityTopic: m.Topic("availability"), StateClass: "measurement", Icon: icon(c), Device: device{Name: "icad2mqtt", Identifiers: []string{m.cfg.ClientID}, SWVersion: m.cfg.Version}})
	}
	return out
}
func icon(c string) string {
	switch c {
	case "fire":
		return "mdi:fire"
	case "ems":
		return "mdi:ambulance"
	case "police":
		return "mdi:police-badge"
	default:
		return "mdi:alert"
	}
}
func (m *Manager) PublishDiscovery() error {
	if !m.cfg.HADiscovery {
		return nil
	}
	for _, d := range m.discovery() {
		b, _ := json.Marshal(d)
		topic := "homeassistant/sensor/icad2mqtt_" + m.cfg.ClientID + "/" + d.UniqueID[len("icad2mqtt_"+m.cfg.ClientID)+1:] + "/config"
		if err := m.publish(topic, true, b); err != nil {
			return err
		}
	}
	return nil
}
func (m *Manager) Availability(online bool) error {
	v := "offline"
	if online {
		v = "online"
	}
	return m.publish(m.Topic("availability"), true, []byte(v))
}
func (m *Manager) Raw(body []byte) error {
	if !m.cfg.PublishRaw {
		return nil
	}
	return m.publish(m.cfg.RawTopic, false, body)
}
func (m *Manager) Health() Health { return m.health }
