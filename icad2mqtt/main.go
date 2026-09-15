package main

import (
	"context"
	"fmt"
	"io"
	"log"
	rand "math/rand/v2"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"
	_ "time/tzdata"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

const eventURL = "https://911events.ongov.net/CADInet/app/events.jsp"

func main() {
	broker := env("MQTT_BROKER", "tcp://localhost:1883")
	baseTopic := env("MQTT_BASE_TOPIC", "911/cad")
	topic := env("MQTT_TOPIC", baseTopic+"/events")
	if strings.ContainsAny(baseTopic, "+#") || strings.TrimSpace(baseTopic) == "" || strings.ContainsAny(topic, "+#") || strings.TrimSpace(topic) == "" {
		log.Fatal("MQTT topics must be non-empty and contain neither + nor #")
	}
	clientID := env("CLIENT_ID", "icad2mqtt")
	interval, err := positiveSeconds(env("POLL_INTERVAL", "60"))
	if err != nil {
		log.Fatal(err)
	}
	if interval < time.Minute {
		log.Printf("POLL_INTERVAL below 60 seconds; clamping to 60 seconds")
		interval = time.Minute
	}
	timeout, err := boundedSeconds(env("HTTP_TIMEOUT", "15"))
	if err != nil {
		log.Fatal(err)
	}
	publishRaw, err := strictBool(env("PUBLISH_RAW", "true"))
	if err != nil {
		log.Fatal(err)
	}

	options := mqtt.NewClientOptions().AddBroker(broker).SetClientID(clientID)
	if username := strings.TrimSpace(os.Getenv("MQTT_USERNAME")); username != "" {
		options.SetUsername(username)
	}
	if password := os.Getenv("MQTT_PASSWORD"); password != "" {
		options.SetPassword(password)
	}
	options.SetAutoReconnect(true).SetConnectRetry(true).SetConnectRetryInterval(2 * time.Second)
	client := mqtt.NewClient(options)
	if token := client.Connect(); !token.WaitTimeout(15 * time.Second) {
		log.Fatal("MQTT connection timeout")
	} else if token.Error() != nil {
		log.Fatalf("MQTT connection failed: %v", token.Error())
	}
	defer client.Disconnect(250)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	var last string
	hasLast := false
	failures := 0
	for {
		data, fetchErr := fetch(ctx, timeout)
		if fetchErr != nil {
			failures++
			log.Printf("failed to fetch CAD events: %v", fetchErr)
		} else {
			failures = 0
		}
		if fetchErr == nil && publishRaw && (!hasLast || data != last) {
			publish := client.Publish(topic, 1, false, data)
			if !publish.WaitTimeout(5*time.Second) || publish.Error() != nil {
				log.Printf("failed to publish CAD events: %v", publish.Error())
			} else {
				last, hasLast = data, true
			}
		}
		delay := backoffDelay(interval, failures)
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}

func backoffDelay(base time.Duration, failures int) time.Duration {
	if base < time.Minute {
		base = time.Minute
	}
	capDelay := base
	for i := 0; i < failures; i++ {
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
	return base + time.Duration(rand.Int64N(int64(span)))
}

func env(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func positiveSeconds(raw string) (time.Duration, error) {
	seconds, err := strconv.Atoi(raw)
	if err != nil || seconds < 1 {
		return 0, fmt.Errorf("POLL_INTERVAL must be a positive integer, got %q", raw)
	}
	return time.Duration(seconds) * time.Second, nil
}

func boundedSeconds(raw string) (time.Duration, error) {
	seconds, err := strconv.Atoi(raw)
	if err != nil || seconds < 5 || seconds > 60 {
		return 0, fmt.Errorf("HTTP_TIMEOUT must be between 5 and 60 seconds, got %q", raw)
	}
	return time.Duration(seconds) * time.Second, nil
}
func strictBool(raw string) (bool, error) {
	if raw != "true" && raw != "false" {
		return false, fmt.Errorf("boolean must be true or false, got %q", raw)
	}
	return raw == "true", nil
}

func fetch(ctx context.Context, timeout time.Duration) (string, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, eventURL, nil)
	if err != nil {
		return "", err
	}
	request.Header.Set("User-Agent", "icad2mqtt/1.0")
	client := &http.Client{Timeout: timeout}
	response, err := client.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected HTTP status: %s", response.Status)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, (5<<20)+1))
	if err == nil && len(body) > 5<<20 {
		return "", fmt.Errorf("HTTP response exceeds 5242880 bytes")
	}
	return string(body), err
}
