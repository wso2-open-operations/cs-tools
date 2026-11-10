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
	"fmt"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// What the CAB said about change requests, for the standard change suggestions
// and the proposal page (the CAB history an approver sees): the CAB's rejections
// and every comment a CAB approver wrote. A CAB decision is an
// approval_stage_approver row on a change request's CAB stage, and which stage
// is the CAB's is decided here, in Go, by the same functions the change request
// approvals view uses (isCABStageInHistory), never by a second copy of them in SQL.

// CABDecisionReader reads the CAB decisions of change requests. Internal callers only.
type CABDecisionReader interface {
	// CABDecisionsOf maps each change request id given (that exists) to the
	// change request and its CAB decisions that matter: rejections and decisions
	// with a comment, newest first. A change request with none is still in the
	// map, with no decisions (its subject is what a look-alike is matched by).
	CABDecisionsOf(ctx context.Context, changeRequestIDs []string) (map[string]CABChangeRequest, error)
	// CABRejectedLookalikes are one customer's change requests that the CAB
	// rejected since the instant given and that are not suggestion candidates:
	// in any state but Closed and Rollback, and not raised from a template. Each
	// carries its implementation plan and its CAB decisions as CABDecisionsOf
	// reads them; newest rejection first, at most cabLookalikeLimit.
	CABRejectedLookalikes(ctx context.Context, accountID string, since time.Time) ([]CABChangeRequest, error)
}

// CABChangeRequest is a change request with its CAB decisions (see CABDecisionReader).
type CABChangeRequest struct {
	ID      string
	Number  string
	Subject string
	// State is the change request's state, lower case ("canceled", "authorize", ...).
	State string
	// ImplementationPlan is read for a look-alike only (it picks among clusters).
	ImplementationPlan string
	// Decisions on its CAB stages that are REJECTED or carry a comment, newest first.
	Decisions []CABDecision
}

// CABDecision is one CAB approver's row on a CAB stage.
type CABDecision struct {
	// State is the row's state: APPROVED, REJECTED, REQUESTED, CANCELLED, ...
	State string
	// Comment as stored: plain text, or rich text copied from ServiceNow.
	Comment      string
	DecidedOn    time.Time
	ApproverName string
}

// Rejected reports whether the CAB rejected the change request at least once.
func (c CABChangeRequest) Rejected() bool {
	for _, d := range c.Decisions {
		if d.State == "REJECTED" {
			return true
		}
	}
	return false
}

// cabLookalikeLimit caps the look-alikes read for one customer (newest rejections first).
const cabLookalikeLimit = 1000

// isCABStageInHistory reports whether a change request's approval stage is the
// CAB's, judged after the fact (the change may be long finished). It is the stage
// the change request approvals view names as the CAB's (changeRequestApprovalStageLabel,
// classified by classifyApprovalStage: "CAB Approval", the legacy "Authorize", the
// historic "ECAB Approval", or an unlabelled stage at position 1), and also an
// unlabelled stage of the "CAB Approval" group wherever it sits, the first and
// only provable rule of runtimeApprovalStageKind. The rest of runtimeApprovalStageKind
// is not used: it judges a stage by the state the change is in NOW, which for a
// finished change says nothing about the stage.
func isCABStageInHistory(label *string, position int, groupName *string, model string) bool {
	shown, _ := changeRequestApprovalStageLabel(changeRequestApprovalStageRow{checkpointLabel: label, assignmentGroupName: groupName}, position, model)
	if classifyApprovalStage(&shown, position) == stageKindCAB {
		return true
	}
	return (label == nil || *label == "") && strings.TrimSpace(stringOrEmpty(groupName)) == domain.CABApprovalGroupName
}

// cabDecisionRowsQuery reads the change requests given, each with every approval
// stage (its label, group and zero-based position, ordered as the approvals view
// orders them) and the approver rows of the stage that are REJECTED or have a
// comment; $2 says whether to read the implementation plan. A change request with
// no stage, or no such row, comes back once with NULLs.
const cabDecisionRowsQuery = `
	SELECT cr.id::text, COALESCE(wi.number, ''), COALESCE(wi.subject, ''), LOWER(COALESCE(cr.state::text, '')),
	       COALESCE(cr.change_model::text, ''),
	       CASE WHEN $2 THEN COALESCE(cr.implementation_plan, '') ELSE '' END,
	       s.checkpoint_label, g.name, COALESCE(s.pos, 0)::int,
	       asa.state::text, COALESCE(asa.comments, ''), asa.updated_on,
	       COALESCE(NULLIF(TRIM(u.name), ''), NULLIF(TRIM(CONCAT_WS(' ', u.first_name, u.last_name)), ''), '')
	  FROM change_request cr
	  JOIN work_item wi ON wi.id = cr.id
	  LEFT JOIN LATERAL (
	        SELECT ast.id, ast.checkpoint_label, ast.assignment_group_id,
	               ROW_NUMBER() OVER (ORDER BY ast.created_on, ast.id) - 1 AS pos
	          FROM approval_stage ast
	         WHERE ast.work_item_id = cr.id) s ON TRUE
	  LEFT JOIN "group" g ON g.id = s.assignment_group_id
	  LEFT JOIN approval_stage_approver asa ON asa.stage_id = s.id
	        AND (asa.state = 'REJECTED' OR btrim(COALESCE(asa.comments, '')) <> '')
	  LEFT JOIN "user" u ON u.id = asa.approver_user_id
	 WHERE cr.id = ANY($1::uuid[])
	 ORDER BY cr.id, asa.updated_on DESC NULLS LAST, asa.id`

// cabDecisions runs cabDecisionRowsQuery and keeps the rows of CAB stages.
func (r *standardChangeSuggestionRepo) cabDecisions(ctx context.Context, ids []string, withPlan bool) (map[string]CABChangeRequest, error) {
	out := map[string]CABChangeRequest{}
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := r.db.Query(ctx, cabDecisionRowsQuery, ids, withPlan)
	if err != nil {
		return nil, fmt.Errorf("read CAB decisions: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var c CABChangeRequest
		var model, comment, approver string
		var label, group, state *string
		var pos int
		var decidedOn *time.Time
		if err := rows.Scan(&c.ID, &c.Number, &c.Subject, &c.State, &model, &c.ImplementationPlan,
			&label, &group, &pos, &state, &comment, &decidedOn, &approver); err != nil {
			return nil, fmt.Errorf("read CAB decisions: %w", err)
		}
		cur, seen := out[c.ID]
		if !seen {
			cur = CABChangeRequest{ID: c.ID, Number: c.Number, Subject: c.Subject, State: c.State, ImplementationPlan: c.ImplementationPlan}
		}
		if state != nil && decidedOn != nil && isCABStageInHistory(label, pos, group, model) {
			cur.Decisions = append(cur.Decisions, CABDecision{State: *state, Comment: comment, DecidedOn: *decidedOn, ApproverName: approver})
		}
		out[c.ID] = cur
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read CAB decisions: %w", err)
	}
	return out, nil
}

// CABDecisionsOf implements CABDecisionReader.
//
// crvis: internal callers only: the suggestion refresh (serviceOnly) and the standard change proposal page, both of which require an internal caller
func (r *standardChangeSuggestionRepo) CABDecisionsOf(ctx context.Context, ids []string) (map[string]CABChangeRequest, error) {
	return r.cabDecisions(ctx, ids, false)
}

// CABRejectedLookalikes implements CABDecisionReader.
//
// crvis: internal callers only: the suggestion refresh (serviceOnly) and the standard change proposal page, both of which require an internal caller
func (r *standardChangeSuggestionRepo) CABRejectedLookalikes(ctx context.Context, accountID string, since time.Time) ([]CABChangeRequest, error) {
	// The candidates first (any rejected row since the instant), then their
	// stages, so the CAB stage is told apart in Go.
	rows, err := r.db.Query(ctx, `
		SELECT cr.id::text
		  FROM approval_stage_approver asa
		  JOIN change_request cr ON cr.id = asa.work_item_id
		  JOIN work_item wi ON wi.id = cr.id AND wi.type = 'CHANGE_REQUEST'
		  JOIN project p ON p.id = wi.project_id
		 WHERE p.account_id = $1::uuid
		   AND asa.state = 'REJECTED' AND asa.updated_on >= $2
		   AND (cr.state IS NULL OR cr.state NOT IN ('CLOSED', 'ROLLBACK'))
		   AND cr.standard_change_template_version_id IS NULL
		 GROUP BY cr.id
		 ORDER BY MAX(asa.updated_on) DESC, cr.id
		 LIMIT $3`, accountID, since, cabLookalikeLimit)
	if err != nil {
		return nil, fmt.Errorf("read CAB-rejected change requests: %w", err)
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, fmt.Errorf("read CAB-rejected change requests: %w", err)
		}
		ids = append(ids, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read CAB-rejected change requests: %w", err)
	}
	byID, err := r.cabDecisions(ctx, ids, true)
	if err != nil {
		return nil, err
	}
	var out []CABChangeRequest
	for _, id := range ids {
		c, ok := byID[id]
		if !ok {
			continue
		}
		// Rejected on a CAB stage since the instant: a rejection on another stage
		// (a peer's, the customer's) does not count.
		for _, d := range c.Decisions {
			if d.State == "REJECTED" && !d.DecidedOn.Before(since) {
				out = append(out, c)
				break
			}
		}
	}
	return out, nil
}
