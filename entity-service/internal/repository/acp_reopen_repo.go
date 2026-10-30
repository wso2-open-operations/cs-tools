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
// KIND, either express or implied. See the License for the
// specific language governing permissions and limitations
// under the License.

package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// ACPReopenRepository is the database side of the ACP reopen check
// (internal/service/acp_reopen.go): it reads a project's closure states and
// sets the tracks the check decided to reopen back to OPEN.
//
// Every method runs as the system: the Salesforce event that triggers the
// check carries no caller, and project is not a row-level-security table
// (see UpdateProject).
type ACPReopenRepository interface {
	// GetProjectBySfID returns the project a Salesforce project id stands for
	// (the row resolveProjectBySfIDQuery picks), nil when CSM has none.
	GetProjectBySfID(ctx context.Context, sfID string) (*domain.ACPReopenProject, error)
	// Reopen runs one reopen in a transaction. It locks the project row and
	// hands plan the project as it is now; plan decides the write from that
	// (ok false: nothing to write, nothing changes). The write sets the
	// chosen tracks to OPEN and stores the suspensionProcessState plan gave,
	// then recomputes the overall closure state with the same rule as
	// UpdateProject. beforeCommit runs next, inside the transaction, with the
	// outcome: an error from it rolls everything back. notice runs last: a
	// payload it returns is queued in event_outbox, in the same transaction,
	// for ACPReactivationOutbox to publish as project.reactivated (nil: no
	// notice). done reports whether anything was written.
	Reopen(ctx context.Context, projectID string,
		plan func(current domain.ACPReopenProject) (domain.ACPReopenWrite, bool, error),
		beforeCommit func(ctx context.Context, outcome domain.ACPReopenOutcome) error,
		notice func(outcome domain.ACPReopenOutcome) (json.RawMessage, error),
	) (outcome domain.ACPReopenOutcome, done bool, err error)
	// AnyACPSuspendedInvoiceTrack reports whether any project has its invoice
	// track suspended by ACP. An invoice event can only reopen such a
	// project, so without one it needs no Salesforce call at all.
	AnyACPSuspendedInvoiceTrack(ctx context.Context) (bool, error)
	// ListACPNotedProjects returns, in id order after afterID ("" for the
	// start), up to limit projects with a Salesforce id whose
	// suspensionProcessState still records an ACP suspension on a track
	// ("suspend"): the projects the daily check looks at.
	ListACPNotedProjects(ctx context.Context, afterID string, limit int) ([]domain.ACPReopenProject, error)
}

// ACPReactivationOutboxEntityType is the event_outbox entity_type of a queued
// "Back to Open state" notice. Every other drainer reads only its own types.
const ACPReactivationOutboxEntityType = "project_reactivated"

type acpReopenRepo struct {
	db *Scoped
}

// NewACPReopenRepository constructs an ACPReopenRepository backed by the scoped pool.
func NewACPReopenRepository(db *Scoped) ACPReopenRepository {
	return &acpReopenRepo{db: db}
}

// acpReopenProjectColumns is what ACPReopenProject holds, read from project p.
const acpReopenProjectColumns = `p.id::text, COALESCE(p.sf_id, ''), COALESCE(p.name, ''), COALESCE(p.key, ''),
	p.end_date_closure_state::TEXT, p.invoice_due_date_closure_state::TEXT,
	p.compliance_violation_closure_state::TEXT, p.wso2_closure_state::TEXT,
	p.suspension_process_state`

func scanACPReopenProject(row pgx.Row) (domain.ACPReopenProject, error) {
	var p domain.ACPReopenProject
	err := row.Scan(&p.ID, &p.SfID, &p.Name, &p.Key,
		&p.EndDateClosureState, &p.InvoiceDueDateClosureState,
		&p.ComplianceViolationClosureState, &p.ClosureState,
		&p.SuspensionProcessState)
	return p, err
}

// GetProjectBySfID implements ACPReopenRepository.
func (r *acpReopenRepo) GetProjectBySfID(ctx context.Context, sfID string) (*domain.ACPReopenProject, error) {
	ctx = WithSystemIdentity(ctx)
	id, err := resolveIDBySfID(ctx, r.db, resolveProjectBySfIDQuery, "project", sfID)
	if err != nil || id == nil {
		return nil, err
	}
	p, err := scanACPReopenProject(r.db.QueryRow(ctx, `SELECT `+acpReopenProjectColumns+` FROM project p WHERE p.id = $1`, *id))
	if errors.Is(err, pgx.ErrNoRows) {
		// Deleted between the two statements.
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("acp reopen: read project: %w", err)
	}
	return &p, nil
}

// acpReopenUpdate sets the chosen tracks to OPEN and stores the new
// suspensionProcessState. A track that is not reopened keeps its value.
const acpReopenUpdate = `
	UPDATE project
	SET end_date_closure_state = CASE WHEN $2 THEN 'OPEN'::end_date_closure_state_enum ELSE end_date_closure_state END,
	    invoice_due_date_closure_state = CASE WHEN $3 THEN 'OPEN'::invoice_due_date_closure_state_enum ELSE invoice_due_date_closure_state END,
	    suspension_process_state = COALESCE($4::jsonb, suspension_process_state),
	    updated_on = NOW(),
	    updated_by = $5
	WHERE id = $1`

// Reopen implements ACPReopenRepository.
func (r *acpReopenRepo) Reopen(ctx context.Context, projectID string,
	plan func(current domain.ACPReopenProject) (domain.ACPReopenWrite, bool, error),
	beforeCommit func(ctx context.Context, outcome domain.ACPReopenOutcome) error,
	notice func(outcome domain.ACPReopenOutcome) (json.RawMessage, error),
) (domain.ACPReopenOutcome, bool, error) {
	ctx = WithSystemIdentity(ctx)
	var outcome domain.ACPReopenOutcome
	done := false
	err := r.db.InTx(ctx, func(tx pgx.Tx) error {
		current, err := scanACPReopenProject(tx.QueryRow(ctx,
			`SELECT `+acpReopenProjectColumns+` FROM project p WHERE p.id = $1 FOR UPDATE`, projectID))
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("acp reopen: lock project: %w", err)
		}
		write, ok, err := plan(current)
		if err != nil || !ok {
			return err
		}
		if !write.ReopenEndDate && !write.ReopenInvoice && !write.SectionsOnly {
			return nil
		}
		if _, err := tx.Exec(ctx, acpReopenUpdate, projectID, write.ReopenEndDate, write.ReopenInvoice,
			suspensionStateArg(write.SuspensionProcessState), domain.ACPReopenActor); err != nil {
			return fmt.Errorf("acp reopen: update project: %w", err)
		}
		if err := tx.QueryRow(ctx,
			`UPDATE project SET wso2_closure_state = `+wso2ClosureStateRule+` WHERE id = $1 RETURNING wso2_closure_state::TEXT`,
			projectID).Scan(&outcome.ClosureState); err != nil {
			return fmt.Errorf("acp reopen: recompute closure state: %w", err)
		}
		outcome.Open = outcome.ClosureState != nil && *outcome.ClosureState == "OPEN"
		if beforeCommit != nil {
			if err := beforeCommit(ctx, outcome); err != nil {
				return err
			}
		}
		if notice != nil {
			payload, err := notice(outcome)
			if err != nil {
				return err
			}
			if payload != nil {
				if _, err := tx.Exec(ctx, `INSERT INTO event_outbox (entity_type, entity_id, changes, snapshot)
					VALUES ($1, $2::uuid, '{}'::jsonb, $3::jsonb)`,
					ACPReactivationOutboxEntityType, projectID, string(payload)); err != nil {
					return fmt.Errorf("acp reopen: queue the reactivation notice: %w", err)
				}
			}
		}
		done = true
		return nil
	})
	if err != nil {
		return domain.ACPReopenOutcome{}, false, err
	}
	return outcome, done, nil
}

// acpSectionSuspended is true when a project's suspensionProcessState records
// ACP's suspension (event_type "suspend") in the section named by $key. A
// section that is not an object reads as NULL, never as a match.
func acpSectionSuspended(key string) string {
	return `COALESCE(p.suspension_process_state -> '` + key + `' ->> 'event_type', '') = 'suspend'`
}

// AnyACPSuspendedInvoiceTrack implements ACPReopenRepository.
func (r *acpReopenRepo) AnyACPSuspendedInvoiceTrack(ctx context.Context) (bool, error) {
	ctx = WithSystemIdentity(ctx)
	var found bool
	err := r.db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM project p
		WHERE p.invoice_due_date_closure_state::TEXT LIKE 'SUSPENDED%' AND `+acpSectionSuspended("based_on_due_invoices")+`)`).Scan(&found)
	if err != nil {
		return false, fmt.Errorf("acp reopen: look for an ACP-suspended invoice track: %w", err)
	}
	return found, nil
}

// ListACPNotedProjects implements ACPReopenRepository.
func (r *acpReopenRepo) ListACPNotedProjects(ctx context.Context, afterID string, limit int) ([]domain.ACPReopenProject, error) {
	ctx = WithSystemIdentity(ctx)
	if afterID == "" {
		afterID = "00000000-0000-0000-0000-000000000000"
	}
	rows, err := r.db.Query(ctx, `SELECT `+acpReopenProjectColumns+` FROM project p
		WHERE p.id > $1::uuid AND COALESCE(p.sf_id, '') <> ''
		  AND (`+acpSectionSuspended("based_on_subscription_end_date")+` OR `+acpSectionSuspended("based_on_due_invoices")+`)
		ORDER BY p.id
		LIMIT $2`, afterID, limit)
	if err != nil {
		return nil, fmt.Errorf("acp reopen: list projects ACP suspended: %w", err)
	}
	defer rows.Close()
	var out []domain.ACPReopenProject
	for rows.Next() {
		p, err := scanACPReopenProject(rows)
		if err != nil {
			return nil, fmt.Errorf("acp reopen: scan project: %w", err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
