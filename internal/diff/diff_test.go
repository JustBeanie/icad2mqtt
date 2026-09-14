package diff

import (
	"icad2mqtt/internal/model"
	"testing"
)

func inc(id, typ, x string) model.Incident {
	return model.Incident{ID: id, Type: model.Type{Raw: typ}, CrossStreetsRaw: x, Status: "active"}
}
func snap(xs ...model.Incident) *model.Snapshot { return &model.Snapshot{Schema: 1, Incidents: xs} }
func TestEngineSequence(t *testing.T) {
	e := New()
	if got := e.Diff(nil, snap(inc("a", "t", "x"))); len(got) != 0 {
		t.Fatal(got)
	}
	got := e.Diff(nil, snap(inc("a", "t2", "x"), inc("b", "t", "x")))
	if len(got) != 2 {
		t.Fatal(got)
	}
	got = e.Diff(nil, snap(inc("b", "t", "x")))
	if len(got) != 1 || got[0].Event != model.Closed || got[0].Incident.Status != "closed" {
		t.Fatal(got)
	}
	if got = e.Diff(nil, snap(inc("b", "t", "x"))); len(got) != 0 {
		t.Fatal(got)
	}
}
func TestFunction(t *testing.T) {
	if got := Diff(nil, snap(inc("a", "t", "x"))); len(got) != 0 {
		t.Fatal(got)
	}
	got := Diff(snap(inc("a", "t", "x")), snap())
	if len(got) != 1 || got[0].Event != model.Closed {
		t.Fatal(got)
	}
}
