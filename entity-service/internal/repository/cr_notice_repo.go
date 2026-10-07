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
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// OutboxChange is one row change observed through event_outbox.
type OutboxChange struct {
	// ID is the outbox row's own id, the handle used to mark it published.
	ID         int64
	EntityType string
	EntityID   string
	// Changes is {"column": {"from": …, "to": …}} for the columns that differ.
	Changes map[string]map[string]any
	// Snapshot is the row as it now stands.
	Snapshot map[string]any
}

// CRNoticeDetails is the surrounding detail a change-request notice needs and
// the outbox row does not carry.
type CRNoticeDetails struct {
	Number           string
	GitReference     string
	RequesterName    string
	ProjectID        string
	ProjectName      string
	ShortDescription string
	Description      string
	ActorName        string
	ActorIsWSO2      bool
}

// CRNoticeRepository reads what the change-request notices need: the outbox
// rows that trigger them, and the audiences they are addressed to.
//
// Reads only, deliberately. Deciding who to tell must never be able to change
// the record it is reporting on.
type CRNoticeRepository interface {
	ClaimChanges(ctx context.Context, entityTypes []string, limit int) ([]OutboxChange, error)
	Details(ctx context.Context, id string) (CRNoticeDetails, error)
	GroupMemberEmails(ctx context.Context, teamName string) ([]string, error)
	ProjectContactEmails(ctx context.Context, projectID string) ([]string, error)
	// CustomerNoticeEmails is the customer audience of one change request's
	// notices: the contacts the change request is visible to (see
	// change_request_visibility.go), not every contact of the project.
	CustomerNoticeEmails(ctx context.Context, changeRequestID, projectID string) ([]string, error)
}

type crNoticeRepository struct {
	db  *Scoped
	vis CRVisibility
}

// NewCRNoticeRepository constructs the change-request notice reader. The
// optional CRVisibility is the change request customer-visibility policy, which
// decides who of a project's contacts a change request's customer notices go to.
func NewCRNoticeRepository(db *Scoped, vis ...CRVisibility) CRNoticeRepository {
	return &crNoticeRepository{db: db, vis: firstCRVisibility(vis)}
}

// ClaimChanges takes up to limit unpublished outbox rows for the given entity
// types, oldest first, and marks them published in the same statement.
//
// FOR UPDATE SKIP LOCKED is what makes more than one replica safe: two
// drainers racing for the same batch each get a disjoint set instead of
// blocking or double-delivering.
//
// Marking published at claim time rather than after the notice is published is
// deliberate. A crash between the two loses that notice, which is acceptable
// for a notification; the alternative — holding the row open until the publish
// acknowledges — costs a second commit on the hot path and risks re-sending
// the same mail after a restart, which is the worse failure for a human
// recipient.
//
// crvis: background notice drainer under the system identity; reads the outbox, no customer reads through it
func (r *crNoticeRepository) ClaimChanges(ctx context.Context, entityTypes []string, limit int) ([]OutboxChange, error) {
	// WithSystemIdentity: every Scoped call requires SOME identity on ctx
	// regardless of whether the target table has RLS (event_outbox does
	// not) -- this repository is the CR-notice drainer's own background
	// process, with no customer viewer to speak for, same reasoning as
	// Details' own stamp below.
	ctx = WithSystemIdentity(ctx)
	const query = `
		WITH claimed AS (
			SELECT id FROM event_outbox
			WHERE published_on IS NULL
			  AND entity_type = ANY($1::text[])
			ORDER BY id
			LIMIT $2
			FOR UPDATE SKIP LOCKED
		)
		UPDATE event_outbox o
		SET published_on = NOW()
		FROM claimed c
		WHERE o.id = c.id
		RETURNING o.id, o.entity_type, o.entity_id, o.changes, o.snapshot`

	rows, err := r.db.Query(ctx, query, entityTypes, limit)
	if err != nil {
		return nil, fmt.Errorf("crnotice: claim changes: %w", err)
	}
	defer rows.Close()

	var out []OutboxChange
	for rows.Next() {
		var c OutboxChange
		var changesRaw, snapRaw []byte
		if err := rows.Scan(&c.ID, &c.EntityType, &c.EntityID, &changesRaw, &snapRaw); err != nil {
			return nil, fmt.Errorf("crnotice: scan change: %w", err)
		}
		if err := json.Unmarshal(changesRaw, &c.Changes); err != nil {
			return nil, fmt.Errorf("crnotice: decode changes for outbox %d: %w", c.ID, err)
		}
		if err := json.Unmarshal(snapRaw, &c.Snapshot); err != nil {
			return nil, fmt.Errorf("crnotice: decode snapshot for outbox %d: %w", c.ID, err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// Details reads the surrounding detail a notice needs.
//
// The outbox snapshot is to_jsonb(NEW) of the table that changed, so a
// change_request row change carries change_request's own columns and nothing
// else: the number is on work_item, the project and the requester are joins
// away. The ServiceNow flows this replaces read them off the triggering record
// too, so reading them here is the faithful port, not an addition.
//
// A change request that does not exist yields a zero value and no error: the
// row can be deleted between the outbox row being written and this running,
// and a notice about a deleted record is a silent no-op, not a fault to retry.
//
// crvis: background notice drainer under the system identity; no customer reads through it
func (r *crNoticeRepository) Details(ctx context.Context, id string) (CRNoticeDetails, error) {
	// WithSystemIdentity: change_request's RLS policies (migration 0145)
	// would otherwise silently filter this read to zero rows -- this
	// repository is the CR-notice drainer's own reader, an internal
	// background process with no customer viewer to speak for (same
	// reasoning as githubMutationRepository.withGithubSystemIdentity), and
	// this method's own doc comment already treats a missing row as a
	// legitimate silent no-op, which is exactly the failure mode a
	// forgotten identity stamp here would produce for every change request,
	// not just deleted ones.
	ctx = WithSystemIdentity(ctx)
	const query = `
		SELECT wi.number,
		       COALESCE(cr.git_reference, ''),
		       COALESCE(
		           NULLIF(TRIM(COALESCE(u.name, '')), ''),
		           NULLIF(TRIM(COALESCE(u.first_name, '') || ' ' || COALESCE(u.last_name, '')), ''),
		           ''
		       ),
		       COALESCE(wi.project_id::text, ''),
		       COALESCE(p.name, ''),
		       COALESCE(wi.subject, ''),
		       COALESCE(wi.description, ''),
		       -- Last name first: the ServiceNow templates interpolate the two
		       -- name pills in that order, and a port that quietly reorders
		       -- them is a port that reads differently from the mail it
		       -- replaces.
		       COALESCE(
		           NULLIF(TRIM(COALESCE(a.last_name, '') || ' ' || COALESCE(a.first_name, '')), ''),
		           NULLIF(TRIM(COALESCE(a.name, '')), ''),
		           ''
		       ),
		       lower(COALESCE(a.email, wi.updated_by, '')) LIKE '%@wso2.com'
		FROM change_request cr
		JOIN work_item wi   ON wi.id = cr.id
		-- requested_by_user_id, not work_item.opened_by_user_id: the change
		-- request carries its own requester, which is ServiceNow's own
		-- requested_by and the person the notice is actually about. The opener
		-- is whoever created the record, frequently the sync itself.
		LEFT JOIN "user" u  ON u.id = cr.requested_by_user_id
		LEFT JOIN project p ON p.id = wi.project_id
		-- The actor: whoever last wrote the row. work_item.updated_by is a
		-- username string, not a reference, so this matches on either of the
		-- two things it is ever set to.
		LEFT JOIN "user" a  ON lower(a.user_name) = lower(wi.updated_by)
		                    OR lower(a.email)     = lower(wi.updated_by)
		WHERE cr.id = $1::uuid`

	var d CRNoticeDetails
	err := r.db.QueryRow(ctx, query, id).Scan(
		&d.Number, &d.GitReference, &d.RequesterName, &d.ProjectID, &d.ProjectName,
		&d.ShortDescription, &d.Description, &d.ActorName, &d.ActorIsWSO2,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return CRNoticeDetails{}, nil
	}
	if err != nil {
		return CRNoticeDetails{}, fmt.Errorf("crnotice: read change request %s: %w", id, err)
	}
	return d, nil
}

// GroupMemberEmails returns the addresses of everyone in a named team — the
// "Devops Approval" / "CAB Approval" / "Devops Review" audiences.
//
// By NAME, not id: the ServiceNow subflow hardcoded three group sys_ids, which
// mean nothing after the migration. The name survives and is also what a
// reader of the ported logic recognises.
//
// crvis: background notice drainer under the system identity; reads team membership, not a change request
func (r *crNoticeRepository) GroupMemberEmails(ctx context.Context, teamName string) ([]string, error) {
	const query = `
		SELECT u.email
		FROM team_member tm
		JOIN team t   ON t.id = tm.team_id
		JOIN "user" u ON u.id = tm.user_id
		WHERE t.name = $1
		  AND COALESCE(u.email, '') <> ''
		ORDER BY u.email`
	return r.emails(ctx, query, teamName)
}

// ProjectContactEmails returns a customer project's contact addresses.
//
// crvis: background notice drainer under the system identity; the building block of CustomerNoticeEmails's legacy audience
func (r *crNoticeRepository) ProjectContactEmails(ctx context.Context, projectID string) ([]string, error) {
	if projectID == "" {
		// No project on the record means no customer audience. An empty list,
		// not an error: the caller treats "nobody to tell" as a silent no-op.
		return nil, nil
	}
	return r.emails(ctx, projectContactEmailsQuery, projectID)
}

// CustomerNoticeEmails returns who a change request's customer notices (the
// request for the customer's approval or review, the answer to a proposed time)
// are addressed to: the contacts the change request was DESIGNATED to, i.e. the
// registered contacts of its project who hold an approver row on one of its
// customer stages, in any state. A change request nobody has been asked about
// has no audience, EXCEPT a legacy one (created before the strict-visibility
// cutover, see CRVisibility): such a change request is visible to its whole
// project and, with no customer rows, keeps going to every contact of the
// project as before.
//
// Mail is a disclosure too: telling a contact who was never asked that a change
// request exists (its number and subject are in the notice) would defeat the
// visibility rule the portal enforces.
//
// crvis: background notice drainer under the system identity: it DECIDES who is mailed, and the audience it returns is the designated contacts (the visibility rule applied to mail)
func (r *crNoticeRepository) CustomerNoticeEmails(ctx context.Context, changeRequestID, projectID string) ([]string, error) {
	if projectID == "" || changeRequestID == "" {
		return nil, nil
	}
	sys := WithSystemIdentity(ctx)
	var anyDesignated, legacy bool
	err := r.db.QueryRow(sys, `
		SELECT EXISTS (SELECT 1 FROM approval_stage_approver asa JOIN approval_stage ast ON ast.id = asa.stage_id
		                WHERE ast.work_item_id = $1::uuid AND ast.checkpoint_label IN (`+sqlStringList(crVisibilityCustomerStageLabels)+`)
		                  AND asa.state IN (`+sqlStringList(crDesignationApproverStates)+`)),
		       COALESCE((SELECT $2::timestamptz IS NULL OR wi.created_on < $2::timestamptz FROM work_item wi WHERE wi.id = $1::uuid), false)`,
		changeRequestID, r.vis.StrictFrom).Scan(&anyDesignated, &legacy)
	if err != nil {
		return nil, fmt.Errorf("crnotice: read the audience of change request %s: %w", changeRequestID, err)
	}
	if !anyDesignated {
		if legacy {
			return r.ProjectContactEmails(ctx, projectID)
		}
		return nil, nil
	}
	rows, err := r.db.Query(sys, `
		SELECT pc.email
		FROM project_contact pc
		WHERE pc.project_id = $2::uuid
		  AND pc.state = 'REGISTERED'
		  AND COALESCE(pc.email, '') <> ''
		  AND LOWER(pc.email) IN (
		      SELECT LOWER(u.email)
		        FROM approval_stage_approver asa
		        JOIN approval_stage ast ON ast.id = asa.stage_id
		        JOIN "user" u ON u.id = asa.approver_user_id
		       WHERE ast.work_item_id = $1::uuid
		         AND ast.checkpoint_label IN (`+sqlStringList(crVisibilityCustomerStageLabels)+`)
		         AND asa.state IN (`+sqlStringList(crDesignationApproverStates)+`)
		         AND COALESCE(u.email, '') <> '')
		ORDER BY pc.email`, changeRequestID, projectID)
	if err != nil {
		return nil, fmt.Errorf("crnotice: query designated contacts: %w", err)
	}
	defer rows.Close()
	out := make([]string, 0)
	for rows.Next() {
		var e string
		if err := rows.Scan(&e); err != nil {
			return nil, fmt.Errorf("crnotice: scan designated contact: %w", err)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// projectContactEmailsQuery reads a project's contact addresses, leaving out
// DEACTIVATED memberships: the Salesforce ingest keeps a removed contact as a
// DEACTIVATED row (ServiceNow hard-deleted it), and a person removed from the
// project must not keep receiving its change-request notices.
const projectContactEmailsQuery = `
		SELECT pc.email
		FROM project_contact pc
		WHERE pc.project_id = $1::uuid
		  AND COALESCE(pc.email, '') <> ''
		  AND (pc.state IS NULL OR pc.state <> 'DEACTIVATED'::project_contact_state_enum)
		ORDER BY pc.email`

func (r *crNoticeRepository) emails(ctx context.Context, query string, arg any) ([]string, error) {
	// WithSystemIdentity: shared by GroupMemberEmails/ProjectContactEmails --
	// same reasoning as ClaimChanges' own stamp above.
	ctx = WithSystemIdentity(ctx)
	rows, err := r.db.Query(ctx, query, arg)
	if err != nil {
		return nil, fmt.Errorf("crnotice: query emails: %w", err)
	}
	defer rows.Close()

	out := make([]string, 0)
	for rows.Next() {
		var e string
		if err := rows.Scan(&e); err != nil {
			return nil, fmt.Errorf("crnotice: scan email: %w", err)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
