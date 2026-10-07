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
	"time"

	"github.com/jackc/pgx/v5"
)

// ServiceRequest is what the sr.* events need to know about one service
// request.
type ServiceRequest struct {
	ID                  string
	Number              string
	WSO2CaseID          string
	Subject             string
	Description         string
	State               string // service_request_state_enum label, e.g. OPEN
	ProjectID           string
	ProjectName         string
	SRETeamID           string
	SRETeamName         string
	AssignmentGroupName string
	CreatedBy           string
	CreatedOn           time.Time
	Tags                []string
}

// SRNoticeRepository backs the service-request automation ported from
// ServiceNow's "SR New Request - Acknowledge & Chat Alert" flow.
type SRNoticeRepository interface {
	// GetServiceRequest returns the SR, or ok=false when caseID is not a
	// service request.
	GetServiceRequest(ctx context.Context, caseID string) (sr ServiceRequest, ok bool, err error)
	// AssignToGroup sets the SR's assignment group, as the flow's first
	// action does with the account's SRE team.
	AssignToGroup(ctx context.Context, caseID, groupID, actor string) error
	// Acknowledge posts the acknowledgement comment and moves the SR to
	// OPEN, in one transaction, as the flow's second action does. Written
	// straight to the tables, not through the case service, so no
	// case.comment_added is published: the flow saves with setWorkflow(false),
	// which suppresses ServiceNow's comment notifications the same way.
	Acknowledge(ctx context.Context, caseID, actor, comment string) (commentID string, err error)
}

type srNoticeRepo struct {
	db *Scoped
}

// NewSRNoticeRepository constructs an SRNoticeRepository.
func NewSRNoticeRepository(db *Scoped) SRNoticeRepository {
	return &srNoticeRepo{db: db}
}

// The SR's account is work_item.account_id, falling back to the project's:
// ServiceNow routes on case.account.u_sre_team, and a case's account is its
// project's.
const getServiceRequestQuery = `
	SELECT wi.id::text, wi.number, COALESCE(wi.wso2_id, ''), COALESCE(wi.subject, ''),
	       COALESCE(wi.description, ''), COALESCE(sr.state::text, ''),
	       COALESCE(wi.project_id::text, ''), COALESCE(p.name, ''),
	       COALESCE(a.sre_team_id::text, ''), COALESCE(st.name, ''),
	       COALESCE(ag.name, ''), wi.created_by, wi.created_on
	FROM work_item wi
	JOIN service_request sr ON sr.id = wi.id
	LEFT JOIN project p ON p.id = wi.project_id
	LEFT JOIN account a ON a.id = COALESCE(wi.account_id, p.account_id)
	LEFT JOIN "group" st ON st.id = a.sre_team_id
	LEFT JOIN "group" ag ON ag.id = wi.assignment_group_id
	WHERE wi.id = $1 AND wi.type = 'SERVICE_REQUEST'`

// GetServiceRequest implements SRNoticeRepository.
func (r *srNoticeRepo) GetServiceRequest(ctx context.Context, caseID string) (ServiceRequest, bool, error) {
	var sr ServiceRequest
	err := r.db.QueryRow(ctx, getServiceRequestQuery, caseID).Scan(
		&sr.ID, &sr.Number, &sr.WSO2CaseID, &sr.Subject,
		&sr.Description, &sr.State,
		&sr.ProjectID, &sr.ProjectName,
		&sr.SRETeamID, &sr.SRETeamName,
		&sr.AssignmentGroupName, &sr.CreatedBy, &sr.CreatedOn,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return ServiceRequest{}, false, nil
	}
	if err != nil {
		return ServiceRequest{}, false, fmt.Errorf("get service request: %w", err)
	}

	rows, err := r.db.Query(ctx, `
		SELECT t.name
		FROM work_item_tag wit
		JOIN tag t ON t.id = wit.tag_id
		WHERE wit.work_item_id = $1
		ORDER BY t.name`, caseID)
	if err != nil {
		return ServiceRequest{}, false, fmt.Errorf("get service request tags: %w", err)
	}
	defer rows.Close()
	sr.Tags = []string{}
	for rows.Next() {
		var name *string
		if err := rows.Scan(&name); err != nil {
			return ServiceRequest{}, false, fmt.Errorf("get service request tags: scan: %w", err)
		}
		if name != nil {
			sr.Tags = append(sr.Tags, *name)
		}
	}
	if err := rows.Err(); err != nil {
		return ServiceRequest{}, false, fmt.Errorf("get service request tags: %w", err)
	}
	return sr, true, nil
}

// AssignToGroup implements SRNoticeRepository.
func (r *srNoticeRepo) AssignToGroup(ctx context.Context, caseID, groupID, actor string) error {
	tag, err := r.db.Exec(ctx, `
		UPDATE work_item
		SET assignment_group_id = $2::uuid, updated_on = NOW(), updated_by = $3
		WHERE id = $1 AND type = 'SERVICE_REQUEST'`, caseID, groupID, actor)
	if err != nil {
		return fmt.Errorf("assign service request: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("assign service request: %s not found", caseID)
	}
	return nil
}

// Acknowledge implements SRNoticeRepository.
//
// The state write is skipped when the SR is already OPEN -- which a natively
// created SR always is, service_request_state_enum having no NEW -- so the
// AFTER UPDATE OF state trigger that pushes state changes to a linked GitHub
// issue (migration 0114) does not fire for a change that did not happen.
func (r *srNoticeRepo) Acknowledge(ctx context.Context, caseID, actor, comment string) (string, error) {
	return InTxReturning(ctx, r.db, func(tx pgx.Tx) (string, error) {
		var commentID string
		if err := tx.QueryRow(ctx, `
			INSERT INTO comment (id, created_on, created_by, type, work_item_id, content)
			VALUES (gen_random_uuid(), NOW(), $2, 'COMMENT'::comment_type_enum, $1, $3)
			RETURNING id::text`, caseID, actor, comment).Scan(&commentID); err != nil {
			return "", fmt.Errorf("acknowledge service request: comment: %w", err)
		}
		if _, err := tx.Exec(ctx, `
			UPDATE service_request SET state = 'OPEN'::service_request_state_enum
			WHERE id = $1 AND state IS DISTINCT FROM 'OPEN'::service_request_state_enum`, caseID); err != nil {
			return "", fmt.Errorf("acknowledge service request: state: %w", err)
		}
		return commentID, nil
	})
}
