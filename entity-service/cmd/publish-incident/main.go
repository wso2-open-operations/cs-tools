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

// Command publish-incident puts one incident.created onto the event topic,
// exactly as this service would.
//
// # WHY THIS EXISTS
//
// POST /incidents returns 503 on DATA_SOURCE=postgres - creating an incident
// needs work_item.number, which has no sequence - so a local stack cannot
// produce this event the way production does. Everything downstream of the
// publish is nonetheless real and running, and had never been exercised:
// the topic, the consumer group, the decode, and the escalation engine.
//
// It matters that this lives in entity-service and marshals
// events.IncidentCreatedPayload from THIS module. The two services keep
// separate copies of that struct, synchronised by hand. A harness that builds
// the event from the consumer's own copy is the consumer talking to itself
// and cannot detect drift between them. This can.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/eventbus"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/events"
)

func main() {
	broker := flag.String("broker", "localhost:9094", "Kafka bootstrap address")
	topic := flag.String("topic", "case-events", "topic to publish to")
	id := flag.String("incident-id", "", "incident id; defaults to one derived from the clock")
	priority := flag.String("priority", "P0", "incident priority")
	team := flag.String("team", "Vega", "assigned team")
	title := flag.String("title", "Gateway returning 500s for every tenant", "incident title")
	account := flag.String("account", "Acme Corporation", "account")
	product := flag.String("product", "", "product; the consumer routes the Chat space by it")
	reportedAt := flag.String("reported-at", "", "RFC3339 report time; decides the shift. Defaults to now")
	flag.Parse()

	reported := time.Now().UTC()
	if *reportedAt != "" {
		parsed, perr := time.Parse(time.RFC3339, *reportedAt)
		if perr != nil {
			fmt.Fprintln(os.Stderr, "reported-at must be RFC3339:", perr)
			os.Exit(1)
		}
		reported = parsed
	}

	incidentID := *id
	if incidentID == "" {
		incidentID = fmt.Sprintf("local-inc-%d", time.Now().Unix())
	}

	payload, err := json.Marshal(events.IncidentCreatedPayload{
		Title:            *title,
		ShortDescription: *title,
		Number:           "INC" + lastN(incidentID, 7),
		Priority:         *priority,
		Account:          *account,
		Team:             *team,
		ReportedAt:       reported.Format(time.RFC3339),
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "encode payload:", err)
		os.Exit(1)
	}
	if *product != "" {
		// This service's own IncidentCreatedPayload has no product field, so
		// there is nothing to set it on. The consumer's copy does have one,
		// and routes the Chat space by it. Splicing it in here keeps that
		// difference visible rather than papering over it.
		var m map[string]any
		_ = json.Unmarshal(payload, &m)
		m["product"] = *product
		payload, _ = json.Marshal(m)
	}

	producer := eventbus.NewProducer(eventbus.Config{
		Broker:           *broker,
		Topic:            *topic,
		ConnectionString: os.Getenv("EVENT_HUB_CONNECTION_STRING"),
	})
	defer producer.Close()

	envelope, err := json.Marshal(events.Envelope{
		Type: events.TypeIncidentCreated, EntityID: incidentID, Payload: payload,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "encode envelope:", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := producer.Publish(ctx, []byte(incidentID), envelope); err != nil {
		fmt.Fprintln(os.Stderr, "publish:", err)
		os.Exit(1)
	}
	fmt.Printf("published %s for %s (priority %s, team %s) to %s\n",
		events.TypeIncidentCreated, incidentID, *priority, *team, *topic)
	fmt.Printf("payload: %s\n", payload)
}

// lastN is the last n characters of s, or all of them when it is shorter.
// Slicing directly panicked on any -incident-id under seven characters, which
// is a crash instead of an error message for a tool somebody is using by hand.
func lastN(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[len(s)-n:]
}
