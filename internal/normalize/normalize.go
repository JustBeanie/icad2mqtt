package normalize

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"icad2mqtt/internal/model"
	"icad2mqtt/internal/parse"
	"regexp"
	"strings"
	"time"
	_ "time/tzdata"
	"unicode"
)

const zoneName = "America/New_York"

var locationLoader = time.LoadLocation
var nyLocation *time.Location
var locationErr error

func init()               { initializeLocation() }
func initializeLocation() { nyLocation, locationErr = locationLoader(zoneName) }

type LocationError struct{ Err error }

func (e *LocationError) Error() string { return fmt.Sprintf("load %s: %v", zoneName, e.Err) }
func (e *LocationError) Unwrap() error { return e.Err }

var rules = []struct {
	cat   model.Category
	words []string
}{
	{model.Fire, []string{"fire"}},
	{model.EMS, []string{"ems", "ambulance", "medical", "rescue squad"}},
	{model.Police, []string{"police", "sheriff", "state police", "troop"}},
}
var ruleRE = map[string]*regexp.Regexp{}

func init() {
	for _, rule := range rules {
		for _, word := range rule.words {
			ruleRE[word] = regexp.MustCompile("(?i)(^|[^\\pL])" + regexp.QuoteMeta(word) + "([^\\pL]|$)")
		}
	}
}
func Rules() []string {
	var result []string
	for _, rule := range rules {
		result = append(result, string(rule.cat)+":"+strings.Join(rule.words, ","))
	}
	return result
}
func Agency(name string) model.Agency {
	name = clean(name)
	category := model.Unknown
	for _, rule := range rules {
		for _, word := range rule.words {
			if ruleRE[word].MatchString(strings.ToLower(name)) {
				category = rule.cat
				break
			}
		}
		if category != model.Unknown {
			break
		}
	}
	return model.Agency{Name: name, Key: strings.ToLower(name), Category: category, CategorySource: "agency_name"}
}
func clean(s string) string {
	var b strings.Builder
	space := false
	for _, r := range s {
		if unicode.IsSpace(r) {
			space = true
			continue
		}
		if space && b.Len() > 0 {
			b.WriteByte(' ')
		}
		space = false
		b.WriteRune(r)
	}
	return strings.TrimSpace(b.String())
}
func Type(raw string) model.Type { n := clean(raw); return model.Type{Raw: n, Key: key(n)} }
func key(s string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			dash = false
		} else if !dash && b.Len() > 0 {
			b.WriteByte('-')
			dash = true
		}
	}
	return strings.Trim(b.String(), "-")
}

var ErrNonexistent = errors.New("nonexistent_local_time")

func checkLocation() error {
	if locationErr != nil || nyLocation == nil {
		if locationErr == nil {
			locationErr = errors.New("location is nil")
		}
		return &LocationError{locationErr}
	}
	return nil
}
func ParseTime(raw string) (time.Time, error) {
	if err := checkLocation(); err != nil {
		return time.Time{}, err
	}
	raw = clean(raw)
	t, err := time.ParseInLocation("01/02/06 15:04", raw, nyLocation)
	if err != nil {
		return time.Time{}, err
	}
	if t.In(nyLocation).Format("01/02/06 15:04") != raw {
		return time.Time{}, ErrNonexistent
	}
	return t, nil
}
func Updated(raw string) (*time.Time, error) {
	if err := checkLocation(); err != nil {
		return nil, err
	}
	t, err := time.ParseInLocation("Monday, January 2, 2006 3:04 PM", clean(raw), nyLocation)
	if err != nil {
		return nil, err
	}
	if t.In(nyLocation).Format("Monday, January 2, 2006 3:04 PM") != clean(raw) {
		return nil, ErrNonexistent
	}
	return &t, nil
}

type Stats struct {
	SkippedRows int
	Dropped     map[string]int
	Duplicates  int
	Warnings    []string
}

func Incident(row []string) (model.Incident, error) {
	if len(row) != 6 {
		return model.Incident{}, errors.New("invalid_row")
	}
	t, err := ParseTime(row[1])
	if err != nil {
		return model.Incident{}, err
	}
	agency := Agency(row[0])
	address := strings.ToUpper(clean(row[3]))
	received := t.Format(time.RFC3339)
	municipality := clean(row[4])
	hash := sha256.Sum256([]byte(agency.Key + "|" + received + "|" + address + "|" + municipality))
	crossRaw := clean(row[5])
	cross := []string{}
	for _, value := range strings.Split(crossRaw, " & ") {
		if value = clean(value); value != "" {
			cross = append(cross, value)
		}
	}
	return model.Incident{ID: hex.EncodeToString(hash[:])[:16], Agency: agency, ReceivedAt: received, ReceivedAtRaw: clean(row[1]), Type: Type(row[2]), AddressRaw: clean(row[3]), AddressClean: address, Municipality: model.Municipality{Raw: municipality}, CrossStreetsRaw: crossRaw, CrossStreets: cross, Status: "active"}, nil
}
func Normalize(rows [][]string) ([]model.Incident, Stats) {
	result := []model.Incident{}
	stats := Stats{Dropped: map[string]int{}, Warnings: []string{}}
	seen := map[string]bool{}
	for _, row := range rows {
		incident, err := Incident(row)
		if err != nil {
			reason := "invalid_time"
			if errors.Is(err, ErrNonexistent) {
				reason = ErrNonexistent.Error()
			}
			stats.Dropped[reason]++
			continue
		}
		if seen[incident.ID] {
			stats.Duplicates++
			continue
		}
		seen[incident.ID] = true
		result = append(result, incident)
	}
	return result, stats
}

// Page converts a successful parse into schema v1. Callers must avoid calling
// it for a parse error, thereby retaining the previous valid snapshot.
func Page(result parse.Result, fetchedAt time.Time) (model.Snapshot, Stats) {
	incidents, stats := Normalize(result.Rows)
	stats.SkippedRows = result.SkippedRows
	var updated *time.Time
	if result.UpdatedRaw != "" {
		updated, _ = Updated(result.UpdatedRaw)
	}
	if updated == nil {
		stats.Warnings = append(stats.Warnings, "page_updated_at_unparsable")
	}
	return model.NewSnapshot(updated, fetchedAt, incidents), stats
}
func SetLocationLoaderForTest(loader func(string) (*time.Location, error)) func() {
	old := locationLoader
	locationLoader = loader
	initializeLocation()
	return func() { locationLoader = old; initializeLocation() }
}
