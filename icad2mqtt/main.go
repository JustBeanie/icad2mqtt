package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

const (
	// 911events endpoint
	cadEventURL = "https://911events.ongov.net/CADInet/app/events.jsp"
	
	// Default configuration
	defaultMqttBroker   = "tcp://localhost:1883"
	defaultMqttTopic    = "911/cad/events"
	defaultClientID     = "icad2mqtt"
	defaultPollInterval = 30
)

var (
	mqttBroker   string
	mqttTopic    string
	clientID     string
	pollInterval int
)

var (
	mqttClient mqtt.Client
	lastUpdate string
)

func init() {
	// Load configuration from environment variables
	mqttBroker = getEnv("MQTT_BROKER", defaultMqttBroker)
	mqttTopic = getEnv("MQTT_TOPIC", defaultMqttTopic)
	clientID = getEnv("CLIENT_ID", defaultClientID)
	
	pollStr := getEnv("POLL_INTERVAL", strconv.Itoa(defaultPollInterval))
	poll, err := strconv.Atoi(pollStr)
	if err != nil || poll < 1 {
		poll = default
	ticker := time.NewTicker(time.Duration(pollInterval)
	pollInterval = poll
}

func getEnv(key, defaultVal string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultVal
}

func main() {
	log.Println("Starting ICAD to MQTT bridge...")
	log.Printf("MQTT Broker: %s", redactBroker(mqttBroker))
	log.Printf("MQTT Topic: %s", mqttTopic)
	log.Printf("Poll Interval: %d seconds", pollInterval)
	
	// Connect to MQTT broker
	if err := connectMQTT(); err != nil {
		log.Fatalf("Failed to connect to MQTT broker: %v", err)
	}
	defer mqttClient.Disconnect(250)
	
	// Polling loop - fetch events every 30 seconds
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	
	// Initial fetch
	fetchAndPublish()
	
	for range ticker.C {
		fetchAndPublish()
	}
}

func connectMQTT() error {
	opts := mqtt.NewClientOptions()
	opts.AddBroker(mqttBroker)
	opts.SetClientID(clientID)
	opts.SetAutoReconnect(true)
	opts.SetConnectRetry(true)
	opts.SetConnectRetryInterval(2 * time.Second)
	opts.OnConnect = func(c mqtt.Client) {
		log.Println("Connected to MQTT broker")
	}
	opts.OnConnectionLost = func(c mqtt.Client, err error) {
		log.Printf("Connection lost: %v", err)
	}
	
	mqttClient = mqtt.NewClient(opts)
	if token := mqttClient.Connect(); token.Wait() && token.Error() != nil {
		return token.Error()
	}
	
	return nil
}

func fetchAndPublish() {
	data, err := fetchEvents()
	if err != nil {
		log.Printf("Error fetching events: %v", err)
		return
	}
	
	// Only publish if data has changed
	if data != lastUpdate {
		lastUpdate = data
		publishToMQTT(data)
		log.Println("Published update to MQTT")
	}
}

func fetchEvents() (string, error) {
	client := &http.Client{
		Timeout: 10 * time.Second,
	}
	
	resp, err := client.Get(cadEventURL)
	if err != nil {
		return "", fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}
	
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}
	
	return string(body), nil
}

func publishToMQTT(data string) {
	token := mqttClient.Publish(mqttTopic, 1, false, data)
	if !token.WaitTimeout(5 * time.Second) {
		log.Println("Warning: MQTT publish timeout")
	}
	if token.Error() != nil {
		log.Printf("Error publishing to MQTT: %v", token.Error())
	}
}

func redactBroker(broker string) string {
	// Redact password from broker URL for logging
	if strings.Contains(broker, "@") {
		parts := strings.Split(broker, "@")
		return parts[0] + "@***:***"
	}
	return broker
}
