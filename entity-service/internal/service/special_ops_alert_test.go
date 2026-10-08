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

package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/events"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// alertPublisher records what was published; failWith makes Publish fail.
type alertPublisher struct {
	sent     []published
	failWith error
}

func (p *alertPublisher) Publish(_ context.Context, t events.Type, id string, raw json.RawMessage) error {
	if p.failWith != nil {
		return p.failWith
	}
	p.sent = append(p.sent, published{t, id, raw})
	return nil
}
func (p *alertPublisher) Close() {}

const (
	soChoreoRuntimeGroup = "80dade5d-1b70-0710-a002-c9d3604bcbd7"
	soOtherGroup         = "11111111-1111-4111-8111-111111111111"
)

func soTeams(t *testing.T) *SpecialistHandoffConfig {
	t.Helper()
	cfg, err := ParseSpecialistHandoffConfig(`{"products":[
		{"name":"Choreo","serviceIds":["b9c999f8-1b86-a010-00ae-86acdd4bcb61"],"teams":[
			{"key":"choreo-special-ops","label":"Choreo Special Ops","groupId":"fe0d8868-1b0b-3010-d64e-64a2604bcb3c"},
			{"key":"choreo-runtime-team","label":"Choreo Runtime Team","groupId":"` + soChoreoRuntimeGroup + `"}]},
		{"name":"Asgardeo","serviceIds":["97ed1b8b-1ba2-6c10-00ae-86acdd4bcbd3"],"teams":[
			{"key":"asgardeo-special-ops","label":"Asgardeo Special Ops","groupId":"7fb4f4c6-1b4b-3810-aea4-a936604bcb90"}]}]}`)
	if err != nil {
		t.Fatalf("config: %v", err)
	}
	return cfg
}

func groupChange(from, to any, by string) repository.IncidentReportChange {
	return repository.IncidentReportChange{
		OutboxID: 7, IncidentID: incidentReportTestID,
		Changes:    map[string]map[string]any{"assignment_group_id": {"from": from, "to": to}},
		Snapshot:   map[string]any{"updated_by": by},
		OccurredOn: time.Date(2026, 10, 8, 9, 30, 0, 0, time.UTC),
	}
}

func alertTx() *fakeIncidentReportTx {
	s := func(v string) *string { return &v }
	return &fakeIncidentReportTx{alertSrc: repository.SpecialOpsAlertSource{
		IncidentID: incidentReportTestID, Number: "INC0099990", Subject: "Choreo runtime degradation",
		Description: s("<p>pods restarting</p>"), State: s("IN_PROGRESS"), Priority: s("HIGH"),
		Impact: s("HIGH"), Urgency: s("MEDIUM"), ServiceID: s("b9c999f8-1b86-a010-00ae-86acdd4bcb61"), ServiceName: s("Choreo"),
		GroupName: s("Choreo Runtime Team"), PreviousGroupName: s("Choreo SRE"),
	}}
}

// A change to a Special Ops team's group publishes the alert with the
// incident, the team and the change.
func TestSpecialOpsAlert_PublishedForATeamGroup(t *testing.T) {
	pub := &alertPublisher{}
	svc := WithSpecialOpsAlerts(NewIncidentReportService(), pub, soTeams(t))
	if err := svc.HandleChange(context.Background(), alertTx(), groupChange(soOtherGroup, soChoreoRuntimeGroup, "jane.doe@wso2.com")); err != nil {
		t.Fatalf("HandleChange: %v", err)
	}
	if len(pub.sent) != 1 || pub.sent[0].Type != events.TypeIncidentSpecialOpsAlert || pub.sent[0].EntityID != incidentReportTestID {
		t.Fatalf("sent = %+v, want one incident.special_ops_alert keyed by the incident", pub.sent)
	}
	var got events.IncidentSpecialOpsAlertPayload
	if err := json.Unmarshal(pub.sent[0].Payload, &got); err != nil {
		t.Fatalf("payload: %v", err)
	}
	want := events.IncidentSpecialOpsAlertPayload{
		IncidentID: incidentReportTestID, Number: "INC0099990", Subject: "Choreo runtime degradation",
		Description: "<p>pods restarting</p>", State: "IN_PROGRESS", Priority: "HIGH", Impact: "HIGH", Urgency: "MEDIUM",
		ServiceID: "b9c999f8-1b86-a010-00ae-86acdd4bcb61", ServiceName: "Choreo",
		Product: "Choreo", TeamKey: "choreo-runtime-team", TeamLabel: "Choreo Runtime Team",
		AssignmentGroupID: soChoreoRuntimeGroup, AssignmentGroupName: "Choreo Runtime Team",
		PreviousAssignmentGroupID: soOtherGroup, PreviousAssignmentGroupName: "Choreo SRE",
		ChangedBy: "jane.doe@wso2.com", ChangedOn: "2026-10-08T09:30:00Z",
	}
	if got != want {
		t.Errorf("payload =\n%+v\nwant\n%+v", got, want)
	}
}

// Group ids match case-insensitively, and an incident with no previous group
// still alerts.
func TestSpecialOpsAlert_CaseInsensitiveAndNoPreviousGroup(t *testing.T) {
	pub := &alertPublisher{}
	svc := WithSpecialOpsAlerts(NewIncidentReportService(), pub, soTeams(t))
	if err := svc.HandleChange(context.Background(), alertTx(), groupChange(nil, "7FB4F4C6-1B4B-3810-AEA4-A936604BCB90", "system")); err != nil {
		t.Fatalf("HandleChange: %v", err)
	}
	if len(pub.sent) != 1 {
		t.Fatalf("sent = %d, want 1", len(pub.sent))
	}
	var got events.IncidentSpecialOpsAlertPayload
	_ = json.Unmarshal(pub.sent[0].Payload, &got)
	if got.TeamKey != "asgardeo-special-ops" || got.Product != "Asgardeo" || got.PreviousAssignmentGroupID != "" {
		t.Errorf("payload = %+v", got)
	}
}

// Any other group, a cleared group, no alerts configured, or a deleted
// incident publish nothing -- and never touch the report flows.
func TestSpecialOpsAlert_NothingOtherwise(t *testing.T) {
	cases := map[string]struct {
		svc    func(*alertPublisher) IncidentReportService
		change repository.IncidentReportChange
		tx     *fakeIncidentReportTx
	}{
		"another group": {
			svc: func(p *alertPublisher) IncidentReportService {
				return WithSpecialOpsAlerts(NewIncidentReportService(), p, soTeams(t))
			},
			change: groupChange(soChoreoRuntimeGroup, soOtherGroup, "x"), tx: alertTx(),
		},
		"group cleared": {
			svc: func(p *alertPublisher) IncidentReportService {
				return WithSpecialOpsAlerts(NewIncidentReportService(), p, soTeams(t))
			},
			change: groupChange(soChoreoRuntimeGroup, nil, "x"), tx: alertTx(),
		},
		"no alerts configured": {
			svc:    func(*alertPublisher) IncidentReportService { return NewIncidentReportService() },
			change: groupChange(soOtherGroup, soChoreoRuntimeGroup, "x"), tx: alertTx(),
		},
		"incident deleted": {
			svc: func(p *alertPublisher) IncidentReportService {
				return WithSpecialOpsAlerts(NewIncidentReportService(), p, soTeams(t))
			},
			change: groupChange(soOtherGroup, soChoreoRuntimeGroup, "x"),
			tx:     &fakeIncidentReportTx{alertSrcErr: repository.ErrIncidentNotFound},
		},
	}
	for name, c := range cases {
		pub := &alertPublisher{}
		if err := c.svc(pub).HandleChange(context.Background(), c.tx, c.change); err != nil {
			t.Errorf("%s: HandleChange: %v", name, err)
		}
		if len(pub.sent) != 0 || len(c.tx.created) != 0 || len(c.tx.reports) != 0 || len(c.tx.problems) != 0 {
			t.Errorf("%s: sent %d, report writes %d/%d/%d, want none", name, len(pub.sent), len(c.tx.created), len(c.tx.reports), len(c.tx.problems))
		}
	}
}

// A failed publish is returned, so the drainer retries the row.
func TestSpecialOpsAlert_PublishFailureIsRetried(t *testing.T) {
	boom := errors.New("event hub down")
	svc := WithSpecialOpsAlerts(NewIncidentReportService(), &alertPublisher{failWith: boom}, soTeams(t))
	if err := svc.HandleChange(context.Background(), alertTx(), groupChange(soOtherGroup, soChoreoRuntimeGroup, "x")); !errors.Is(err, boom) {
		t.Errorf("err = %v, want the publish error", err)
	}
}

// The dual-write flow publishes too: the alert is independent of where the
// workaround problem is created.
func TestSpecialOpsAlert_DualWrite(t *testing.T) {
	pub := &alertPublisher{}
	svc := WithSpecialOpsAlerts(NewDualWriteIncidentReportService(), pub, soTeams(t))
	if err := svc.HandleChange(context.Background(), alertTx(), groupChange(soOtherGroup, soChoreoRuntimeGroup, "x")); err != nil || len(pub.sent) != 1 {
		t.Errorf("sent %d, err %v, want one alert", len(pub.sent), err)
	}
}
