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
	"errors"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// oneEscalationHistory returns a single EL1 escalation whose list is
// notified, so SearchCaseEscalations has a CurrentNotifiedUsers to report.
type oneEscalationHistory struct{ EscalationService }

func (oneEscalationHistory) SearchEscalations(context.Context, domain.SearchEscalationsRequest) (domain.SearchEscalationsResponse, error) {
	email := "notified@wso2.com"
	return domain.SearchEscalationsResponse{
		Escalations: []domain.Escalation{{ID: "e1", NotificationSentTo: []domain.EscalationNotifiedUser{{ID: "u-notified", Email: &email}}}},
		Total:       1,
	}, nil
}

type fakeTeamLeads struct {
	leads []domain.EscalationNotifiedUser
	err   error
}

func (f fakeTeamLeads) CaseTeamLeads(context.Context, string) ([]domain.EscalationNotifiedUser, error) {
	return f.leads, f.err
}

func TestSearchCaseEscalations_TeamLeads(t *testing.T) {
	lead := "lead@wso2.com"
	svc := WithCaseTeamLeads(NewCaseEscalationService(oneEscalationHistory{}, nil),
		fakeTeamLeads{leads: []domain.EscalationNotifiedUser{{ID: "u-lead", Email: &lead}}})

	got, err := svc.SearchCaseEscalations(context.Background(), escalationTestCaseID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.TeamLeads) != 1 || got.TeamLeads[0].ID != "u-lead" {
		t.Errorf("TeamLeads = %+v, want the one ABT lead", got.TeamLeads)
	}
	if len(got.CurrentNotifiedUsers) != 1 || got.CurrentNotifiedUsers[0].ID != "u-notified" {
		t.Errorf("CurrentNotifiedUsers = %+v, want the latest escalation's list unchanged", got.CurrentNotifiedUsers)
	}
}

// TestSearchCaseEscalations_NoTeamLeadReader: without the lookup (no
// database), TeamLeads is an empty array -- never null -- so nobody is shown
// the de-escalate action rather than the client failing.
func TestSearchCaseEscalations_NoTeamLeadReader(t *testing.T) {
	got, err := NewCaseEscalationService(oneEscalationHistory{}, nil).SearchCaseEscalations(context.Background(), escalationTestCaseID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.TeamLeads == nil || len(got.TeamLeads) != 0 {
		t.Errorf("TeamLeads = %#v, want an empty, non-nil array", got.TeamLeads)
	}
}

func TestSearchCaseEscalations_TeamLeadLookupFails(t *testing.T) {
	svc := WithCaseTeamLeads(NewCaseEscalationService(oneEscalationHistory{}, nil), fakeTeamLeads{err: errors.New("db down")})
	if _, err := svc.SearchCaseEscalations(context.Background(), escalationTestCaseID); err == nil {
		t.Fatal("a failed team-lead lookup must fail the read, not report nobody as a lead")
	}
}
