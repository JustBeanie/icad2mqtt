package fetch

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	rand "math/rand/v2"
	"net/http"
	"strings"
	"time"
)

const MaxResponseSize = 5 << 20

type Response struct {
	Body string
	Hash [32]byte
	Date time.Time
}
type Fetcher struct {
	Client         *http.Client
	URL, UserAgent string
	PollInterval   time.Duration
	Failures       int
	Rand           func(int64) int64
	Clock          func() time.Time
	LastFetched    time.Time
}

func (f *Fetcher) Fetch(ctx context.Context) (Response, error) {
	if f.Clock != nil {
		f.LastFetched = f.Clock()
	}
	client := f.Client
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, f.URL, nil)
	if err != nil {
		f.Failures++
		return Response{}, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("User-Agent", f.UserAgent)
	resp, err := client.Do(req)
	if err != nil {
		f.Failures++
		return Response{}, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		f.Failures++
		return Response{}, fmt.Errorf("unexpected HTTP status: %s", resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, MaxResponseSize+1))
	if err != nil {
		f.Failures++
		return Response{}, fmt.Errorf("read response: %w", err)
	}
	if len(body) > MaxResponseSize {
		f.Failures++
		return Response{}, fmt.Errorf("response exceeds %d bytes", MaxResponseSize)
	}
	decoded := iso88591(body)
	result := Response{Body: decoded, Hash: sha256.Sum256([]byte(decoded))}
	if resp.Header.Get("Date") != "" {
		result.Date, _ = http.ParseTime(resp.Header.Get("Date"))
	}
	f.Failures = 0
	return result, nil
}

func (f *Fetcher) BackoffDelay() time.Duration {
	base := f.PollInterval
	if base < time.Minute {
		base = time.Minute
	}
	capDelay := base
	for i := 0; i < f.Failures; i++ {
		if capDelay >= 10*time.Minute/2 {
			capDelay = 10 * time.Minute
			break
		}
		capDelay *= 2
	}
	if capDelay > 10*time.Minute {
		capDelay = 10 * time.Minute
	}
	span := capDelay - base
	if span <= 0 {
		return base
	}
	random := f.Rand
	if random == nil {
		random = rand.Int64N
	}
	return base + time.Duration(random(int64(span)))
}

func iso88591(b []byte) string {
	var s strings.Builder
	s.Grow(len(b))
	for _, v := range b {
		s.WriteRune(rune(v))
	}
	return s.String()
}
