// Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied. See the License for the
// specific language governing permissions and limitations
// under the License.

package service

import (
	"context"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

func TestIsGithubRelayedComment(t *testing.T) {
	cases := []struct {
		content string
		want    bool
	}{
		{"@someone (GitHub Comment) https://github.com/example-org/example-repo/issues/1\n\n### Dev: done", true},
		{"@someone (GitHub Comment) [https://github.com/example-org/example-repo/issues/42#issuecomment-1]\n\nbody", true},
		{"  @a-b_c (GitHub Comment) https://github.com/x/y/issues/1", true},
		{"Please see @someone (GitHub Comment) https://github.com/x/y/issues/1", false},
		{"@someone (GitHub Comment) https://example.com/x/y/issues/1", false},
		{"@someone (Github comment) https://github.com/x/y/issues/1", false},
		{"Plain reply", false},
	}
	for _, c := range cases {
		if got := IsGithubRelayedComment(c.content); got != c.want {
			t.Errorf("IsGithubRelayedComment(%q) = %v, want %v", c.content, got, c.want)
		}
	}
}

type recordingCaseStateMover struct{ moved []string }

func (r *recordingCaseStateMover) MoveAwaitingInfoToWaitingOnWSO2(_ context.Context, caseID string) {
	r.moved = append(r.moved, caseID)
}

func TestHandleWebhook_RelayedCommentMovesItsCase(t *testing.T) {
	r := &fakeGhRepo{mapping: mapped(), caseID: "case-1", cr: &repository.GithubChangeRequest{ID: "cr-1"}}
	mover := &recordingCaseStateMover{}
	svc := writingSvc(r, &fakeGhMutations{}, &fakeGhClient{})
	svc.(*githubSyncService).caseStates = mover

	if _, err := svc.HandleWebhook(context.Background(), commentDelivery("Done on dev.", "someone")); err != nil {
		t.Fatalf("HandleWebhook: %v", err)
	}
	if len(mover.moved) != 1 || mover.moved[0] != "case-1" {
		t.Errorf("moved %v, want [case-1]", mover.moved)
	}
}

func TestHandleWebhook_SkippedCommentDoesNotMoveTheCase(t *testing.T) {
	r := &fakeGhRepo{mapping: mapped(), caseID: "case-1", cr: &repository.GithubChangeRequest{ID: "cr-1"}}
	mover := &recordingCaseStateMover{}
	svc := writingSvc(r, &fakeGhMutations{}, &fakeGhClient{})
	svc.(*githubSyncService).caseStates = mover

	if _, err := svc.HandleWebhook(context.Background(), commentDelivery("/close", "someone")); err != nil {
		t.Fatalf("HandleWebhook: %v", err)
	}
	if len(mover.moved) != 0 {
		t.Errorf("a slash command moved the case: %v", mover.moved)
	}
}

const githubRelayedContent = "@someone (GitHub Comment) https://github.com/example-org/example-repo/issues/1\n\nDone on dev."

// The real move, without a user token as the webhook relay calls it: only Awaiting Info moves, attributed to the system.
func TestMoveAwaitingInfoToWaitingOnWSO2_MovesOnlyAwaitingInfo(t *testing.T) {
	states := []struct {
		state domain.CaseState
		moves bool
	}{
		{domain.CaseStateAwaitingInfo, true},
		{domain.CaseStateSolutionProposed, false},
		{domain.CaseStateOpen, false},
		{domain.CaseStateWorkInProgress, false},
		{domain.CaseStateWaitingOnWSO2, false},
		{domain.CaseStateClosed, false},
	}
	for _, caseType := range []string{"case", "service_request"} {
		for _, tc := range states {
			t.Run(caseType+"/"+string(tc.state), func(t *testing.T) {
				f := newCustomerReplyFixture(caseType, tc.state)
				svc := NewCaseService(f.repo, f.userRepo, nil, stubAccess{scope: AccessScope{Unrestricted: true}}, nil).(*caseService)

				svc.MoveAwaitingInfoToWaitingOnWSO2(context.Background(), testDeploymentUUID)

				moves := f.movesTo()
				if !tc.moves {
					if len(moves) != 0 {
						t.Fatalf("moved %v from %s, want no move", moves, tc.state)
					}
					return
				}
				if len(moves) != 1 || moves[0] != domain.CaseStateWaitingOnWSO2 {
					t.Fatalf("moved %v, want exactly one move to waiting_on_wso2", moves)
				}
				acts := f.activities()
				if len(acts) != 1 || acts[0].field != "state" || acts[0].actor != domain.AutoclosureDefaultActor {
					t.Errorf("activity = %+v, want one state entry by %q", acts, domain.AutoclosureDefaultActor)
				}
			})
		}
	}
}

// A relayed comment posted with a user token moves the case and keeps that user as the actor.
func TestCreateCaseComment_GithubRelayedCommentKeepsTheTokenUserAsActor(t *testing.T) {
	f := newCustomerReplyFixture("service_request", domain.CaseStateAwaitingInfo)
	svc := NewCaseService(f.repo, f.userRepo, nil, customerReplyExternalAccess, nil)
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, customerReplyCustomerEmail))
	req := customerReplyRequest(domain.CommentTypeComment)
	req.Content = githubRelayedContent

	if _, err := svc.CreateCaseComment(ctx, req); err != nil {
		t.Fatalf("CreateCaseComment: %v", err)
	}
	if moves := f.movesTo(); len(moves) != 1 || moves[0] != domain.CaseStateWaitingOnWSO2 {
		t.Fatalf("moved %v, want exactly one move to waiting_on_wso2", moves)
	}
	if acts := f.activities(); len(acts) != 1 || acts[0].actor != customerReplyCustomerEmail {
		t.Errorf("activity = %+v, want one entry by %q", acts, customerReplyCustomerEmail)
	}
}

// A relayed comment stored as a work note never moves the case.
func TestCreateCaseComment_GithubRelayedWorkNoteDoesNotMove(t *testing.T) {
	f := newCustomerReplyFixture("case", domain.CaseStateAwaitingInfo)
	svc := NewCaseService(f.repo, f.userRepo, nil, stubAccess{scope: AccessScope{Unrestricted: true}}, nil)
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "eng@wso2.com"))
	req := customerReplyRequest(domain.CommentTypeWorkNote)
	req.Content = githubRelayedContent

	if _, err := svc.CreateCaseComment(ctx, req); err != nil {
		t.Fatalf("CreateCaseComment: %v", err)
	}
	if moves := f.movesTo(); len(moves) != 0 {
		t.Errorf("a work note moved the case: %v", moves)
	}
}
