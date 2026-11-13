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
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// decideRepo is a StandardChangeRepository that answers what deciding a
// proposal and reading it back need; any other method panics on the nil
// embedded interface.
type decideRepo struct {
	repository.StandardChangeRepository
	proposal domain.StandardChangeProposal
	cab      map[string]bool // email -> on the CAB now
	decided  []string        // the expectedStage of each DecideProposal call
}

func (r *decideRepo) GetProposal(context.Context, string) (*domain.StandardChangeProposal, error) {
	p := r.proposal
	p.Approvers = append([]domain.StandardChangeProposalApprover(nil), r.proposal.Approvers...)
	return &p, nil
}

func (r *decideRepo) IsGroupMember(_ context.Context, email, group string) (bool, error) {
	return group == domain.CABApprovalGroupName && r.cab[strings.ToLower(email)], nil
}

func (r *decideRepo) DecideProposal(_ context.Context, _, _ string, _ bool, _ *string, expectedStage string) error {
	r.decided = append(r.decided, expectedStage)
	return nil
}

const (
	decideProposalID = "5b3f1c2e-0000-4000-8000-0000000000a1"
	decideStageID    = "5b3f1c2e-0000-4000-8000-0000000000b1"
	decideMember     = "member@scp.test"
	decideLeaver     = "leaver@scp.test"
)

func newDecideRepo() *decideRepo {
	row := func(email string) domain.StandardChangeProposalApprover {
		return domain.StandardChangeProposalApprover{ID: email, StageID: decideStageID, State: "REQUESTED",
			Approver: domain.NewUserReference("u-"+email, email, email)}
	}
	return &decideRepo{
		proposal: domain.StandardChangeProposal{
			ID: decideProposalID, State: domain.StandardChangeProposalStateAuthorityReview,
			ProposedBy: domain.NewUserReference("u-p", "proposer@scp.test", "Proposer"),
			Stages:     []domain.StandardChangeProposalStage{{ID: decideStageID, RequiredApprovals: domain.StandardChangeCABApprovals}},
			Approvers:  []domain.StandardChangeProposalApprover{row(decideMember), row(decideLeaver)},
		},
		// The leaver was on the CAB at submit (they have a row) and has left since.
		cab: map[string]bool{decideMember: true},
	}
}

// A decision without the stage it was made on is refused before anything is
// read or written: the stale-stage guard is not something a caller can skip by
// leaving the field out.
func TestStandardChangeDecide_RequiresStage(t *testing.T) {
	for name, stage := range map[string]*string{"absent": nil, "empty": stdp(""), "blank": stdp("  ")} {
		t.Run(name, func(t *testing.T) {
			repo := newDecideRepo()
			svc := NewStandardChangeService(repo, ctxIdentityAccess{})
			_, err := svc.DecideProposal(stdCtx(decideMember), decideProposalID, domain.DecideStandardChangeProposalRequest{Decision: "approved", StageID: stage})
			var ve *apierror.ValidationError
			if err == nil || !errors.As(err, &ve) || !strings.Contains(err.Error(), "stageId is required") {
				t.Fatalf("err = %v (%T), want a ValidationError naming stageId", err, err)
			}
			if len(repo.decided) != 0 {
				t.Fatalf("the repository was asked to decide: %v", repo.decided)
			}
		})
	}
	repo := newDecideRepo()
	svc := NewStandardChangeService(repo, ctxIdentityAccess{})
	_, err := svc.DecideProposal(stdCtx(decideMember), decideProposalID, domain.DecideStandardChangeProposalRequest{Decision: "approved", StageID: stdp("not-a-uuid")})
	var ve *apierror.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("a malformed stageId: err = %v (%T)", err, err)
	}
	upper := " " + strings.ToUpper(decideStageID) + " "
	if _, err := svc.DecideProposal(stdCtx(decideMember), decideProposalID, domain.DecideStandardChangeProposalRequest{Decision: "approved", StageID: &upper}); err != nil {
		t.Fatalf("a decision with its stage: %v", err)
	}
	if len(repo.decided) != 1 || repo.decided[0] != decideStageID {
		t.Fatalf("the repository got stage %v, want [%s]", repo.decided, decideStageID)
	}
}

// Someone who has left the CAB since the proposal was submitted keeps their
// open row (it was written at submit) but is not offered a decision, which the
// server would refuse; a current member with a row is.
func TestStandardChangeDecide_CanDecideNeedsTheCABNow(t *testing.T) {
	svc := NewStandardChangeService(newDecideRepo(), ctxIdentityAccess{})
	for email, want := range map[string]bool{decideMember: true, decideLeaver: false} {
		p, err := svc.GetProposal(stdCtx(email), decideProposalID)
		if err != nil {
			t.Fatal(err)
		}
		for _, a := range p.Approvers {
			mine := a.Approver.Email == email
			if a.CanDecide != (mine && want) {
				t.Fatalf("viewer %s: row of %s CanDecide = %v", email, a.Approver.Email, a.CanDecide)
			}
		}
		if p.CanCancel != want {
			t.Fatalf("viewer %s: CanCancel = %v, want %v", email, p.CanCancel, want)
		}
	}
}
