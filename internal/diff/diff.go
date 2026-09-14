package diff

import (
	"icad2mqtt/internal/model"
	"sort"
)

type Engine struct {
	seeded   bool
	previous map[string]model.Incident
}

func New() *Engine { return &Engine{previous: map[string]model.Incident{}} }
func (e *Engine) Diff(prev, next *model.Snapshot) []model.Event {
	if prev == nil {
		prev = &model.Snapshot{}
	}
	if !e.seeded {
		e.previous = incidents(prev)
		e.seeded = true
		if len(prev.Incidents) == 0 {
			e.previous = map[string]model.Incident{}
		}
		return nil
	}
	return e.compare(next)
}
func incidents(s *model.Snapshot) map[string]model.Incident {
	m := map[string]model.Incident{}
	if s != nil {
		for _, i := range s.Incidents {
			m[i.ID] = i
		}
	}
	return m
}
func (e *Engine) compare(next *model.Snapshot) []model.Event {
	cur := incidents(next)
	out := []model.Event{}
	for id, n := range cur {
		old, ok := e.previous[id]
		if !ok {
			out = append(out, model.Event{Schema: 1, Event: model.New, Incident: n})
		} else if old.Type.Raw != n.Type.Raw || old.CrossStreetsRaw != n.CrossStreetsRaw {
			out = append(out, model.Event{Schema: 1, Event: model.Updated, Incident: n})
		}
	}
	for id, old := range e.previous {
		if _, ok := cur[id]; !ok {
			old.Status = "closed"
			out = append(out, model.Event{Schema: 1, Event: model.Closed, Incident: old})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Incident.ID < out[j].Incident.ID })
	e.previous = cur
	return out
}
func Diff(prev, next *model.Snapshot) []model.Event {
	e := New()
	if prev == nil {
		return e.Diff(nil, next)
	}
	e.previous = incidents(prev)
	e.seeded = true
	return e.compare(next)
}
