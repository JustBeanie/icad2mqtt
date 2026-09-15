package fetch

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestFetchHeadersDecodeHashDateAndStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.UserAgent() != "test-agent" {
			t.Errorf("UA=%q", r.UserAgent())
		}
		w.Header().Set("Date", "Tue, 15 Nov 1994 08:12:31 GMT")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte{0x63, 0x61, 0x66, 0xe9})
	}))
	defer server.Close()
	f := &Fetcher{Client: server.Client(), URL: server.URL, UserAgent: "test-agent", PollInterval: time.Minute}
	got, err := f.Fetch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	wantBody := "caf\u00e9"
	if got.Body != wantBody || got.Hash != sha256.Sum256([]byte(wantBody)) || got.Date.IsZero() {
		t.Fatalf("response=%+v", got)
	}

	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
		_, _ = w.Write([]byte("secret page"))
	}))
	defer bad.Close()
	f.URL = bad.URL
	if _, err = f.Fetch(context.Background()); err == nil || strings.Contains(err.Error(), "secret page") {
		t.Fatalf("bounded error=%v", err)
	}
}

func TestFetchMissingOrInvalidDateH(t *testing.T) {
	for _, header := range []string{"", "not-a-date"} {
		f := &Fetcher{Client: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			h := make(http.Header)
			if header != "" {
				h.Set("Date", header)
			}
			return &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Header: h, Body: io.NopCloser(strings.NewReader("ok")), Request: r}, nil
		})}, URL: "http://example.test", PollInterval: time.Minute}
		got, err := f.Fetch(context.Background())
		if err != nil || !got.Date.IsZero() {
			t.Fatalf("header %q: response=%+v err=%v", header, got, err)
		}
	}
	f := &Fetcher{URL: "://bad", PollInterval: time.Minute}
	if _, err := f.Fetch(context.Background()); err == nil || f.Failures != 1 {
		t.Fatalf("bad URL err=%v failures=%d", err, f.Failures)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestFetchRejectsOversize(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write(make([]byte, MaxResponseSize+1)) }))
	defer s.Close()
	f := &Fetcher{Client: s.Client(), URL: s.URL, PollInterval: time.Minute}
	if _, err := f.Fetch(context.Background()); err == nil {
		t.Fatal("expected oversize")
	}
}

func TestFetchDefaultClientAndReadError(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("ok")) }))
	f := &Fetcher{URL: s.URL}
	if got, err := f.Fetch(context.Background()); err != nil || got.Body != "ok" {
		t.Fatalf("default client response=%+v err=%v", got, err)
	}
	s.Close()
	f = &Fetcher{Client: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Body: errorReader{}, Header: make(http.Header), Request: r}, nil
	})}, URL: "http://example.test"}
	if _, err := f.Fetch(context.Background()); err == nil || f.Failures != 1 {
		t.Fatalf("read error=%v failures=%d", err, f.Failures)
	}
}

type errorReader struct{}

func (errorReader) Read([]byte) (int, error) { return 0, fmt.Errorf("read failed") }
func (errorReader) Close() error             { return nil }

func TestBackoffNilRandHasRangeAndVaries(t *testing.T) {
	f := &Fetcher{PollInterval: time.Minute, Failures: 1}
	seen := map[time.Duration]bool{}
	for i := 0; i < 100; i++ {
		got := f.BackoffDelay()
		if got < time.Minute || got >= 2*time.Minute {
			t.Fatalf("delay=%s", got)
		}
		seen[got] = true
	}
	if len(seen) < 2 {
		t.Fatalf("nil random source produced one value: %v", seen)
	}
}

func TestBackoffInjectedRandBoundsAndCap(t *testing.T) {
	f := &Fetcher{PollInterval: time.Minute, Failures: 1, Rand: func(n int64) int64 { return 0 }}
	if got := f.BackoffDelay(); got != time.Minute {
		t.Fatalf("minimum=%s", got)
	}
	f.Rand = func(n int64) int64 { return n - 1 }
	if got := f.BackoffDelay(); got != 2*time.Minute-1 {
		t.Fatalf("maximum=%s", got)
	}
	f.Failures = 10
	if got := f.BackoffDelay(); got != 10*time.Minute-1 {
		t.Fatalf("cap=%s", got)
	}
	f.PollInterval = 30 * time.Second
	if got := f.BackoffDelay(); got < time.Minute {
		t.Fatalf("floor=%s", got)
	}
}

func TestFailuresResetAfterSuccessD(t *testing.T) {
	count := 0
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count++
		if count < 3 {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		_, _ = w.Write([]byte("ok"))
	}))
	defer s.Close()
	f := &Fetcher{Client: s.Client(), URL: s.URL, PollInterval: time.Minute}
	_, _ = f.Fetch(context.Background())
	_, _ = f.Fetch(context.Background())
	if f.Failures != 2 {
		t.Fatalf("failures=%d", f.Failures)
	}
	if _, err := f.Fetch(context.Background()); err != nil || f.Failures != 0 {
		t.Fatalf("success err=%v failures=%d", err, f.Failures)
	}
	if got := f.BackoffDelay(); got != time.Minute {
		t.Fatalf("reset delay=%s", got)
	}
}

func TestClockInjection(t *testing.T) {
	want := time.Unix(123, 0)
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("ok")) }))
	defer s.Close()
	f := &Fetcher{Client: s.Client(), URL: s.URL, Clock: func() time.Time { return want }}
	if _, err := f.Fetch(context.Background()); err != nil || !f.LastFetched.Equal(want) {
		t.Fatalf("clock=%s err=%v", f.LastFetched, err)
	}
}
