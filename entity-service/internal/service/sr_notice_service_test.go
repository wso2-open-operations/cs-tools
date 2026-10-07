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

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/events"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

const (
	srTestCaseID = "11111111-1111-1111-1111-111111111111"
	// MS/PC SRE Group's sys_id as a UUID. The automated-team test configures
	// it upper-cased and padded, to prove the allow-list match ignores both.
	srTestTeamID = "6c3db375-1b1c-b2d0-a002-c9d3604bcb0c"
)

type fakeSRNoticeRepo struct {
	sr        repository.ServiceRequest
	found     bool
	getErr    error
	assignErr error
	ackErr    error

	assigned     []string // groupID per AssignToGroup call
	acknowledged []string // comment text per Acknowledge call
}

func (f *fakeSRNoticeRepo) GetServiceRequest(context.Context, string) (repository.ServiceRequest, bool, error) {
	return f.sr, f.found, f.getErr
}
func (f *fakeSRNoticeRepo) AssignToGroup(_ context.Context, _, groupID, _ string) error {
	f.assigned = append(f.assigned, groupID)
	return f.assignErr
}
func (f *fakeSRNoticeRepo) Acknowledge(_ context.Context, _, _, comment string) (string, error) {
	f.acknowledged = append(f.acknowledged, comment)
	return "c0000000-0000-0000-0000-000000000001", f.ackErr
}

func newTestSR(teamID, teamName string) repository.ServiceRequest {
	return repository.ServiceRequest{
		ID: srTestCaseID, Number: "CS-PORTAL-000042", WSO2CaseID: "ACME-12", Subject: "Rotate the API key",
		Description: "please", State: "OPEN", ProjectName: "ACME", SRETeamID: teamID, SRETeamName: teamName,
		CreatedBy: "jane@acme.test", CreatedOn: time.Date(2026, 10, 7, 9, 0, 0, 0, time.UTC),
		Tags: []string{"devops-sm"},
	}
}

func srPublishedTypes(m *mockEventPublisher) []events.Type {
	var out []events.Type
	for _, c := range m.calls {
		out = append(out, c.eventType)
	}
	return out
}

func TestSRNoticeService_OnCreated_AutomatedTeam(t *testing.T) {
	repo := &fakeSRNoticeRepo{sr: newTestSR(srTestTeamID, "MS/PC SRE Group"), found: true}
	pub := &mockEventPublisher{}
	svc := NewSRNoticeService(repo, pub, []string{" 6C3DB375-1B1C-B2D0-A002-C9D3604BCB0C "})

	svc.OnCreated(context.Background(), srTestCaseID)

	if len(repo.assigned) != 1 || repo.assigned[0] != srTestTeamID {
		t.Fatalf("assigned = %v, want the SR's SRE team once", repo.assigned)
	}
	if len(repo.acknowledged) != 1 || repo.acknowledged[0] != srAcknowledgement {
		t.Fatalf("acknowledged = %v, want ServiceNow's acknowledgement once", repo.acknowledged)
	}
	got := srPublishedTypes(pub)
	if len(got) != 2 || got[0] != events.TypeSRCreated || got[1] != events.TypeSRAcknowledged {
		t.Fatalf("published %v, want [sr.created sr.acknowledged] in that order", got)
	}
	var created events.SRCreatedPayload
	if err := json.Unmarshal(pub.calls[0].payload, &created); err != nil {
		t.Fatal(err)
	}
	if created.AssignmentGroupName != "MS/PC SRE Group" || created.State != "Open" || created.SRETeamName != "MS/PC SRE Group" ||
		created.Number != "CS-PORTAL-000042" || created.CreatedOn != "2026-10-07T09:00:00Z" {
		t.Errorf("sr.created payload = %+v", created)
	}
	var ack events.SRAcknowledgedPayload
	if err := json.Unmarshal(pub.calls[1].payload, &ack); err != nil {
		t.Fatal(err)
	}
	if ack.CommentID == "" || ack.AssignmentGroupName != "MS/PC SRE Group" {
		t.Errorf("sr.acknowledged payload = %+v", ack)
	}
	for _, c := range pub.calls {
		if c.entityID != srTestCaseID {
			t.Errorf("%s keyed by %q, want the SR id so the events stay ordered", c.eventType, c.entityID)
		}
	}
}

func TestSRNoticeService_OnCreated_TeamNotAutomated(t *testing.T) {
	for name, sr := range map[string]repository.ServiceRequest{
		"other team": newTestSR("22222222-2222-2222-2222-222222222222", "Apollo SRE Group"),
		"no team":    newTestSR("", ""),
	} {
		t.Run(name, func(t *testing.T) {
			repo := &fakeSRNoticeRepo{sr: sr, found: true}
			pub := &mockEventPublisher{}
			NewSRNoticeService(repo, pub, []string{srTestTeamID}).OnCreated(context.Background(), srTestCaseID)

			if len(repo.assigned) != 0 || len(repo.acknowledged) != 0 {
				t.Errorf("assigned=%v acknowledged=%v, want neither for a team that is not automated", repo.assigned, repo.acknowledged)
			}
			if got := srPublishedTypes(pub); len(got) != 1 || got[0] != events.TypeSRCreated {
				t.Errorf("published %v, want sr.created only", got)
			}
		})
	}
}

func TestSRNoticeService_OnCreated_AssignFailureStillAcknowledges(t *testing.T) {
	repo := &fakeSRNoticeRepo{sr: newTestSR(srTestTeamID, "MS/PC SRE Group"), found: true, assignErr: errors.New("boom")}
	pub := &mockEventPublisher{}
	NewSRNoticeService(repo, pub, []string{srTestTeamID}).OnCreated(context.Background(), srTestCaseID)

	var created events.SRCreatedPayload
	if err := json.Unmarshal(pub.calls[0].payload, &created); err != nil {
		t.Fatal(err)
	}
	if created.AssignmentGroupName != "" {
		t.Errorf("card would claim the SR is assigned to %q after the assignment failed", created.AssignmentGroupName)
	}
	if len(repo.acknowledged) != 1 {
		t.Errorf("acknowledged %d times, want 1: ServiceNow acknowledges even when assignment_group stays unset", len(repo.acknowledged))
	}
}

func TestSRNoticeService_OnCreated_AcknowledgeFailurePublishesNoAck(t *testing.T) {
	repo := &fakeSRNoticeRepo{sr: newTestSR(srTestTeamID, "MS/PC SRE Group"), found: true, ackErr: errors.New("boom")}
	pub := &mockEventPublisher{}
	NewSRNoticeService(repo, pub, []string{srTestTeamID}).OnCreated(context.Background(), srTestCaseID)

	if got := srPublishedTypes(pub); len(got) != 1 || got[0] != events.TypeSRCreated {
		t.Errorf("published %v, want sr.created only when the acknowledgement was not written", got)
	}
}

func TestSRNoticeService_OnCreated_NotAServiceRequest(t *testing.T) {
	repo := &fakeSRNoticeRepo{found: false}
	pub := &mockEventPublisher{}
	NewSRNoticeService(repo, pub, []string{srTestTeamID}).OnCreated(context.Background(), srTestCaseID)
	if len(pub.calls) != 0 || len(repo.assigned) != 0 {
		t.Errorf("published %v / assigned %v for a case that is not an SR", srPublishedTypes(pub), repo.assigned)
	}
}

func TestSRNoticeService_OnComment(t *testing.T) {
	created := time.Date(2026, 10, 7, 10, 0, 0, 0, time.UTC)
	tests := []struct {
		name     string
		found    bool
		typ      domain.CommentType
		wantType events.SRCommentType
		publish  bool
	}{
		{"comment on an SR", true, domain.CommentTypeComment, events.SRCommentTypeComment, true},
		{"work note on an SR", true, domain.CommentTypeWorkNote, events.SRCommentTypeWorkNote, true},
		{"activity entry", true, domain.CommentTypeActivity, "", false},
		{"comment on a case that is not an SR", false, domain.CommentTypeComment, "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &fakeSRNoticeRepo{sr: newTestSR(srTestTeamID, "MS/PC SRE Group"), found: tt.found}
			pub := &mockEventPublisher{}
			NewSRNoticeService(repo, pub, nil).OnComment(context.Background(), srTestCaseID, "cm-1", tt.typ,
				"it is still broken", "jane@acme.test", "Jane", created)

			if !tt.publish {
				if len(pub.calls) != 0 {
					t.Errorf("published %v, want nothing", srPublishedTypes(pub))
				}
				return
			}
			if len(pub.calls) != 1 || pub.calls[0].eventType != events.TypeSRCommentAdded {
				t.Fatalf("published %v, want one sr.comment_added", srPublishedTypes(pub))
			}
			var p events.SRCommentAddedPayload
			if err := json.Unmarshal(pub.calls[0].payload, &p); err != nil {
				t.Fatal(err)
			}
			if p.CommentType != tt.wantType || p.AuthorEmail != "jane@acme.test" || p.CommentID != "cm-1" ||
				len(p.Tags) != 1 || p.Tags[0] != "devops-sm" || p.CreatedOn != "2026-10-07T10:00:00Z" || p.SRETeamName != "MS/PC SRE Group" {
				t.Errorf("payload = %+v", p)
			}
		})
	}
}

func TestSRNoticeService_OnComment_NoAuthorEmail(t *testing.T) {
	repo := &fakeSRNoticeRepo{sr: newTestSR(srTestTeamID, "MS/PC SRE Group"), found: true}
	pub := &mockEventPublisher{}
	NewSRNoticeService(repo, pub, nil).OnComment(context.Background(), srTestCaseID, "cm-1", domain.CommentTypeComment,
		"hello", " ", "", time.Now())
	if len(pub.calls) != 0 {
		t.Errorf("published %v for a comment with no author email", srPublishedTypes(pub))
	}
}

func TestSRStateLabel(t *testing.T) {
	for in, want := range map[string]string{"OPEN": "Open", "WAITING_ON_WSO2": "Waiting On WSO2", "WORK_IN_PROGRESS": "Work In Progress", "": ""} {
		if got := srStateLabel(in); got != want {
			t.Errorf("srStateLabel(%q) = %q, want %q", in, got, want)
		}
	}
}

type recordingSRNotifier struct {
	created  []string
	comments []domain.CommentType
}

func (r *recordingSRNotifier) OnCreated(_ context.Context, caseID string) {
	r.created = append(r.created, caseID)
}
func (r *recordingSRNotifier) OnComment(_ context.Context, _, _ string, commentType domain.CommentType, _, _, _ string, _ time.Time) {
	r.comments = append(r.comments, commentType)
}

// TestCaseService_CreateCaseComment_NotifiesSRWithoutRecipients: the
// sr.comment_added hook must not depend on case.comment_added, which is
// skipped for a case with no recipients -- an SR the devops-sm alert is about
// may well have none.
func TestCaseService_CreateCaseComment_NotifiesSRWithoutRecipients(t *testing.T) {
	repo := &stubCaseRepo{
		createCaseComment: func(_ context.Context, req domain.CreateCaseCommentRequest, _ *time.Time) (domain.CaseComment, error) {
			return domain.CaseComment{ID: "comment-1", CaseID: req.CaseID, Type: req.Type, Content: req.Content}, nil
		},
	}
	// The comment path also subscribes the commenter to the watch list
	// (subscribeCommenterToWatchList, #2459), which looks the author up. An
	// unknown author is skipped there, so it stays out of this test's way.
	users := stubUserRepo{getUserByEmail: func(context.Context, string) (domain.User, error) {
		return domain.User{}, errors.New("no such user")
	}}
	svc := NewCaseService(repo, users, nil, alwaysUnrestrictedAccess{}, nil)
	rec := &recordingSRNotifier{}
	svc.(*caseService).srNotices = rec

	req := domain.CreateCaseCommentRequest{CaseID: testDeploymentUUID, Type: domain.CommentTypeComment, Content: "still broken"}
	if _, err := svc.CreateCaseCommentAs(context.Background(), req, "jane@acme.test"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rec.comments) != 1 || rec.comments[0] != domain.CommentTypeComment {
		t.Errorf("OnComment calls = %v, want one for the comment", rec.comments)
	}
}

// TestWithSRNotices_IgnoresOtherCaseServices: the ServiceNow-backed case
// service has no Postgres SR to automate.
func TestWithSRNotices_IgnoresOtherCaseServices(t *testing.T) {
	sn := &snCaseService{}
	if got := WithSRNotices(sn, NewSRNoticeService(&fakeSRNoticeRepo{}, nil, nil)); got != CaseService(sn) {
		t.Errorf("WithSRNotices changed a non-Postgres case service")
	}
}
