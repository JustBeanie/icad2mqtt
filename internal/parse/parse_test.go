package parse

import (
	"golang.org/x/net/html"
	"os"
	"strings"
	"testing"
)

func fixture(t *testing.T) string {
	b, e := os.ReadFile("../../testdata/events_all.html")
	if e != nil {
		t.Fatal(e)
	}
	return string(b)
}
func TestParseFixture(t *testing.T) {
	r, e := Parse(fixture(t))
	if e != nil || len(r.Rows) != 11 || r.UpdatedRaw != "Sunday, September 13, 2026 12:10 AM" {
		t.Fatalf("%+v %v", r, e)
	}
}
func TestParseFailuresAndCleanup(t *testing.T) {
	for _, x := range []struct {
		s      string
		reason Reason
	}{{"<table><tr><th>Nope</th></tr></table>", NoHeader}, {"<table><tr><th>Agency</th><th>Date/Time</th><th>x</th><th>Address</th><th>x</th><th>Cross Streets</th></tr><tr><th>Agency</th><th>Date/Time</th><th>x</th><th>Address</th><th>y</th><th>Cross Streets</th></tr></table>", ConflictingHeader}} {
		_, e := Parse(x.s)
		if e == nil || e.(*PageError).Reason != x.reason {
			t.Errorf("%v", e)
		}
	}
	if Clean(" A\u00a0\t B ") != "A B" {
		t.Fail()
	}
}

func TestRowShapes(t *testing.T) {
	root, err := html.Parse(strings.NewReader(fixture(t)))
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range rows(root) {
		if len(row) > 6 {
			t.Logf("shape=%d first=%q second=%q", len(row), row[0], row[1])
		}
	}
}
