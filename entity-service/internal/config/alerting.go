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
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/alertqueue/sqs"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/validate"
)

// AlertingVersion is a stage of the move from the v1 alert services (sre-alert-ingestion, sre-alert-core) to the
// alert pipeline in this service.
type AlertingVersion string

// Alerting versions; v1 is the default, so nothing changes until a deployment opts in.
const (
	// AlertingVersionV1 runs the v1 services only: the consumer, relay and processor are off.
	AlertingVersionV1 AlertingVersion = "v1"
	// AlertingVersionHybrid runs the pipeline in shadow: alerts are stored and decided, and the decision is recorded
	// (SHADOWED) but no incident is raised.
	AlertingVersionHybrid AlertingVersion = "hybrid"
	// AlertingVersionV2 runs the pipeline live: it raises incidents.
	AlertingVersionV2 AlertingVersion = "v2"
)

// Bounds of the alerting settings.
const (
	maxAlertConsumerConcurrency  = 64
	maxAlertProcessorConcurrency = 16
	maxAlertRelayMaxInFlight     = 1000
)

// AlertProcessorConnectionsPerWorker is how many pool connections one alert processor consumer can hold at once: the
// fingerprint lock's transaction and the incident create it runs.
const AlertProcessorConnectionsPerWorker = 2

// AlertRelayConnections is the pool connections the alert relay holds at once: one short transaction to mark a batch.
const AlertRelayConnections = 1

// AlertingConfig is the SRE alert pipeline's configuration: the queue consumer, the relay that hands each stored
// alert over through Event Hubs, and the processor that turns it into an incident.
type AlertingConfig struct {
	// AlertingVersion picks which alert pipeline runs (ALERTING_VERSION: v1, hybrid or v2; default v1).
	AlertingVersion AlertingVersion

	// AlertServiceBusConnectionString is a shared access policy's connection string for the Azure gateway's Service
	// Bus namespace (ALERT_SERVICEBUS_CONNECTION_STRING); empty leaves Azure out.
	AlertServiceBusConnectionString string
	// AlertServiceBusPriorityQueues and AlertServiceBusStandardQueues are the queue names of each tier
	// (ALERT_SERVICEBUS_PRIORITY_QUEUES, ALERT_SERVICEBUS_STANDARD_QUEUES, comma-separated).
	AlertServiceBusPriorityQueues []string
	AlertServiceBusStandardQueues []string

	// AlertSQSPriorityQueueURLs and AlertSQSStandardQueueURLs are the SQS standard queue URLs of each tier
	// (ALERT_SQS_PRIORITY_QUEUE_URLS, ALERT_SQS_STANDARD_QUEUE_URLS, comma-separated); none leaves AWS out.
	AlertSQSPriorityQueueURLs []string
	AlertSQSStandardQueueURLs []string
	// AlertSQSDeadLetterQueueURL is where a message that can never be stored goes (ALERT_SQS_DEAD_LETTER_QUEUE_URL);
	// empty leaves it to the queue's redrive policy.
	AlertSQSDeadLetterQueueURL string
	// AlertSQSRegion is the queues' region (ALERT_SQS_REGION); empty reads it from the first priority queue URL.
	AlertSQSRegion string
	// AlertSQSAccessKeyID and AlertSQSSecretAccessKey are an IAM user's keys (ALERT_SQS_ACCESS_KEY_ID,
	// ALERT_SQS_SECRET_ACCESS_KEY), set together; empty uses the default AWS credential chain.
	AlertSQSAccessKeyID     string
	AlertSQSSecretAccessKey string

	// AlertConsumerConcurrency is how many queue messages are stored at once (ALERT_CONSUMER_CONCURRENCY, default 8).
	AlertConsumerConcurrency int

	// AlertEventHubTopic is the Event Hub the relay publishes alert.created to and the processor consumes
	// (ALERT_EVENT_HUB_TOPIC, default alert-events). It must be its own: csm-notification-service dead-letters event
	// types it does not know.
	AlertEventHubTopic string
	// AlertProcessorConsumerGroup is the processor's consumer group on it (ALERT_PROCESSOR_CONSUMER_GROUP, default
	// entity-service-alert-processor).
	AlertProcessorConsumerGroup string
	// AlertRelayMaxInFlight is the relay's backpressure (ALERT_RELAY_MAX_IN_FLIGHT, default 20): no more is published
	// while this many alerts are published and undecided, so a backlog waits in the table, priority first.
	AlertRelayMaxInFlight int
	// AlertRelayPollInterval is the relay's backstop poll (ALERT_RELAY_POLL_INTERVAL, default 5s).
	AlertRelayPollInterval time.Duration

	// AlertProcessorCallerID is the "user" UUID alert-born incidents are raised by (ALERT_PROCESSOR_CALLER_ID);
	// required for v2.
	AlertProcessorCallerID string
	// AlertProcessorConcurrency is how many alert.created consumers each replica runs (ALERT_PROCESSOR_CONCURRENCY,
	// default 4).
	AlertProcessorConcurrency int
	// AlertProcessorMaxIncidentAge is how old an open incident may be and still take a new alert of its fingerprint
	// (ALERT_PROCESSOR_MAX_INCIDENT_AGE, default 24h, 0 = no limit).
	AlertProcessorMaxIncidentAge time.Duration
}

func loadAlertingConfig() AlertingConfig {
	return AlertingConfig{
		AlertingVersion:                 AlertingVersion(strings.ToLower(strings.TrimSpace(getEnvOrDefault("ALERTING_VERSION", string(AlertingVersionV1))))),
		AlertServiceBusConnectionString: strings.TrimSpace(os.Getenv("ALERT_SERVICEBUS_CONNECTION_STRING")),
		AlertServiceBusPriorityQueues:   envList("ALERT_SERVICEBUS_PRIORITY_QUEUES"),
		AlertServiceBusStandardQueues:   envList("ALERT_SERVICEBUS_STANDARD_QUEUES"),
		AlertSQSPriorityQueueURLs:       envList("ALERT_SQS_PRIORITY_QUEUE_URLS"),
		AlertSQSStandardQueueURLs:       envList("ALERT_SQS_STANDARD_QUEUE_URLS"),
		AlertSQSDeadLetterQueueURL:      strings.TrimSpace(os.Getenv("ALERT_SQS_DEAD_LETTER_QUEUE_URL")),
		AlertSQSRegion:                  strings.TrimSpace(os.Getenv("ALERT_SQS_REGION")),
		AlertSQSAccessKeyID:             strings.TrimSpace(os.Getenv("ALERT_SQS_ACCESS_KEY_ID")),
		AlertSQSSecretAccessKey:         os.Getenv("ALERT_SQS_SECRET_ACCESS_KEY"),
		AlertConsumerConcurrency:        envIntInRange("ALERT_CONSUMER_CONCURRENCY", 8, 1, maxAlertConsumerConcurrency),
		AlertEventHubTopic:              strings.TrimSpace(getEnvOrDefault("ALERT_EVENT_HUB_TOPIC", "alert-events")),
		AlertProcessorConsumerGroup:     strings.TrimSpace(getEnvOrDefault("ALERT_PROCESSOR_CONSUMER_GROUP", "entity-service-alert-processor")),
		AlertRelayMaxInFlight:           envIntInRange("ALERT_RELAY_MAX_IN_FLIGHT", 20, 1, maxAlertRelayMaxInFlight),
		AlertRelayPollInterval:          envDuration("ALERT_RELAY_POLL_INTERVAL", 5*time.Second),
		AlertProcessorCallerID:          strings.TrimSpace(os.Getenv("ALERT_PROCESSOR_CALLER_ID")),
		AlertProcessorConcurrency:       envIntInRange("ALERT_PROCESSOR_CONCURRENCY", 4, 1, maxAlertProcessorConcurrency),
		AlertProcessorMaxIncidentAge:    envDurationOrOff("ALERT_PROCESSOR_MAX_INCIDENT_AGE", 24*time.Hour),
	}
}

// envList reads a comma-separated list, trimming each entry and dropping empty ones.
func envList(key string) []string {
	var out []string
	for _, v := range strings.Split(os.Getenv(key), ",") {
		if v = strings.TrimSpace(v); v != "" {
			out = append(out, v)
		}
	}
	return out
}

// AlertConsumerEnabled reports whether the alert pipeline (consumer, relay, processor) runs: in hybrid and v2.
func (c *AlertingConfig) AlertConsumerEnabled() bool {
	return c.AlertingVersion == AlertingVersionHybrid || c.AlertingVersion == AlertingVersionV2
}

// AlertProcessorLive reports whether the processor raises incidents (v2); in hybrid it only records what it would do.
func (c *AlertingConfig) AlertProcessorLive() bool {
	return c.AlertingVersion == AlertingVersionV2
}

// HasAlertServiceBus reports whether the consumer receives from Azure Service Bus.
func (c *AlertingConfig) HasAlertServiceBus() bool {
	return c.AlertServiceBusConnectionString != "" || len(c.AlertServiceBusPriorityQueues)+len(c.AlertServiceBusStandardQueues) > 0
}

// HasAlertSQS reports whether the consumer receives from AWS SQS.
func (c *AlertingConfig) HasAlertSQS() bool {
	return len(c.AlertSQSPriorityQueueURLs)+len(c.AlertSQSStandardQueueURLs) > 0
}

// AlertSQSResolvedRegion is ALERT_SQS_REGION, else the region of the first priority queue URL.
func (c *AlertingConfig) AlertSQSResolvedRegion() string {
	if c.AlertSQSRegion != "" {
		return c.AlertSQSRegion
	}
	for _, u := range append(append([]string{}, c.AlertSQSPriorityQueueURLs...), c.AlertSQSStandardQueueURLs...) {
		if r := sqs.RegionOf(u); r != "" {
			return r
		}
	}
	return ""
}

// validateAlerting refuses a hybrid or v2 deployment that could not run the whole pipeline. v1 checks nothing.
func (c *Config) validateAlerting() error {
	switch c.AlertingVersion {
	case "", AlertingVersionV1:
		return nil
	case AlertingVersionHybrid, AlertingVersionV2:
	default:
		return fmt.Errorf("ALERTING_VERSION must be v1, hybrid or v2, got %q", c.AlertingVersion)
	}
	if !c.HasDatabase() {
		return fmt.Errorf("ALERTING_VERSION %s needs DB_USER, DB_PASSWORD and DB_NAME: alerts are stored in the database", c.AlertingVersion)
	}
	if c.IncidentDefaultServiceID == "" {
		return fmt.Errorf("ALERTING_VERSION %s needs INCIDENT_DEFAULT_SERVICE_ID, the service an incident gets when its alert names none", c.AlertingVersion)
	}
	if !c.HasAlertServiceBus() && !c.HasAlertSQS() {
		return fmt.Errorf("ALERTING_VERSION %s needs Azure Service Bus (ALERT_SERVICEBUS_*), AWS SQS (ALERT_SQS_*), or both", c.AlertingVersion)
	}
	if c.HasAlertServiceBus() {
		if err := c.validateAlertServiceBus(); err != nil {
			return err
		}
	}
	if c.HasAlertSQS() {
		if err := c.validateAlertSQS(); err != nil {
			return err
		}
	}
	if err := c.validateAlertTopic(); err != nil {
		return err
	}
	// The consumer, the relay and the processor share the pool with API traffic, so together they must leave room.
	used := c.AlertConsumerConcurrency + AlertRelayConnections + AlertProcessorConnectionsPerWorker*c.AlertProcessorConcurrency
	if int64(used) >= int64(c.DBPoolMaxConns) {
		return fmt.Errorf("ALERT_CONSUMER_CONCURRENCY (%d) plus the relay's %d connection plus %d per ALERT_PROCESSOR_CONCURRENCY (%d) must stay below DB_POOL_MAX_CONNS (%d)",
			c.AlertConsumerConcurrency, AlertRelayConnections, AlertProcessorConnectionsPerWorker, c.AlertProcessorConcurrency, c.DBPoolMaxConns)
	}
	if c.AlertingVersion == AlertingVersionV2 && c.AlertProcessorCallerID == "" {
		return fmt.Errorf("ALERTING_VERSION v2 needs ALERT_PROCESSOR_CALLER_ID, the user its incidents are raised by")
	}
	if v := c.AlertProcessorCallerID; v != "" && !validate.IsUUID(v) {
		return fmt.Errorf("ALERT_PROCESSOR_CALLER_ID %q is not a valid UUID", v)
	}
	return nil
}

// validateAlertServiceBus needs the connection string and at least one queue per tier, no queue twice (Service Bus
// names ignore case). The connection string is never quoted: it holds the key.
func (c *Config) validateAlertServiceBus() error {
	cs := c.AlertServiceBusConnectionString
	if cs == "" {
		return fmt.Errorf("ALERT_SERVICEBUS_*_QUEUES are set without ALERT_SERVICEBUS_CONNECTION_STRING")
	}
	if !strings.Contains(cs, "Endpoint=sb://") || !strings.Contains(cs, "SharedAccessKeyName=") || !strings.Contains(cs, "SharedAccessKey=") {
		return fmt.Errorf("ALERT_SERVICEBUS_CONNECTION_STRING must be a shared access policy's connection string (Endpoint=sb://...;SharedAccessKeyName=...;SharedAccessKey=...)")
	}
	if len(c.AlertServiceBusPriorityQueues) == 0 || len(c.AlertServiceBusStandardQueues) == 0 {
		return fmt.Errorf("ALERT_SERVICEBUS_PRIORITY_QUEUES and ALERT_SERVICEBUS_STANDARD_QUEUES each need at least one queue")
	}
	seen := map[string]bool{}
	for _, q := range append(append([]string{}, c.AlertServiceBusPriorityQueues...), c.AlertServiceBusStandardQueues...) {
		key := strings.ToLower(q)
		if seen[key] {
			return fmt.Errorf("Service Bus queue %q is listed more than once across ALERT_SERVICEBUS_PRIORITY_QUEUES and ALERT_SERVICEBUS_STANDARD_QUEUES", q)
		}
		seen[key] = true
	}
	return nil
}

// validateAlertSQS needs at least one queue per tier, no queue twice, every queue in one known region, and the
// access keys set together.
func (c *Config) validateAlertSQS() error {
	if len(c.AlertSQSPriorityQueueURLs) == 0 || len(c.AlertSQSStandardQueueURLs) == 0 {
		return fmt.Errorf("ALERT_SQS_PRIORITY_QUEUE_URLS and ALERT_SQS_STANDARD_QUEUE_URLS each need at least one queue")
	}
	region := c.AlertSQSResolvedRegion()
	if region == "" {
		return fmt.Errorf("the SQS queue URLs name no region; set ALERT_SQS_REGION")
	}
	seen := map[string]bool{}
	for _, u := range append(append([]string{}, c.AlertSQSPriorityQueueURLs...), c.AlertSQSStandardQueueURLs...) {
		if seen[u] {
			return fmt.Errorf("SQS queue %q is listed more than once across ALERT_SQS_PRIORITY_QUEUE_URLS and ALERT_SQS_STANDARD_QUEUE_URLS", u)
		}
		seen[u] = true
		if r := sqs.RegionOf(u); r != "" && r != region {
			return fmt.Errorf("SQS queue %q is in %s, not %s: every alert queue must be in one region", u, r, region)
		}
	}
	if (c.AlertSQSAccessKeyID == "") != (c.AlertSQSSecretAccessKey == "") {
		return fmt.Errorf("ALERT_SQS_ACCESS_KEY_ID and ALERT_SQS_SECRET_ACCESS_KEY must be set together")
	}
	return nil
}

// validateAlertTopic checks the Event Hub the relay publishes alert.created to and the processor reads it from: the
// connection, and a topic of its own, apart from every topic another reader consumes.
func (c *Config) validateAlertTopic() error {
	if c.EventHubBroker == "" || c.EventHubConnectionString == "" {
		return fmt.Errorf("ALERTING_VERSION %s needs EVENT_HUB_BROKER and EVENT_HUB_CONNECTION_STRING: stored alerts reach the alert processor as alert.created events", c.AlertingVersion)
	}
	topic := c.AlertEventHubTopic
	if topic == "" {
		return fmt.Errorf("ALERT_EVENT_HUB_TOPIC must not be empty")
	}
	if c.AlertProcessorConsumerGroup == "" {
		return fmt.Errorf("ALERT_PROCESSOR_CONSUMER_GROUP must not be empty")
	}
	// alert.created on a topic csm-notification-service or another reader consumes would be dead-lettered there.
	others := map[string]string{
		"EVENT_HUB_TOPIC":                    c.EventHubTopic,
		"INCIDENT_EVENT_HUB_TOPIC":           c.IncidentEventHubTopic,
		"SRE_EVENT_HUB_TOPIC":                c.SREEventHubTopic,
		"CR_EVENT_HUB_TOPIC":                 c.CREventHubTopic,
		"OUTAGE_EVENT_HUB_TOPIC":             c.OutageEventHubTopic,
		"PROJECT_EVENT_HUB_TOPIC":            c.ProjectEventHubTopic,
		"SALESFORCE_WRITE_EVENT_HUB_TOPIC":   c.SalesforceWriteEventHubTopic,
		"SALESFORCE_WRITE_RESULTS_TOPIC":     c.SalesforceWriteResultsTopic,
		"SALESFORCE_WRITE_RESULTS_DLQ_TOPIC": c.SalesforceWriteResultsDLQTopic,
	}
	for name, other := range others {
		if other != "" && strings.EqualFold(strings.TrimSpace(other), topic) {
			return fmt.Errorf("ALERT_EVENT_HUB_TOPIC must be its own: %s is %q too", name, other)
		}
	}
	return nil
}
