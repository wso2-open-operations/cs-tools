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

package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// This file owns the approved-content lock: once a change request has been
// approved (CAB approval received -- or, for a Standard change, which is
// pre-approved and has no CAB stage, Request Approval having scheduled it), what
// was approved is what gets implemented, so none of its fields can be changed any
// more. Work notes (and the customer-visible comment) can still be added, the
// deviation from the implementation plan can still be recorded from Implement on
// (below), and the lifecycle still moves: state changes, on hold, a Re-schedule
// out of Customer Approval (the customer is asked about the new time) and WSO2's
// answer to a time the customer proposed are not edits of the approved content.
//
// Like the customer requirements lock it is a pure function of the stored state
// (no marker, no column): the states below are only ever reached through an
// approval (a manual Scheduled / Customer Approval is refused from every state),
// and a change request never returns to an earlier one. A request is judged by
// what it CHANGES: a field resent with the value it already holds is the accepted
// no-op it is everywhere in this PATCH, so a client that sends the whole form back
// is not punished. Canceled is not in the set: a change can be cancelled before
// anybody approved it.
//
// Fields this lock does not list are covered by a rule of their own that is at
// least as strict: the Customer Project and the type are frozen from Request
// Approval on (change_request_customer_lock.go), and the deployments and
// deployment products by changeRequestLinksLockedStates, which holds these states.
// The one exception, on purpose, is the deviation from the implementation plan: it
// records what the implementation did, not what was approved, so it is outside
// this lock and has a rule of its own (change_request_deviation.go): recordable in
// implement, review and customer_review, and final once the change is closed,
// canceled or rolled back.

// changeRequestApprovedStates are the (upper-case) states a change request is in
// only once it has been approved to proceed.
var changeRequestApprovedStates = map[string]bool{
	crStateCustomerApproval: true,
	crStateScheduled:        true,
	crStateImplement:        true,
	crStateReview:           true,
	crStateCustomerReview:   true,
	crStateClosed:           true,
	crStateRollback:         true,
}

// changeRequestContentApproved reports whether the (upper-case) stored state puts
// the change request's fields under the approved-content lock.
func changeRequestContentApproved(state string) bool {
	return changeRequestApprovedStates[state]
}

// approvedContentLockedMsg is the 400 for a PATCH that changes the named fields of
// an approved change request. It says what is still possible: work notes, and,
// from Implement on until the change is closed, canceled or rolled back, the
// deviation from the implementation plan (change_request_deviation.go): in
// implement, review and customer_review. In customer_approval and scheduled
// implementation has not started yet, and closed and rollback are final, so there
// only work notes can be added.
func approvedContentLockedMsg(fields []string, state string) string {
	still := "only work notes can be added"
	if deviationRecordableState(state) {
		still = "only work notes and the deviation from the implementation plan can be added"
	}
	return fmt.Sprintf("%s can no longer be changed: the change request has been approved (current state: %s), so its fields are read-only; %s",
		strings.Join(fields, ", "), lockStateName(state), still)
}

// approvedLockField is one field of a PATCH the lock judges: the API name, a SQL
// predicate (over work_item wi / change_request cr) that is true when the
// requested value differs from the stored one, and the value bound to it ($%d in
// the predicate). An explicit null (a **T field set to null) binds nothing.
type approvedLockField struct {
	name      string
	predicate string
	value     any
	bind      bool
}

// approvedLockFields lists the fields of req the lock judges, in the order the
// message names them. The planned window is left out of a staff time response
// (state "authorize": a Re-schedule or WSO2's answer to a proposed time), which is
// how the window of an approved change is moved -- with the customer asked again.
func approvedLockFields(req domain.PatchChangeRequestRequest, staffTimeResponse bool) []approvedLockField {
	var fields []approvedLockField
	text := func(name, stored string, v *string) {
		if v != nil {
			fields = append(fields, approvedLockField{name: name, predicate: stored + " IS DISTINCT FROM $%d::text", value: *v, bind: true})
		}
	}
	nullableText := func(name, stored string, v **string) {
		if v == nil {
			return
		}
		if *v == nil {
			fields = append(fields, approvedLockField{name: name, predicate: stored + " IS NOT NULL"})
			return
		}
		text(name, stored, *v)
	}
	// Ids are compared as text, so a malformed one never reaches a uuid cast here
	// (the service validated them; the write casts them).
	id := func(name, stored string, v *string) {
		if v != nil {
			fields = append(fields, approvedLockField{name: name, predicate: "LOWER(" + stored + "::text) IS DISTINCT FROM LOWER(BTRIM($%d::text))", value: *v, bind: true})
		}
	}
	enumLabel := func(name, stored string, v *string) {
		if v != nil {
			fields = append(fields, approvedLockField{name: name, predicate: stored + "::text IS DISTINCT FROM UPPER(BTRIM($%d::text))", value: *v, bind: true})
		}
	}
	boolean := func(name, stored string, v *bool) {
		if v != nil {
			fields = append(fields, approvedLockField{name: name, predicate: "COALESCE(" + stored + ", FALSE) IS DISTINCT FROM $%d::boolean", value: *v, bind: true})
		}
	}

	text("title", "wi.subject", req.Title)
	text("description", "wi.description", req.Description)
	text("justification", "cr.justification", req.Justification)
	// The detail reads these two through the column the create of an older build
	// wrote, so the stored value is compared the same way.
	text("impactDescription", "COALESCE(cr.impact_description, cr.risk_impact_analysis)", req.ImpactDescription)
	nullableText("implementationPlan", "cr.implementation_plan", req.ImplementationPlan)
	text("rollbackPlan", "COALESCE(cr.rollback_process, cr.backout_plan)", req.RollbackPlan)
	text("testPlan", "cr.test_plan", req.TestPlan)
	text("serviceOutage", "cr.service_outage_downtime", req.ServiceOutage)
	text("communicationPlan", "cr.communication_plan", req.CommunicationPlan)
	nullableText("affectedServicesText", "cr.affected_services", req.AffectedServicesText)
	nullableText("affectedComponentsText", "cr.affected_component", req.AffectedComponentsText)
	nullableText("rollbackDurationText", "cr.rollback_duration", req.RollbackDurationText)
	if req.Impact != nil {
		v := string(*req.Impact)
		enumLabel("impact", "cr.impact", &v)
	}
	if req.Priority != nil {
		if *req.Priority == nil {
			fields = append(fields, approvedLockField{name: "priority", predicate: "cr.priority IS NOT NULL"})
		} else {
			v := string(**req.Priority)
			enumLabel("priority", "cr.priority", &v)
		}
	}
	if req.Category != nil {
		if *req.Category == nil {
			fields = append(fields, approvedLockField{name: "category", predicate: "cr.category IS NOT NULL"})
		} else {
			v := string(**req.Category)
			enumLabel("category", "cr.category", &v)
		}
	}
	if !staffTimeResponse {
		if req.PlannedStartOn != nil {
			// The window was parsed and re-written as RFC 3339 UTC before this runs
			// (normalizePatchPlannedWindow), so the cast cannot fail.
			fields = append(fields, approvedLockField{name: "plannedStartOn", predicate: "cr.start_on IS DISTINCT FROM $%d::text::timestamptz", value: *req.PlannedStartOn, bind: true})
		}
		if req.PlannedEndOn != nil {
			fields = append(fields, approvedLockField{name: "plannedEndOn", predicate: "cr.end_on IS DISTINCT FROM $%d::text::timestamptz", value: *req.PlannedEndOn, bind: true})
		}
	}
	id("assignedTeamId", "wi.assignment_group_id", req.AssignedTeamID)
	id("assignedEngineerId", "wi.assigned_to_id", req.AssignedEngineerID)
	if req.RequestedByID != nil {
		if *req.RequestedByID == nil {
			fields = append(fields, approvedLockField{name: "requestedById", predicate: "cr.requested_by_user_id IS NOT NULL"})
		} else {
			id("requestedById", "cr.requested_by_user_id", *req.RequestedByID)
		}
	}
	id("caseId", "wi.parent_id", req.CaseID)
	id("serviceId", "cr.service_id", req.ServiceID)
	id("serviceOfferingId", "cr.service_offering_id", req.ServiceOfferingID)
	boolean("isPlanningVisibleToCustomers", "cr.is_planning_visible_to_customers", req.IsPlanningVisibleToCustomers)
	boolean("customerApprovalRequired", "cr.customer_approval_required", req.CustomerApprovalRequired)
	boolean("customerReviewRequired", "cr.customer_review_required", req.CustomerReviewRequired)
	return fields
}

// changeRequestPatchTouchesApprovedContent reports whether req names a field the
// approved-content lock judges, so patchChangeRequestTx takes the row lock and
// reads the state for it.
func changeRequestPatchTouchesApprovedContent(req domain.PatchChangeRequestRequest) bool {
	return len(approvedLockFields(req, false)) > 0
}

// approvedContentQuery builds the one statement that names, among fields, those
// whose requested value differs from the stored one ($1 is the change request id).
func approvedContentQuery(fields []approvedLockField) (string, []any) {
	args := []any{}
	cases := make([]string, 0, len(fields))
	for _, f := range fields {
		predicate := f.predicate
		if f.bind {
			args = append(args, f.value)
			predicate = fmt.Sprintf(predicate, len(args)+1)
		}
		cases = append(cases, fmt.Sprintf("CASE WHEN %s THEN '%s' END", predicate, f.name))
	}
	query := `SELECT ARRAY_REMOVE(ARRAY[` + strings.Join(cases, ", ") + `]::text[], NULL)
		FROM work_item wi JOIN change_request cr ON cr.id = wi.id
		WHERE wi.id = $1::uuid`
	return query, append([]any{nil}, args...)
}

// checkApprovedContentLock refuses (400, CodeChangeRequestApprovedReadOnly) a PATCH
// that changes any field of a change request that has been approved, naming every
// field it would change. snap is the state read under the row lock; nothing is
// queried while the change has not been approved, or when the request names no
// field the lock judges.
func checkApprovedContentLock(ctx context.Context, tx pgx.Tx, id string, snap changeRequestGateSnapshot, req domain.PatchChangeRequestRequest, staffTimeResponse bool) error {
	if !changeRequestContentApproved(snap.state) {
		return nil
	}
	fields := approvedLockFields(req, staffTimeResponse)
	if len(fields) == 0 {
		return nil
	}
	query, args := approvedContentQuery(fields)
	args[0] = id
	var changed []string
	if err := tx.QueryRow(ctx, query, args...).Scan(&changed); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return &apierror.NotFoundError{Msg: "change request not found"}
		}
		return fmt.Errorf("patch change request: compare approved fields: %w", err)
	}
	if len(changed) == 0 {
		return nil
	}
	return &apierror.ValidationError{Msg: approvedContentLockedMsg(changed, snap.state), Code: apierror.CodeChangeRequestApprovedReadOnly}
}
