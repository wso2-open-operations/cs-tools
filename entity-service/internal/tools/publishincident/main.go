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

//go:build ignore

// Excluded from every normal build on purpose. Choreo's Go buildpack builds
// entity-service only when `go list ./...` finds exactly one main package
// (cmd/api); a second one under cmd/ broke the build (#2393). Build this local
// test tool by its path instead:
//
//	go build -o /tmp/publish-incident ./internal/tools/publishincident/main.go

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
	contactType := flag.String("contact-type", "", "how the incident was raised: AZURE, SITE_247, SENTINEL (monitoring) or EMAIL, PHONE ...; routing reads it")
	event := flag.String("event", "created", "what to publish: created, or a stop gesture for an existing -incident-id: assigned, acknowledged (left NEW), comment (public)")
	record := flag.String("record", "incident", "incident, or case: a customer case, which is what CRE paging starts from")
	caseEvent := flag.String("case-event", "created", "with -record case: created, severity (-from/-priority), assigned (-assignee), comment (-author), closed")
	from := flag.String("from", "HIGH", "with -case-event severity: the old severity")
	assignee := flag.String("assignee", "local.engineer@wso2.com", "with -case-event assigned: who the case is assigned to")
	author := flag.String("author", "local.engineer@wso2.com", "with -case-event comment: who wrote it")
	workNote := flag.Bool("work-note", false, "with -case-event comment: a work note rather than a public comment")
	engineer := flag.Bool("engineer", true, "with -case-event comment: written by a support engineer (false = the customer)")
	flag.BoolVar(&dryRun, "dry-run", false, "with -record case: print the envelope instead of publishing it")
	flag.Parse()

	if *record == "case" {
		caseID := *id
		if caseID == "" {
			if *caseEvent != "created" {
				fmt.Fprintln(os.Stderr, "-case-event", *caseEvent, "needs the -incident-id of the case")
				os.Exit(1)
			}
			caseID = fmt.Sprintf("local-case-%d", time.Now().Unix())
		}
		publishCaseEvent(*broker, *topic, caseID, *caseEvent, caseFields{
			severity: severityLabel(*priority), from: severityLabel(*from), team: *team, title: *title,
			account: *account, product: *product, assignee: *assignee, author: *author,
			workNote: *workNote, engineer: *engineer, at: time.Now().UTC(),
		})
		return
	}

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
		if *event != "created" {
			fmt.Fprintln(os.Stderr, "-event", *event, "needs the -incident-id of the incident to acknowledge")
			os.Exit(1)
		}
		incidentID = fmt.Sprintf("local-inc-%d", time.Now().Unix())
	}

	// The stop gestures, each as entity-service itself publishes it.
	if *event != "created" {
		var (
			typ  events.Type
			body any
		)
		switch *event {
		case "assigned":
			typ, body = events.TypeIncidentAssigned, events.IncidentAssignedPayload{AssigneeID: "local-engineer", AssigneeName: "Local Engineer"}
		case "acknowledged":
			typ, body = events.TypeIncidentAcknowledged, events.IncidentAcknowledgedPayload{PreviousState: "NEW", NewState: "IN_PROGRESS"}
		case "comment":
			typ, body = events.TypeIncidentCommentAdded, events.IncidentCommentAddedPayload{CommentID: fmt.Sprintf("local-comment-%d", time.Now().Unix()), IsPublic: true}
		default:
			fmt.Fprintln(os.Stderr, "-event must be created, assigned, acknowledged or comment")
			os.Exit(1)
		}
		publishOne(*broker, *topic, incidentID, typ, body)
		return
	}

	payload, err := json.Marshal(events.IncidentCreatedPayload{
		Title:            *title,
		ShortDescription: *title,
		Number:           "INC" + lastN(incidentID, 7),
		Priority:         *priority,
		Account:          *account,
		Team:             *team,
		ContactType:      *contactType,
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

// dryRun prints an envelope instead of publishing it.
var dryRun bool

// publishOne puts one event about an existing incident on the topic.
func publishOne(broker, topic, incidentID string, typ events.Type, body any) {
	payload, err := json.Marshal(body)
	if err != nil {
		fmt.Fprintln(os.Stderr, "encode payload:", err)
		os.Exit(1)
	}
	envelope, err := json.Marshal(events.Envelope{Type: typ, EntityID: incidentID, Payload: payload})
	if err != nil {
		fmt.Fprintln(os.Stderr, "encode envelope:", err)
		os.Exit(1)
	}
	if dryRun {
		fmt.Println(string(envelope))
		return
	}
	producer := eventbus.NewProducer(eventbus.Config{
		Broker: broker, Topic: topic, ConnectionString: os.Getenv("EVENT_HUB_CONNECTION_STRING"),
	})
	defer producer.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := producer.Publish(ctx, []byte(incidentID), envelope); err != nil {
		fmt.Fprintln(os.Stderr, "publish:", err)
		os.Exit(1)
	}
	fmt.Printf("published %s for %s to %s\n", typ, incidentID, topic)
	fmt.Printf("payload: %s\n", payload)
}

// caseFields are what the case events carry, from the flags.
type caseFields struct {
	severity, from, team, title, account, product, assignee, author string
	workNote, engineer                                              bool
	at                                                              time.Time
}

// localRecipients satisfies the consumer's validation; nobody reads it in a
// local run.
var localRecipients = []string{"local.watcher@wso2.com"}

// publishCaseEvent puts one customer case event on the topic, with the fields
// csm-notification-service's events.Validate requires. Built as plain maps
// with the wire names, the way entity-service's own publisher spells them.
func publishCaseEvent(broker, topic, caseID, event string, f caseFields) {
	number := "CS" + lastN(caseID, 7)
	var (
		typ  events.Type
		body map[string]any
	)
	switch event {
	case "created":
		typ, body = events.TypeCaseCreated, map[string]any{
			"reporterName": "Local Customer", "projectName": f.account, "projectId": "local-project",
			"caseId": caseID, "caseNumber": number, "caseTitle": f.title, "caseType": "CASE",
			"priority": f.severity, "team": f.team, "createdAt": f.at.Format(time.RFC3339),
			"description": f.title, "recipients": localRecipients,
		}
		if f.product != "" {
			body["product"] = f.product
		}
	case "severity":
		typ, body = events.TypeSeverityChanged, map[string]any{
			"projectId": "local-project", "caseId": caseID, "caseNumber": number, "caseTitle": f.title,
			"oldSeverity": f.from, "newSeverity": f.severity, "team": f.team, "recipients": localRecipients,
		}
	case "assigned":
		typ, body = events.TypeCaseAssigned, map[string]any{
			"assigneeName": "Local Engineer", "assigneeEmail": f.assignee, "projectId": "local-project",
			"caseId": caseID, "caseNumber": number, "recipients": localRecipients,
		}
	case "comment":
		typ, body = events.TypeCommentAdded, map[string]any{
			"name": "Local Author", "projectId": "local-project", "caseId": caseID, "caseNumber": number,
			"caseTitle": f.title, "caseComment": "looking into it", "commentId": fmt.Sprintf("local-comment-%d", time.Now().UnixNano()),
			"isInternalNote": f.workNote, "authorEmail": f.author, "isSupportEngineerResponse": f.engineer,
			"recipients": localRecipients,
		}
	case "closed":
		typ, body = events.TypeStatusChanged, map[string]any{
			"projectId": "local-project", "caseId": caseID, "caseNumber": number, "newStatus": "Closed",
			"recipients": localRecipients,
		}
	default:
		fmt.Fprintln(os.Stderr, "-case-event must be created, severity, assigned, comment or closed")
		os.Exit(1)
	}
	publishOne(broker, topic, caseID, typ, body)
}

// severityLabel accepts S0-S4 or a label and returns the label a case event
// carries (CATASTROPHIC ... LOW).
func severityLabel(s string) string {
	switch s {
	case "S0", "P0":
		return "CATASTROPHIC"
	case "S1", "P1":
		return "CRITICAL"
	case "S2", "P2":
		return "HIGH"
	case "S3", "P3":
		return "MEDIUM"
	case "S4", "P4":
		return "LOW"
	}
	return s
}
