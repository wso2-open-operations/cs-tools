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
	"fmt"
	"log/slog"
	"strings"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// caseEscalationSearchPageSize is the page size used internally when reading
// a case's escalation history through the generic EscalationService.
// GET /cases/{id}/escalations promises the case's FULL history, and repeated
// escalate/de-escalate actions can in principle produce more than one page's
// worth of records, so SearchCaseEscalations below pages through every result
// rather than returning just the first page.
const caseEscalationSearchPageSize = 100

type caseEscalationService struct {
	escalations EscalationService
	caseSvc     CaseService
}

// NewCaseEscalationService constructs a CaseEscalationService: a case-scoped
// convenience layer over the generic EscalationService (which mirrors the
// backing API's own /escalations resource 1:1, shared with other consumers of
// this entity service). This layer adds three things the generic resource
// doesn't: pagination looped to the full history, the case's current
// notified-users list (who may de-escalate), and a work-note side effect on
// create -- see NewServiceNowEscalationService's own doc comment for why the
// work note isn't baked into the shared generic service instead.
func NewCaseEscalationService(escalations EscalationService, caseSvc CaseService) CaseEscalationService {
	return &caseEscalationService{escalations: escalations, caseSvc: caseSvc}
}

// SearchCaseEscalations implements CaseEscalationService.
func (s *caseEscalationService) SearchCaseEscalations(ctx context.Context, caseID string) (domain.CaseEscalationHistory, error) {
	escalations := make([]domain.Escalation, 0, caseEscalationSearchPageSize)
	total := 0

	for offset := 0; ; offset += caseEscalationSearchPageSize {
		resp, err := s.escalations.SearchEscalations(ctx, domain.SearchEscalationsRequest{
			Filters: &domain.SearchEscalationsFilters{CaseIDs: []string{caseID}},
			// Newest first, explicitly -- CurrentNotifiedUsers below relies on
			// escalations[0] being the most recent record.
			SortBy: &domain.EscalationSort{
				Field: domain.EscalationSortFieldCreatedOn,
				Order: domain.EscalationSortOrderDesc,
			},
			Pagination: domain.Pagination{Limit: caseEscalationSearchPageSize, Offset: offset},
		})
		if err != nil {
			return domain.CaseEscalationHistory{}, err
		}

		escalations = append(escalations, resp.Escalations...)
		total = resp.Total
		if len(resp.Escalations) == 0 || len(escalations) >= total {
			break
		}
	}

	currentNotifiedUsers := []domain.EscalationNotifiedUser{}
	if len(escalations) > 0 && escalations[0].NotificationSentTo != nil {
		currentNotifiedUsers = escalations[0].NotificationSentTo
	}

	return domain.CaseEscalationHistory{
		Escalations:          escalations,
		Total:                total,
		CurrentNotifiedUsers: currentNotifiedUsers,
	}, nil
}

// CreateCaseEscalation implements CaseEscalationService.
func (s *caseEscalationService) CreateCaseEscalation(ctx context.Context, caseID string, reason *string, action *domain.EscalationAction) (domain.CreatedEscalation, error) {
	resp, err := s.escalations.CreateEscalation(ctx, domain.CreateEscalationRequest{
		CaseID: caseID,
		Reason: reason,
		Action: action,
	})
	if err != nil {
		return domain.CreatedEscalation{}, err
	}

	// The escalation record itself carries no case activity of its own
	// (verified live against SN dev: creating a case-escalation row produces
	// no journal/work-note entry on the parent case). Record one here so the
	// case's comment trail reflects the action. The escalation already
	// happened by this point, so a failure here must not fail the request --
	// log and return the successful escalation instead of telling the caller
	// their escalation failed when it didn't.
	// Normalize the same way snEscalationService.CreateEscalation does before
	// comparing -- a caller can send any case ("deescalate"), and this
	// content string must agree with what was actually just recorded, not
	// silently mismatch on an unnormalized case.
	effectiveAction := domain.EscalationActionEscalate
	if action != nil {
		effectiveAction = domain.EscalationAction(strings.ToUpper(string(*action)))
	}
	if _, err := s.caseSvc.CreateCaseComment(ctx, domain.CreateCaseCommentRequest{
		CaseID:  caseID,
		Type:    domain.CommentTypeWorkNote,
		Content: caseEscalationWorkNoteContent(effectiveAction, resp.Escalation),
	}); err != nil {
		slog.ErrorContext(ctx, "create case escalation: failed to record case work note",
			"caseID", caseID, "escalationID", resp.Escalation.ID, "error", err)
	}

	return resp.Escalation, nil
}

// caseEscalationWorkNoteContent builds the case work note text recorded after
// a successful escalation create, e.g. "Case escalated from EL1 to EL2.
// Reason: customer requested management involvement." or "Case de-escalated
// from EL2 to EL1." (no reason line when none was given). Level wording comes
// from the backing service's own choice-list labels rather than a hardcoded
// map, so it can't drift from whatever SN's escalation_level choices say.
func caseEscalationWorkNoteContent(action domain.EscalationAction, e domain.CreatedEscalation) string {
	verb := "escalated"
	if action == domain.EscalationActionDeescalate {
		verb = "de-escalated"
	}

	content := fmt.Sprintf("Case %s from %s to %s.", verb, e.PreviousLevel.Label, e.CurrentLevel.Label)
	if e.Reason != nil && *e.Reason != "" {
		content += fmt.Sprintf(" Reason: %s.", *e.Reason)
	}
	return content
}
