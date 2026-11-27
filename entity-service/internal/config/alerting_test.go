// Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package config

import (
	"slices"
	"strings"
	"testing"
	"time"
)

const testAlertServiceBusConnectionString = "Endpoint=sb://sbns-example.servicebus.windows.net/;SharedAccessKeyName=listen;SharedAccessKey=a2V5"

// alertingConfig is a complete hybrid configuration with both clouds.
func alertingConfig() Config {
	c := baseValidConfig()
	c.DBPoolMaxConns = 20
	c.IncidentDefaultServiceID = "00000000-0000-4000-8000-000000000001"
	c.EventHubBroker = "ns-example.servicebus.windows.net:9093"
	c.EventHubConnectionString = "Endpoint=sb://ns-example.servicebus.windows.net/;SharedAccessKeyName=k;SharedAccessKey=dGVzdA=="
	c.AlertingConfig = AlertingConfig{
		AlertingVersion:                 AlertingVersionHybrid,
		AlertServiceBusConnectionString: testAlertServiceBusConnectionString,
		AlertServiceBusPriorityQueues:   []string{"priority-queue"},
		AlertServiceBusStandardQueues:   []string{"standard-queue"},
		AlertSQSPriorityQueueURLs:       []string{"https://sqs.us-east-1.amazonaws.com/000000000000/alert-priority"},
		AlertSQSStandardQueueURLs:       []string{"https://sqs.us-east-1.amazonaws.com/000000000000/alert-standard"},
		AlertConsumerConcurrency:        8,
		AlertEventHubTopic:              "alert-events",
		AlertProcessorConsumerGroup:     "entity-service-alert-processor",
		AlertRelayMaxInFlight:           20,
		AlertRelayPollInterval:          5 * time.Second,
		AlertProcessorCallerID:          "00000000-0000-4000-8000-000000000002",
		AlertProcessorConcurrency:       4,
		AlertProcessorMaxIncidentAge:    24 * time.Hour,
	}
	return c
}

func TestConfig_ValidateAlerting(t *testing.T) {
	tests := []struct {
		name    string
		change  func(c *Config)
		wantErr string
	}{
		{name: "complete", change: func(*Config) {}},
		{name: "v1 ignores everything else", change: func(c *Config) { c.AlertingConfig = AlertingConfig{AlertingVersion: AlertingVersionV1} }},
		{name: "unknown version", change: func(c *Config) { c.AlertingVersion = "v3" }, wantErr: "ALERTING_VERSION must be"},
		{name: "no database", change: func(c *Config) { c.DBUser, c.DBPassword, c.DBName = "", "", "" }, wantErr: "DB_USER"},
		{name: "no default service", change: func(c *Config) { c.IncidentDefaultServiceID = "" }, wantErr: "INCIDENT_DEFAULT_SERVICE_ID"},
		{name: "no cloud", change: func(c *Config) {
			c.AlertServiceBusConnectionString, c.AlertServiceBusPriorityQueues, c.AlertServiceBusStandardQueues = "", nil, nil
			c.AlertSQSPriorityQueueURLs, c.AlertSQSStandardQueueURLs = nil, nil
		}, wantErr: "needs Azure Service Bus"},
		{name: "Service Bus only", change: func(c *Config) { c.AlertSQSPriorityQueueURLs, c.AlertSQSStandardQueueURLs = nil, nil }},
		{name: "SQS only", change: func(c *Config) {
			c.AlertServiceBusConnectionString, c.AlertServiceBusPriorityQueues, c.AlertServiceBusStandardQueues = "", nil, nil
		}},
		{name: "Service Bus queues without a connection string", change: func(c *Config) { c.AlertServiceBusConnectionString = "" }, wantErr: "without ALERT_SERVICEBUS_CONNECTION_STRING"},
		{name: "not a connection string", change: func(c *Config) { c.AlertServiceBusConnectionString = "sbns-example" }, wantErr: "shared access policy"},
		{name: "a Service Bus tier with no queue", change: func(c *Config) { c.AlertServiceBusStandardQueues = nil }, wantErr: "each need at least one queue"},
		{name: "a Service Bus queue twice, in another case", change: func(c *Config) {
			c.AlertServiceBusStandardQueues = []string{"Priority-Queue"}
		}, wantErr: "more than once"},
		{name: "an SQS tier with no queue", change: func(c *Config) { c.AlertSQSStandardQueueURLs = nil }, wantErr: "each need at least one queue"},
		{name: "an SQS queue twice", change: func(c *Config) { c.AlertSQSStandardQueueURLs = c.AlertSQSPriorityQueueURLs }, wantErr: "more than once"},
		{name: "SQS queues in two regions", change: func(c *Config) {
			c.AlertSQSStandardQueueURLs = []string{"https://sqs.eu-west-1.amazonaws.com/000000000000/alert-standard"}
		}, wantErr: "one region"},
		{name: "SQS URLs with no region", change: func(c *Config) {
			c.AlertSQSPriorityQueueURLs = []string{"http://localstack:4566/000000000000/p"}
			c.AlertSQSStandardQueueURLs = []string{"http://localstack:4566/000000000000/s"}
		}, wantErr: "ALERT_SQS_REGION"},
		{name: "SQS URLs with no region and ALERT_SQS_REGION", change: func(c *Config) {
			c.AlertSQSPriorityQueueURLs = []string{"http://localstack:4566/000000000000/p"}
			c.AlertSQSStandardQueueURLs = []string{"http://localstack:4566/000000000000/s"}
			c.AlertSQSRegion = "us-east-1"
		}},
		{name: "one SQS key without the other", change: func(c *Config) { c.AlertSQSAccessKeyID = "AKIAEXAMPLE" }, wantErr: "set together"},
		{name: "no Event Hub broker", change: func(c *Config) { c.EventHubBroker = "" }, wantErr: "EVENT_HUB_BROKER"},
		{name: "no Event Hub connection string", change: func(c *Config) { c.EventHubConnectionString = "" }, wantErr: "EVENT_HUB_CONNECTION_STRING"},
		{name: "no alert topic", change: func(c *Config) { c.AlertEventHubTopic = "" }, wantErr: "ALERT_EVENT_HUB_TOPIC"},
		{name: "no consumer group", change: func(c *Config) { c.AlertProcessorConsumerGroup = "" }, wantErr: "ALERT_PROCESSOR_CONSUMER_GROUP"},
		{name: "alert topic shared with sre-events", change: func(c *Config) { c.SREEventHubTopic = "Alert-Events" }, wantErr: "must be its own"},
		{name: "alert topic shared with the case topic", change: func(c *Config) { c.EventHubTopic = "alert-events" }, wantErr: "must be its own"},
		{name: "consumer, relay and processor take the whole pool", change: func(c *Config) { c.AlertConsumerConcurrency = 11 }, wantErr: "DB_POOL_MAX_CONNS"},
		{name: "consumer, relay and processor leave room in the pool", change: func(c *Config) { c.AlertConsumerConcurrency = 10 }},
		{name: "v2 without a caller", change: func(c *Config) { c.AlertingVersion = AlertingVersionV2; c.AlertProcessorCallerID = "" }, wantErr: "ALERT_PROCESSOR_CALLER_ID"},
		{name: "hybrid without a caller", change: func(c *Config) { c.AlertProcessorCallerID = "" }},
		{name: "caller that is not a UUID", change: func(c *Config) { c.AlertProcessorCallerID = "alert-core" }, wantErr: "not a valid UUID"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := alertingConfig()
			tt.change(&c)
			err := c.validateAlerting()
			if tt.wantErr == "" && err != nil {
				t.Fatalf("validateAlerting() = %v, want nil", err)
			}
			if tt.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tt.wantErr)) {
				t.Fatalf("validateAlerting() = %v, want an error mentioning %q", err, tt.wantErr)
			}
			if err != nil && strings.Contains(err.Error(), "a2V5") {
				t.Fatalf("the error quotes the Service Bus key: %v", err)
			}
		})
	}
}

func TestLoad_AlertingDefaults(t *testing.T) {
	for _, key := range []string{"ALERTING_VERSION", "ALERT_CONSUMER_CONCURRENCY", "ALERT_EVENT_HUB_TOPIC", "ALERT_PROCESSOR_CONSUMER_GROUP",
		"ALERT_RELAY_MAX_IN_FLIGHT", "ALERT_RELAY_POLL_INTERVAL", "ALERT_PROCESSOR_CONCURRENCY", "ALERT_PROCESSOR_MAX_INCIDENT_AGE"} {
		t.Setenv(key, "")
	}
	c := Load()
	if c.AlertingVersion != AlertingVersionV1 || c.AlertConsumerEnabled() || c.AlertProcessorLive() {
		t.Errorf("version %q (enabled %v, live %v), want v1 with the pipeline off", c.AlertingVersion, c.AlertConsumerEnabled(), c.AlertProcessorLive())
	}
	if c.AlertConsumerConcurrency != 8 || c.AlertProcessorConcurrency != 4 || c.AlertRelayMaxInFlight != 20 ||
		c.AlertRelayPollInterval != 5*time.Second || c.AlertProcessorMaxIncidentAge != 24*time.Hour {
		t.Errorf("defaults = %d, %d, %d, %s, %s; want 8, 4, 20, 5s, 24h", c.AlertConsumerConcurrency, c.AlertProcessorConcurrency,
			c.AlertRelayMaxInFlight, c.AlertRelayPollInterval, c.AlertProcessorMaxIncidentAge)
	}
	if c.AlertEventHubTopic != "alert-events" || c.AlertProcessorConsumerGroup != "entity-service-alert-processor" {
		t.Errorf("topic %q, group %q; want alert-events and entity-service-alert-processor", c.AlertEventHubTopic, c.AlertProcessorConsumerGroup)
	}
}

func TestLoad_AlertingSettings(t *testing.T) {
	t.Setenv("ALERTING_VERSION", " V2 ")
	t.Setenv("ALERT_SERVICEBUS_PRIORITY_QUEUES", " p1 , ,p2")
	t.Setenv("ALERT_SQS_STANDARD_QUEUE_URLS", "https://sqs.ap-south-1.amazonaws.com/000000000000/s")
	t.Setenv("ALERT_RELAY_MAX_IN_FLIGHT", "5000")
	t.Setenv("ALERT_PROCESSOR_MAX_INCIDENT_AGE", "0")
	c := Load()
	if c.AlertingVersion != AlertingVersionV2 || !c.AlertConsumerEnabled() || !c.AlertProcessorLive() {
		t.Errorf("version %q, want v2 enabled and live", c.AlertingVersion)
	}
	if !slices.Equal(c.AlertServiceBusPriorityQueues, []string{"p1", "p2"}) {
		t.Errorf("priority queues = %q, want the trimmed, non-empty names", c.AlertServiceBusPriorityQueues)
	}
	if c.AlertSQSResolvedRegion() != "ap-south-1" {
		t.Errorf("SQS region = %q, want ap-south-1 from the URL", c.AlertSQSResolvedRegion())
	}
	if c.AlertRelayMaxInFlight != 20 || c.AlertProcessorMaxIncidentAge != 0 {
		t.Errorf("max in flight %d (want the default 20 for a value above the cap), max incident age %s (want 0, no limit)",
			c.AlertRelayMaxInFlight, c.AlertProcessorMaxIncidentAge)
	}
}
