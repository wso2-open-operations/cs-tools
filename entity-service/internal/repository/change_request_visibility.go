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
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
)

// This file is the ONE place the question "may this caller see this change
// request" is answered for a customer. Everything customer-reachable (the list,
// the counts, the detail, the approvals, a decision, a PATCH, the comments, the
// stats, the notice audience) asks it through the SQL fragment below or through
// requireVisibleChangeRequest, and TestChangeRequestVisibilityLint pins that no
// repository function touching a change request can skip both.
//
// THE RULE. A customer (an identity that is not Unrestricted) sees a change
// request only while ALL of these hold:
//
//  1. they are a REGISTERED contact of the change request's CURRENT project (the
//     membership row-level security and the project-scoped list already require:
//     moving a change request to another project takes it away from the old
//     project's contacts);
//  2. the change request was DESIGNATED to them: the customer's approval or
//     review was asked of them, i.e. they hold (or ever held, in any state
//     including Approved / Rejected / Cancelled) an approval_stage_approver row
//     on a "Customer Approval" / "Customer Review" stage of it. Designation is
//     per person and permanent: a contact who registers AFTER the stage was
//     provisioned was never asked and does not see it, a sibling whose row a
//     colleague's answer Cancelled still does, and nothing in this codebase
//     deletes an approver row;
//     -- OR --
//  3. it is a LEGACY change request: one created before the strict-visibility
//     cutover instant (CRVisibility.StrictFrom, env CR_STRICT_VISIBILITY_FROM;
//     unset = every change request is legacy), which keeps being visible to its
//     project's registered contacts in exactly the states customers saw before
//     this rule existed: everything past Authorize (Scheduled, Customer
//     Approval, Implement, Review, Customer Review, Rollback, Closed, Canceled)
//     and never New / Assess / Authorize or a NULL state.
//
// WHY THE CUTOVER INSTANT. Change requests migrated or synced from ServiceNow
// were never asked through our flow: they carry no Customer Approval / Customer
// Review stage rows, so "designated" would hide every one of them from the
// customers who see them today. No existing column says whether a row came from
// ServiceNow or from this service (and none may be added), so the one
// deterministic marker left is time: work_item.created_on against a configured
// instant. The failure modes are documented in CLAUDE.md ("Customer visibility
// and the cutover").
//
// WHERE IT IS ENFORCED. In Go SQL, not in a row-level-security policy: the legacy
// test needs the change request's state, its creation time and a configured
// instant, and a policy would need a new session setting on every statement and
// ACCESS EXCLUSIVE on work_item to create. The fragment is self-contained: it
// reads the caller's email from the Go context identity (never from the
// app.viewer_email setting, which six mid-transaction identity escalations blank)
// and checks project membership itself, so it holds identically for a superuser
// and a restricted database role. Row-level security stays what it was, a second,
// coarser line (project membership) underneath.

// CRVisibility is the customer-visibility policy of change requests. The zero
// value (StrictFrom nil) is the safe default and the rollback: every change
// request is legacy, so customers see exactly what they saw before the strict
// rule existed, plus the change requests designated to them.
type CRVisibility struct {
	// StrictFrom is the cutover instant: change requests created at or after it
	// are visible to a customer only when designated to them; earlier ones are
	// legacy. nil means no cutover (every change request is legacy).
	StrictFrom *time.Time
}

// crLegacyVisibleStates are the change_request_state_enum labels a LEGACY
// change request is visible to its project's registered contacts in: what the
// customer portal has always shown (everything but New / Assess / Authorize).
// A NULL state is not in the list and is therefore hidden.
var crLegacyVisibleStates = []string{
	crStateScheduled, crStateCustomerApproval, crStateImplement, crStateReview,
	crStateCustomerReview, crStateRollback, crStateClosed, crStateCanceled,
}

// crVisibilityCustomerStageLabels are the approval_stage.checkpoint_label values
// of the two customer stages: the only stages designation is read from.
// Pinned to the labels provisionCustomerStage writes by a test.
var crVisibilityCustomerStageLabels = []string{approvalStageLabelCustomerApproval, approvalStageLabelCustomerReview}

// crDesignationApproverStates are the approval_stage_approver.state values that
// mean "this person was asked". NOT_REQUESTED / NOT_REQUIRED / NOT_ENTITLED are
// the ServiceNow sync's vocabulary for "was not asked" and do not designate.
var crDesignationApproverStates = []string{"REQUESTED", "APPROVED", "REJECTED", "CANCELLED"}

// firstCRVisibility is the optional CRVisibility argument of the repository
// constructors: none given is the zero value, no cutover.
func firstCRVisibility(v []CRVisibility) CRVisibility {
	if len(v) == 0 {
		return CRVisibility{}
	}
	return v[0]
}

// sqlStringList renders values as a SQL list of string literals. The values are
// code constants (state and label names), never request input.
func sqlStringList(values []string) string {
	quoted := make([]string, len(values))
	for i, v := range values {
		quoted[i] = "'" + strings.ReplaceAll(v, "'", "''") + "'"
	}
	return strings.Join(quoted, ", ")
}

// crViewer reports whether ctx belongs to a caller the visibility rule restricts
// (anyone who is not Unrestricted, and a context with no identity at all, which
// Scoped refuses anyway) and the viewer's email, lower-cased and trimmed.
func crViewer(ctx context.Context) (restricted bool, email string) {
	scope, ok := CallerIdentityFromContext(ctx)
	if !ok {
		return true, ""
	}
	return !scope.Unrestricted, strings.ToLower(strings.TrimSpace(scope.ViewerEmail))
}

// clause returns the visibility predicate for a restricted caller as a SQL
// boolean expression (to be AND-ed into a WHERE) with the two arguments it binds
// from nextArg on, or ("", nil) for an Unrestricted caller, who is not narrowed.
// wi and cr are the aliases of work_item and change_request in the query.
//
//	$nextArg    the viewer's email (text)
//	$nextArg+1  the cutover instant (timestamptz, NULL when unset)
//
// A restricted caller with no email matches nothing (the clause is FALSE): fail
// closed. Every subquery is uncorrelated, so Postgres evaluates each once per
// statement (a hashed subplan) instead of once per change request row.
func (v CRVisibility) clause(ctx context.Context, wi, cr string, nextArg int) (string, []any) {
	restricted, email := crViewer(ctx)
	if !restricted {
		return "", nil
	}
	if email == "" {
		return "FALSE", nil
	}
	em := fmt.Sprintf("$%d::text", nextArg)
	cut := fmt.Sprintf("$%d::timestamptz", nextArg+1)
	return fmt.Sprintf(`(
		%[1]s.project_id IN (SELECT pc.project_id FROM project_contact pc
		                      WHERE LOWER(pc.email) = LOWER(%[3]s) AND pc.state = 'REGISTERED')
		AND (
		  %[1]s.id IN (SELECT ast.work_item_id
		                 FROM approval_stage_approver asa
		                 JOIN approval_stage ast ON ast.id = asa.stage_id
		                WHERE ast.checkpoint_label IN (%[5]s)
		                  AND asa.state IN (%[6]s)
		                  AND asa.approver_user_id IN (SELECT u.id FROM "user" u WHERE LOWER(u.email) = LOWER(%[3]s)))
		  OR (%[2]s.state::text IN (%[7]s) AND (%[4]s IS NULL OR %[1]s.created_on < %[4]s))
		)
	)`, wi, cr, em, cut,
			sqlStringList(crVisibilityCustomerStageLabels),
			sqlStringList(crDesignationApproverStates),
			sqlStringList(crLegacyVisibleStates)),
		[]any{email, v.StrictFrom}
}

// andClause is clause with the " AND " glued on, "" for an Unrestricted caller.
func (v CRVisibility) andClause(ctx context.Context, wi, cr string, args []any) (string, []any) {
	sql, extra := v.clause(ctx, wi, cr, len(args)+1)
	if sql == "" {
		return "", args
	}
	return " AND " + sql, append(args, extra...)
}

// isLegacy reports whether a change request created at createdOn is legacy.
func (v CRVisibility) isLegacy(createdOn time.Time) bool {
	return v.StrictFrom == nil || createdOn.Before(*v.StrictFrom)
}

// requireVisibleChangeRequest refuses (404 "change request not found") a
// restricted caller who may not see the change request id, and does nothing for
// an Unrestricted caller. It answers the same question as the list and detail
// fragment, from the same SQL, so "not visible" is one thing everywhere: absent
// from the list AND 404 on every read and write by id. It is the FIRST statement
// of every by-id operation that reaches a customer (the approvals read, a
// decision, a PATCH), before anything is looked at or locked, and runs before
// any in-transaction identity escalation (it does not depend on the session
// identity anyway: it binds the caller's email from the Go context).
//
// A change request that does not exist and one the caller may not see are the
// same answer, so an id cannot be probed.
func (v CRVisibility) requireVisibleChangeRequest(ctx context.Context, q crQuerier, id string) error {
	frag, args := v.clause(ctx, "wi", "cr", 2)
	if frag == "" {
		return nil
	}
	var visible bool
	err := q.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM work_item wi JOIN change_request cr ON cr.id = wi.id
		                 WHERE wi.id = $1::uuid AND wi.type = 'CHANGE_REQUEST' AND `+frag+`)`,
		append([]any{id}, args...)...).Scan(&visible)
	if err != nil {
		return fmt.Errorf("check change request visibility: %w", err)
	}
	if !visible {
		return &apierror.NotFoundError{Msg: "change request not found"}
	}
	return nil
}

// rejectHiddenChangeRequest is the guard for operations that take a "case" id but
// run against any work item the id names, so a change request's id can be passed
// in the case's place (comments, tags, the watch list): for a restricted caller
// it refuses (404 notFound) an id that names a change request the caller may not
// see. Any other work item -- a case, or an id that exists nowhere -- passes
// untouched and is left to the operation's own checks and to row-level security.
// A no-op for an Unrestricted caller.
func (v CRVisibility) rejectHiddenChangeRequest(ctx context.Context, q crQuerier, id, notFound string) error {
	frag, args := v.clause(ctx, "wi", "cr", 2)
	if frag == "" {
		return nil
	}
	var hidden bool
	err := q.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM work_item wi JOIN change_request cr ON cr.id = wi.id
		                 WHERE wi.id = $1::uuid AND wi.type = 'CHANGE_REQUEST' AND NOT `+frag+`)`,
		append([]any{id}, args...)...).Scan(&hidden)
	if err != nil {
		return fmt.Errorf("check change request visibility: %w", err)
	}
	if hidden {
		return &apierror.NotFoundError{Msg: notFound}
	}
	return nil
}

// legacyAndState reads a change request's (upper-case) state, project and whether
// it is legacy under v, for the lazy-provisioning checks. ok is false when the
// change request does not exist.
func (v CRVisibility) legacyAndState(ctx context.Context, q crQuerier, id string) (legacy bool, state string, projectID *string, ok bool, err error) {
	var st *string
	var createdOn time.Time
	row := q.QueryRow(ctx,
		`SELECT cr.state::text, wi.created_on, wi.project_id::text
		   FROM change_request cr JOIN work_item wi ON wi.id = cr.id WHERE cr.id = $1::uuid`, id)
	if err := row.Scan(&st, &createdOn, &projectID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, "", nil, false, nil
		}
		return false, "", nil, false, fmt.Errorf("read change request for legacy check: %w", err)
	}
	return v.isLegacy(createdOn), strings.ToUpper(stringOrEmpty(st)), projectID, true, nil
}

// crVisibilityContextKey is the context key the repository's visibility policy
// travels under, from the exported repository method that owns it down to the
// helpers that need it (the lazy provisioning of a legacy change request's
// customer stage, the customerCanAnswer read), so the deep call chain of a PATCH
// does not have to thread one more parameter through every signature. Only the
// repository methods stamp it (withCRVisibility); a context that carries none
// reads back as the zero policy.
type crVisibilityContextKey struct{}

func withCRVisibility(ctx context.Context, v CRVisibility) context.Context {
	return context.WithValue(ctx, crVisibilityContextKey{}, v)
}

func crVisibilityFromContext(ctx context.Context) CRVisibility {
	v, _ := ctx.Value(crVisibilityContextKey{}).(CRVisibility)
	return v
}
