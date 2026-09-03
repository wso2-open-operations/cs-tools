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
	"fmt"
	"log/slog"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/middleware"
	integrationservice "github.com/wso2-open-operations/cs-tools/entity-service/internal/servicenow-integration-service"
)

// validEscalationAction is the set of accepted CreateEscalationRequest.Action
// values. Mirrors the backing service's own default: an absent action means
// ESCALATE.
var validEscalationAction = map[domain.EscalationAction]bool{
	domain.EscalationActionEscalate:   true,
	domain.EscalationActionDeescalate: true,
}

// snEscalationCaseRef mirrors the case reference embedded in an escalation record.
type snEscalationCaseRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// snEscalationChoiceItem mirrors a ServiceNow choice-list {id, label} pair, used
// for currentLevel/previousLevel. ID is one of validEscalationLevel's keys
// ("0" through "5").
type snEscalationChoiceItem struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

// snEscalationNotifiedUser mirrors one entry of an escalation's
// notificationSentTo list.
type snEscalationNotifiedUser struct {
	ID       *string `json:"id"`
	UserName string  `json:"userName"`
	Name     *string `json:"name"`
	Email    *string `json:"email"`
}

// snEscalation mirrors one record in the backing service's escalation
// search/create responses.
type snEscalation struct {
	ID                 string                     `json:"id"`
	Case               snEscalationCaseRef        `json:"case"`
	CurrentLevel       snEscalationChoiceItem     `json:"currentLevel"`
	PreviousLevel      snEscalationChoiceItem     `json:"previousLevel"`
	CreatedBy          string                     `json:"createdBy"`
	CreatedOn          string                     `json:"createdOn"`
	UpdatedOn          string                     `json:"updatedOn"`
	Reason             *string                    `json:"reason"`
	NotificationSentTo []snEscalationNotifiedUser `json:"notificationSentTo"`
}

// snEscalationSearchFilters mirrors the POST /escalations/search request body's
// filters object.
type snEscalationSearchFilters struct {
	CaseIDs       []string `json:"caseIds,omitempty"`
	CurrentLevels []int    `json:"currentLevels,omitempty"`
}

// snEscalationSort mirrors the POST /escalations/search request body's sortBy object.
type snEscalationSort struct {
	Field string `json:"field"`
	Order string `json:"order"`
}

// snEscalationSearchPayload is the POST /escalations/search request body.
type snEscalationSearchPayload struct {
	Filters    *snEscalationSearchFilters `json:"filters,omitempty"`
	SortBy     *snEscalationSort          `json:"sortBy,omitempty"`
	Pagination snProjectPagination        `json:"pagination"`
}

// snEscalationSearchResponse is the POST /escalations/search response body.
type snEscalationSearchResponse struct {
	Escalations  []snEscalation `json:"escalations"`
	TotalRecords int            `json:"totalRecords"`
	Offset       int            `json:"offset"`
	Limit        int            `json:"limit"`
}

// snCreateEscalationPayload is the POST /escalations request body.
type snCreateEscalationPayload struct {
	CaseID string  `json:"caseId"`
	Reason *string `json:"reason,omitempty"`
	Action *string `json:"action,omitempty"`
}

// snCreateEscalationResponse is the POST /escalations response body.
type snCreateEscalationResponse struct {
	Message    string       `json:"message"`
	Escalation snEscalation `json:"escalation"`
}

// escalationSearchPageSize is the page size used internally when reading a
// case's escalation history from the backing service. GET /cases/{id}/escalations
// promises the case's FULL history (see domain.SearchEscalationsResponse's doc
// comment), and repeated escalate/de-escalate actions can in principle produce
// more than one page's worth of records, so SearchEscalations below pages
// through every result rather than returning just the first page — a case with
// a long escalation history must not silently lose its oldest records.
const escalationSearchPageSize = 100

func snEscalationToDomain(ctx context.Context, e snEscalation) (domain.Escalation, error) {
	createdOn, err := parseSNDateTime(ctx, "SearchEscalations", "createdOn", e.CreatedOn)
	if err != nil {
		return domain.Escalation{}, fmt.Errorf("sn escalation: parse createdOn: %w", err)
	}
	updatedOn, err := parseSNDateTime(ctx, "SearchEscalations", "updatedOn", e.UpdatedOn)
	if err != nil {
		return domain.Escalation{}, fmt.Errorf("sn escalation: parse updatedOn: %w", err)
	}

	var notified []domain.EscalationNotifiedUser
	for _, u := range e.NotificationSentTo {
		ref := domain.EscalationNotifiedUser{
			UserName: u.UserName,
			Name:     u.Name,
			Email:    u.Email,
		}
		if u.ID != nil && *u.ID != "" {
			id := sysidToUUID(*u.ID)
			ref.ID = &id
		}
		notified = append(notified, ref)
	}

	return domain.Escalation{
		ID:            sysidToUUID(e.ID),
		CaseID:        sysidToUUID(e.Case.ID),
		CurrentLevel:  e.CurrentLevel.ID,
		PreviousLevel: e.PreviousLevel.ID,
		CreatedBy:     e.CreatedBy,
		CreatedOn:     createdOn,
		UpdatedOn:     updatedOn,
		Reason:        e.Reason,
		NotifiedUsers: notified,
	}, nil
}

type snEscalationService struct {
	client  *integrationservice.Client
	caseSvc CaseService
}

// NewServiceNowEscalationService constructs an EscalationService backed by the
// backing service's shared escalation endpoints (already deployed and unchanged
// by this service — see /escalations and /escalations/search). caseSvc is used
// to record a work note on the parent case after a successful escalation
// create: verified live against SN dev data that creating an escalation record
// does not itself produce any case activity/comment entry, so this service adds
// the one the backing API doesn't.
func NewServiceNowEscalationService(client *integrationservice.Client, caseSvc CaseService) EscalationService {
	return &snEscalationService{client: client, caseSvc: caseSvc}
}

// escalationWorkNoteContent builds the case work note text recorded after a
// successful escalation create, e.g. "Case escalated from EL1 to EL2. Reason:
// customer requested management involvement." or "Case de-escalated from EL2 to
// EL1." (no reason line when none was given). Level wording comes straight from
// the backing service's own choice-list labels rather than a hardcoded map, so
// it can't drift from whatever SN's escalation_level choices actually say.
func escalationWorkNoteContent(action domain.EscalationAction, e snEscalation) string {
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

// SearchEscalations implements EscalationService. It pages through every
// upstream result rather than returning only the first page — see
// escalationSearchPageSize's doc comment for why.
func (s *snEscalationService) SearchEscalations(ctx context.Context, caseID string) (domain.SearchEscalationsResponse, error) {
	token := middleware.UserIDTokenFromContext(ctx)

	if err := validateUUIDs("caseId", []string{caseID}); err != nil {
		return domain.SearchEscalationsResponse{}, err
	}

	sysid := uuidToSysid(caseID)
	escalations := make([]domain.Escalation, 0, escalationSearchPageSize)
	total := 0

	for offset := 0; ; offset += escalationSearchPageSize {
		payload := snEscalationSearchPayload{
			Filters: &snEscalationSearchFilters{
				CaseIDs: []string{sysid},
			},
			Pagination: snProjectPagination{Limit: escalationSearchPageSize, Offset: offset},
		}

		raw, err := s.client.Post(ctx, "/escalations/search", token, payload)
		if err != nil {
			return domain.SearchEscalationsResponse{}, err
		}

		var snResp snEscalationSearchResponse
		if err := json.Unmarshal(raw, &snResp); err != nil {
			return domain.SearchEscalationsResponse{}, fmt.Errorf("sn search escalations: parse response: %w", err)
		}

		for _, e := range snResp.Escalations {
			view, err := snEscalationToDomain(ctx, e)
			if err != nil {
				return domain.SearchEscalationsResponse{}, err
			}
			escalations = append(escalations, view)
		}

		total = snResp.TotalRecords
		if len(snResp.Escalations) == 0 || len(escalations) >= total {
			break
		}
	}

	return domain.SearchEscalationsResponse{
		Escalations: escalations,
		Total:       total,
		Offset:      0,
		Limit:       len(escalations),
	}, nil
}

// CreateEscalation implements EscalationService.
func (s *snEscalationService) CreateEscalation(ctx context.Context, caseID string, reason *string, action *domain.EscalationAction) (domain.Escalation, error) {
	token := middleware.UserIDTokenFromContext(ctx)

	if err := validateUUIDs("caseId", []string{caseID}); err != nil {
		return domain.Escalation{}, err
	}

	effectiveAction := domain.EscalationActionEscalate
	if action != nil {
		if !validEscalationAction[*action] {
			return domain.Escalation{}, &apierror.ValidationError{Msg: "action contains invalid value: " + string(*action)}
		}
		effectiveAction = *action
	}

	if effectiveAction == domain.EscalationActionEscalate && (reason == nil || *reason == "") {
		return domain.Escalation{}, &apierror.ValidationError{Msg: "reason is required when escalating"}
	}

	payload := snCreateEscalationPayload{
		CaseID: uuidToSysid(caseID),
		Reason: reason,
	}
	if action != nil {
		actionStr := string(effectiveAction)
		payload.Action = &actionStr
	}

	raw, err := s.client.Post(ctx, "/escalations", token, payload)
	if err != nil {
		return domain.Escalation{}, err
	}

	var snResp snCreateEscalationResponse
	if err := json.Unmarshal(raw, &snResp); err != nil {
		return domain.Escalation{}, fmt.Errorf("sn create escalation: parse response: %w", err)
	}

	view, err := snEscalationToDomain(ctx, snResp.Escalation)
	if err != nil {
		return domain.Escalation{}, err
	}

	// The escalation record itself carries no case activity of its own (verified
	// live against SN dev: creating sn_customerservice_case_escalation rows produces
	// no sys_journal_field entry on the parent case). Record one here so the case's
	// comment/work-note trail reflects the action. The escalation already happened
	// by this point, so a failure here must not fail the request -- log and return
	// the successful escalation instead of telling the caller their escalation failed
	// when it didn't.
	if _, err := s.caseSvc.CreateCaseComment(ctx, domain.CreateCaseCommentRequest{
		CaseID:  caseID,
		Type:    domain.CommentTypeWorkNote,
		Content: escalationWorkNoteContent(effectiveAction, snResp.Escalation),
	}); err != nil {
		slog.ErrorContext(ctx, "sn create escalation: failed to record case work note",
			"caseID", caseID, "escalationID", view.ID, "error", err)
	}

	return view, nil
}
