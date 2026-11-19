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
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/validate"
)

const (
	// waitingOnProductTeamTag is the case tag meaning "waiting on a product team".
	waitingOnProductTeamTag = "s_dip"
	// productIssueTagActor is created_by on the tag rows and work notes the rule
	// writes. It is also how the rule tells its own tag attachment from a person's:
	// a tag attached under any other actor is manual.
	productIssueTagActor = "product-issue-case-tags"
	// productCaseTagRuleTimeout bounds one hook run made after the caller's own
	// write has committed.
	productCaseTagRuleTimeout = 15 * time.Second
)

// ProductIssueCaseTagRule keeps the case tag s_dip ("waiting on a product team")
// in step with the product issues linked to the case.
//
// s_dip is on a case exactly when it has at least one active AFFECTS link from a
// product issue whose effective status is not RESOLVED or CLOSED (CLOSED when
// GitHub says the issue is closed, else the board status; an issue not on a board
// counts as open). Two limits keep it from fighting people:
//
//   - It only acts on a case that has, or had, an AFFECTS link from a product issue.
//   - It only removes an s_dip it added itself (case_tag_automation, migration
//     0308, and a tag attachment written under productIssueTagActor). An s_dip a
//     person added, before or after the system, stays until a person removes it.
//
// The rule is recomputed from the case's current tags and links on every call, so
// a person removing the tag while an issue is open has it added back, and a call
// that finds nothing to change writes nothing.
type ProductIssueCaseTagRule interface {
	// ApplyCaseTagRuleForCase converges one case.
	ApplyCaseTagRuleForCase(ctx context.Context, caseID string) error
	// ApplyCaseTagRuleForWorkItem converges every case the product issue has an
	// AFFECTS link to, active or not. Every case is tried; the errors are joined.
	ApplyCaseTagRuleForWorkItem(ctx context.Context, productIssueID string) error
}

// productIssueCaseTagCases is the slice of CaseService the rule uses.
type productIssueCaseTagCases interface {
	AddCaseTagAs(ctx context.Context, caseID, label, actorEmail string) (domain.Tag, error)
	RemoveCaseTagAs(ctx context.Context, caseID, tagID, actorEmail string) error
	CreateCaseCommentAs(ctx context.Context, req domain.CreateCaseCommentRequest, actorEmail string) (domain.CreateCaseCommentResponse, error)
}

type productIssueCaseTagService struct {
	repo  repository.ProductIssueCaseTagRepository
	cases productIssueCaseTagCases
}

// NewProductIssueCaseTagRule constructs the rule.
func NewProductIssueCaseTagRule(repo repository.ProductIssueCaseTagRepository, cases productIssueCaseTagCases) ProductIssueCaseTagRule {
	return &productIssueCaseTagService{repo: repo, cases: cases}
}

func (s *productIssueCaseTagService) ApplyCaseTagRuleForWorkItem(ctx context.Context, productIssueID string) error {
	if !validate.IsUUID(productIssueID) {
		return &apierror.ValidationError{Msg: fmt.Sprintf("id contains invalid UUID: %q", productIssueID)}
	}
	caseIDs, err := s.repo.CaseIDsAffectedBy(ctx, productIssueID)
	if err != nil {
		return err
	}
	var errs []error
	for _, id := range caseIDs {
		if err := s.ApplyCaseTagRuleForCase(ctx, id); err != nil {
			errs = append(errs, fmt.Errorf("case %s: %w", id, err))
		}
	}
	return errors.Join(errs...)
}

func (s *productIssueCaseTagService) ApplyCaseTagRuleForCase(ctx context.Context, caseID string) error {
	if !validate.IsUUID(caseID) {
		return &apierror.ValidationError{Msg: fmt.Sprintf("id contains invalid UUID: %q", caseID)}
	}
	return s.repo.WithCaseLock(ctx, caseID, func(ctx context.Context, tx repository.CaseTagRuleTx) error {
		st, err := tx.State(ctx, waitingOnProductTeamTag)
		if err != nil {
			return err
		}
		if !st.HasProductLink {
			return nil // never a product issue's case: not the rule's to touch
		}
		want, has := len(st.OpenIssues) > 0, len(st.Tags) > 0
		// The rule's own tag: recorded, and every attachment written by the rule. A
		// record beside a tag a person attached since is stale: the tag is manual now.
		system := st.Recorded && has
		for _, a := range st.Tags {
			if a.CreatedBy != productIssueTagActor {
				system = false
			}
		}

		switch {
		case want && !has:
			// Record first, in the lock's transaction: if adding the tag fails the
			// record rolls back with it.
			if err := tx.RecordAdd(ctx, waitingOnProductTeamTag); err != nil {
				return err
			}
			if _, err := s.cases.AddCaseTagAs(ctx, caseID, waitingOnProductTeamTag, productIssueTagActor); err != nil {
				return fmt.Errorf("add %s: %w", waitingOnProductTeamTag, err)
			}
			s.note(ctx, caseID, waitingOnProductTeamTag+" added automatically: "+openIssuesText(st.OpenIssues))
		case !want && has && system:
			for _, a := range st.Tags {
				if err := s.cases.RemoveCaseTagAs(ctx, caseID, a.TagID, productIssueTagActor); err != nil {
					var nf *apierror.NotFoundError
					if !errors.As(err, &nf) { // already gone: a person removed it at the same moment
						return fmt.Errorf("remove %s: %w", waitingOnProductTeamTag, err)
					}
				}
			}
			if err := tx.ClearRecord(ctx, waitingOnProductTeamTag); err != nil {
				return err
			}
			reason := "every linked product issue is resolved or closed"
			if st.ActiveLinks == 0 {
				reason = "no product issue is linked to the case any more"
			}
			s.note(ctx, caseID, waitingOnProductTeamTag+" removed automatically: "+reason)
		case st.Recorded && !system:
			// A record nothing backs (a person removed the tag, or took it over):
			// drop it so the tag is manual from here on.
			return tx.ClearRecord(ctx, waitingOnProductTeamTag)
		}
		return nil
	})
}

// note writes the work note for an automatic change. The tag change has already
// happened, so a failure here is logged and never fails or rolls back the rule.
func (s *productIssueCaseTagService) note(ctx context.Context, caseID, content string) {
	if _, err := s.cases.CreateCaseCommentAs(ctx, domain.CreateCaseCommentRequest{
		CaseID: caseID, Type: domain.CommentTypeWorkNote, Content: content,
	}, productIssueTagActor); err != nil {
		slog.WarnContext(ctx, "case tag rule: the tag changed but its work note could not be written", "caseId", caseID, "err", err)
	}
}

func openIssuesText(numbers []string) string {
	if len(numbers) == 1 {
		return "product issue " + numbers[0] + " is open"
	}
	return "product issues " + strings.Join(numbers, ", ") + " are open"
}

// applyCaseTagRule runs the rule after the caller's write has committed. It is best
// effort: a failure is logged and never fails the user's request. It runs even if
// the caller has gone away, bounded by productCaseTagRuleTimeout.
func applyCaseTagRule(ctx context.Context, rule ProductIssueCaseTagRule, caseID string) {
	if rule == nil {
		return
	}
	ruleCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), productCaseTagRuleTimeout)
	defer cancel()
	if err := rule.ApplyCaseTagRuleForCase(ruleCtx, caseID); err != nil {
		slog.ErrorContext(ctx, "case tag rule: could not converge the case's tags", "caseId", caseID, "err", err)
	}
}

// WithProductCaseTagRule attaches the case tag rule to a product work item service
// (the create flow) and returns it. A nil rule leaves the service as it is.
func WithProductCaseTagRule(svc ProductWorkItemService, rule ProductIssueCaseTagRule) ProductWorkItemService {
	if s, ok := svc.(*productWorkItemService); ok {
		s.tagRule = rule
	}
	return svc
}

// WithProductWorkItemSLA attaches the SLA engine to a product work item service
// (the create flow) so a new product issue's clock is registered at creation.
// A nil engine leaves the service as it is.
func WithProductWorkItemSLA(svc ProductWorkItemService, sla WorkItemSLAEngine) ProductWorkItemService {
	if s, ok := svc.(*productWorkItemService); ok {
		s.sla = sla
	}
	return svc
}

// WithProductIssueCaseTagRule attaches the case tag rule to a product issue service
// (manual link and unlink) and returns it.
func WithProductIssueCaseTagRule(svc ProductIssueService, rule ProductIssueCaseTagRule) ProductIssueService {
	if s, ok := svc.(*productIssueService); ok {
		s.tagRule = rule
	}
	return svc
}
