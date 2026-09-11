package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

const eventURL = "https://911events.ongov.net/CADInet/app/events.jsp"

func main() {
	broker := env("MQTT_BROKER", "tcp://localhost:1883")
	topic := env("MQTT_TOPIC", "911/cad/events")
	clientID := env("CLIENT_ID", "icad2mqtt")
	interval, err := positiveSeconds(env("POLL_INTERVAL", "30"))
	if err != nil {
		log.Fatal(err)
	}

	options := mqtt.NewClientOptions().AddBroker(broker).SetClientID(clientID)
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
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		data, fetchErr := fetch(ctx)
		if fetchErr != nil {
			log.Printf("failed to fetch CAD events: %v", fetchErr)
		} else if !hasLast || data != last {
			publish := client.Publish(topic, 1, false, data)
			if !publish.WaitTimeout(5*time.Second) || publish.Error() != nil {
				log.Printf("failed to publish CAD events: %v", publish.Error())
			} else {
				last, hasLast = data, true
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
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

func fetch(ctx context.Context) (string, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, eventURL, nil)
	if err != nil {
		return "", err
	}
	request.Header.Set("User-Agent", "icad2mqtt/1.0")
	client := &http.Client{Timeout: 10 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected HTTP status: %s", response.Status)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, 5<<20))
	return string(body), err
}
