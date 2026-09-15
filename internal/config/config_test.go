package config

import (
	"bytes"
	"fmt"
	"log"
	"strings"
	"testing"
	"time"
)

var configEnvKeys = []string{"MQTT_BROKER", "MQTT_TOPIC", "MQTT_BASE_TOPIC", "CLIENT_ID", "POLL_INTERVAL", "HTTP_USER_AGENT", "PUBLISH_RAW", "HA_DISCOVERY", "MQTT_USERNAME", "MQTT_PASSWORD", "HTTP_TIMEOUT"}

func clearConfigEnv(t *testing.T) {
	t.Helper()
	for _, key := range configEnvKeys {
		t.Setenv(key, "")
	}
}

func TestConfigClampWarningA(t *testing.T) {
	clearConfigEnv(t)
	var output bytes.Buffer
	old := log.Writer()
	log.SetOutput(&output)
	t.Cleanup(func() { log.SetOutput(old) })
	t.Setenv("POLL_INTERVAL", "30")
	if got, err := Load(); err != nil || got.PollInterval != time.Minute {
		t.Fatalf("clamp = %+v, err=%v", got, err)
	}
	if strings.Count(output.String(), "clamping to 60 seconds") != 1 {
		t.Fatalf("warning output = %q", output.String())
	}
	output.Reset()
	t.Setenv("POLL_INTERVAL", "60")
	if _, err := Load(); err != nil {
		t.Fatal(err)
	}
	if output.Len() != 0 {
		t.Fatalf("unexpected warning at floor: %q", output.String())
	}
}

func TestConfigTableB(t *testing.T) {
	tests := []struct {
		name, key, value string
		want             string
		bad              bool
	}{
		{"broker", "MQTT_BROKER", "mqtt://broker:1883", "mqtt://broker:1883", false},
		{"topic-derived", "MQTT_BASE_TOPIC", "custom/cad", "custom/cad/events", false},
		{"topic-empty-derived", "MQTT_TOPIC", "", "911/cad/events", false},
		{"client", "CLIENT_ID", "client-2", "client-2", false},
		{"poll", "POLL_INTERVAL", "120", "2m0s", false},
		{"agent", "HTTP_USER_AGENT", "test-agent", "test-agent", false},
		{"raw-false", "PUBLISH_RAW", "false", "false", false},
		{"raw-invalid", "PUBLISH_RAW", "TRUE", "", true},
		{"raw-one", "PUBLISH_RAW", "1", "", true},
		{"discovery-true", "HA_DISCOVERY", "true", "true", false},
		{"discovery-invalid", "HA_DISCOVERY", "yes", "", true},
		{"username", "MQTT_USERNAME", "alice", "alice", false},
		{"password", "MQTT_PASSWORD", "secret", "secret", false},
		{"timeout-five", "HTTP_TIMEOUT", "5", "5s", false},
		{"timeout-sixty", "HTTP_TIMEOUT", "60", "1m0s", false},
		{"timeout-four", "HTTP_TIMEOUT", "4", "", true},
		{"timeout-sixty-one", "HTTP_TIMEOUT", "61", "", true},
		{"topic-plus", "MQTT_TOPIC", "a/+/b", "", true},
		{"topic-hash", "MQTT_TOPIC", "#", "", true},
		{"topic-spaces", "MQTT_TOPIC", "   ", "", true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			clearConfigEnv(t)
			t.Setenv(test.key, test.value)
			got, err := Load()
			if test.bad {
				if err == nil {
					t.Fatalf("Load(%s=%q) unexpectedly succeeded", test.key, test.value)
				}
				return
			}
			if err != nil {
				t.Fatalf("Load returned error: %v", err)
			}
			var actual string
			switch test.key {
			case "MQTT_BROKER":
				actual = got.MqttBroker
			case "MQTT_BASE_TOPIC":
				actual = got.MqttTopic
			case "MQTT_TOPIC":
				actual = got.MqttTopic
			case "CLIENT_ID":
				actual = got.ClientID
			case "POLL_INTERVAL":
				actual = got.PollInterval.String()
			case "HTTP_USER_AGENT":
				actual = got.UserAgent
			case "PUBLISH_RAW":
				actual = fmt.Sprint(got.PublishRaw)
			case "HA_DISCOVERY":
				actual = fmt.Sprint(got.HADiscovery)
			case "MQTT_USERNAME":
				actual = got.MqttUsername
			case "MQTT_PASSWORD":
				actual = got.MqttPassword
			case "HTTP_TIMEOUT":
				actual = got.RequestTimeout.String()
			}
			if actual != test.want {
				t.Errorf("value = %q, want %q", actual, test.want)
			}
		})
	}
}

func TestConfigDefaultsB(t *testing.T) {
	clearConfigEnv(t)
	got, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.MqttBroker != DefaultBroker || got.MqttTopic != DefaultRawTopic || got.MqttBaseTopic != DefaultBaseTopic || !got.PublishRaw || got.HADiscovery || got.ClientID != DefaultClientID || got.PollInterval != DefaultPollInterval || got.RequestTimeout != DefaultHTTPTimeout || got.UserAgent != "icad2mqtt/1.0" {
		t.Fatalf("defaults = %+v", got)
	}
}

func TestConfigRedactionC(t *testing.T) {
	c := Config{MqttBroker: "mqtt://alice:broker-secret@example.com:1883", MqttPassword: "payload-secret", MqttTopic: "events"}
	output := fmt.Sprintf("%v %+v %#v", c, c, c)
	if strings.Contains(output, "broker-secret") || strings.Contains(output, "payload-secret") || strings.Contains(output, "alice") {
		t.Fatalf("config leaked secret or broker username: %s", output)
	}
}
