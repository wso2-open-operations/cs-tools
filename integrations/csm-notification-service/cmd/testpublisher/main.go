// Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0

// Command testpublisher publishes a single fake case.status_changed event
// (NewStatus="Closed") to the real event bus, for manually testing
// kbdraftengine end to end. TEST-ONLY -- not part of the real service.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"log"
	"os"
	"time"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/eventbus"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/events"
)

func main() {
	caseID := flag.String("case-id", "", "real case UUID to publish a status_changed(Closed) event for")
	projectID := flag.String("project-id", "test-project", "projectId to put on the payload (not validated against anything real)")
	broker := flag.String("broker", os.Getenv("EVENT_HUB_BROKER"), "Kafka/Event Hub broker address")
	topic := flag.String("topic", os.Getenv("EVENT_HUB_TOPIC"), "topic to publish to (must match what your consumer reads -- case-events)")
	connStr := flag.String("conn-string", os.Getenv("EVENT_HUB_CONNECTION_STRING"), "Event Hub namespace connection string (same one your service already uses)")
	flag.Parse()

	if *caseID == "" {
		log.Fatal("testpublisher: -case-id is required")
	}
	if *broker == "" || *topic == "" || *connStr == "" {
		log.Fatal("testpublisher: broker, topic, and conn-string must all be set (via flags or EVENT_BUS_BROKER/EVENT_BUS_TOPIC/EVENT_BUS_CONNECTION_STRING env vars)")
	}

	payload := events.StatusChangedPayload{
		ProjectID:  *projectID,
		CaseID:     *caseID,
		NewStatus:  "Closed",
		Recipients: []string{"test@example.com"}, // required by Validate, unused by kbdraftengine
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		log.Fatalf("testpublisher: encode payload: %v", err)
	}

	if err := events.Validate(*caseID, events.TypeStatusChanged, payloadBytes); err != nil {
		log.Fatalf("testpublisher: payload fails validation (would be dead-lettered): %v", err)
	}

	env := events.Envelope{
		Type:     events.TypeStatusChanged,
		EntityID: *caseID,
		Payload:  payloadBytes,
	}
	envBytes, err := json.Marshal(env)
	if err != nil {
		log.Fatalf("testpublisher: encode envelope: %v", err)
	}

	producer := eventbus.NewProducer(eventbus.Config{
		Broker:           *broker,
		ConnectionString: *connStr,
		Topic:            *topic,
	})
	defer producer.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := producer.Publish(ctx, []byte(*caseID), envBytes); err != nil {
		log.Fatalf("testpublisher: publish failed: %v", err)
	}
	log.Printf("testpublisher: published case.status_changed(Closed) for case %s to topic %s", *caseID, *topic)
}
