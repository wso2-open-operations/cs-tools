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
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// salesforceWriteRequestErrorChars bounds last_error (VARCHAR(500)). The
// stored text is the error's own message, never an upstream response body.
const salesforceWriteRequestErrorChars = 500

// SalesforceWriteRequestRepository is the ledger of deferred Salesforce
// membership writes (migration 0249). Rows are created and completed inside
// the membership write transaction (see upsertWithinTx); the methods here are
// the out-of-transaction reads and the attempt bookkeeping the consumer does.
type SalesforceWriteRequestRepository interface {
	Get(ctx context.Context, id string) (domain.SalesforceWriteRequest, error)
	// PendingForProjectContact returns the open request for a membership row,
	// nil when there is none. The partial unique index guarantees at most one.
	PendingForProjectContact(ctx context.Context, projectContactID string) (*domain.SalesforceWriteRequest, error)
	// LatestForProjectContact returns the most recent request for a
	// membership row whatever its status, nil when there is none: what a
	// write on a row with no Salesforce id consults to tell "still being
	// synced" from "sync failed, nothing is coming".
	LatestForProjectContact(ctx context.Context, projectContactID string) (*domain.SalesforceWriteRequest, error)
	// SyncStatusForProjectContacts returns, for each membership row that has
	// a request, the status the portals show: PENDING when a request is
	// open, FAILED when the latest one gave up. Rows whose latest request
	// SUCCEEDED are omitted -- they are in Salesforce like any other.
	SyncStatusForProjectContacts(ctx context.Context, projectContactIDs []string) (map[string]string, error)
	// MarkPublished stamps published_on once Event Hub acknowledged the
	// request's event.
	MarkPublished(ctx context.Context, id string) error
	// RecordAttempt counts one failed Salesforce attempt and keeps its error;
	// the row stays PENDING.
	RecordAttempt(ctx context.Context, id string, cause error) error
	// MarkFailed ends the request: Salesforce refused it for good
	// (retryable false), or every retry tier was exhausted while it was
	// unavailable (retryable true). attempts is bumped too, since this is
	// the outcome of an attempt.
	MarkFailed(ctx context.Context, id string, cause error, retryable bool) error
}

type salesforceWriteRequestRepo struct {
	db *pgxpool.Pool
}

// NewSalesforceWriteRequestRepository constructs the repository on the pool.
// The table carries no customer-visible data beyond what project_contact
// already holds and is read only by internal callers, so it is not under RLS.
func NewSalesforceWriteRequestRepository(db *pgxpool.Pool) SalesforceWriteRequestRepository {
	return &salesforceWriteRequestRepo{db: db}
}

const salesforceWriteRequestColumns = `
	id, operation, project_id, project_contact_id, email, intent, status,
	attempts, last_error, last_attempt_on, published_on, completed_on, retryable, created_on, updated_on`

func scanSalesforceWriteRequest(row pgx.Row) (domain.SalesforceWriteRequest, error) {
	var r domain.SalesforceWriteRequest
	var projectContactID *string
	var status string
	if err := row.Scan(&r.ID, &r.Operation, &r.ProjectID, &projectContactID, &r.Email, &r.Intent, &status,
		&r.Attempts, &r.LastError, &r.LastAttemptOn, &r.PublishedOn, &r.CompletedOn, &r.Retryable, &r.CreatedOn, &r.UpdatedOn); err != nil {
		return domain.SalesforceWriteRequest{}, err
	}
	if projectContactID != nil {
		r.ProjectContactID = *projectContactID
	}
	r.Status = domain.SalesforceWriteRequestStatus(status)
	return r, nil
}

func (r *salesforceWriteRequestRepo) Get(ctx context.Context, id string) (domain.SalesforceWriteRequest, error) {
	req, err := scanSalesforceWriteRequest(r.db.QueryRow(ctx,
		`SELECT `+salesforceWriteRequestColumns+` FROM salesforce_write_request WHERE id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.SalesforceWriteRequest{}, &apierror.NotFoundError{Msg: "salesforce write request not found"}
	}
	if err != nil {
		return domain.SalesforceWriteRequest{}, fmt.Errorf("salesforce write request: get: %w", err)
	}
	return req, nil
}

func (r *salesforceWriteRequestRepo) PendingForProjectContact(ctx context.Context, projectContactID string) (*domain.SalesforceWriteRequest, error) {
	req, err := scanSalesforceWriteRequest(r.db.QueryRow(ctx,
		`SELECT `+salesforceWriteRequestColumns+` FROM salesforce_write_request
		 WHERE project_contact_id = $1 AND status = 'PENDING' LIMIT 1`, projectContactID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("salesforce write request: pending for project contact: %w", err)
	}
	return &req, nil
}

func (r *salesforceWriteRequestRepo) LatestForProjectContact(ctx context.Context, projectContactID string) (*domain.SalesforceWriteRequest, error) {
	req, err := scanSalesforceWriteRequest(r.db.QueryRow(ctx,
		`SELECT `+salesforceWriteRequestColumns+` FROM salesforce_write_request
		 WHERE project_contact_id = $1 ORDER BY created_on DESC LIMIT 1`, projectContactID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("salesforce write request: latest for project contact: %w", err)
	}
	return &req, nil
}

func (r *salesforceWriteRequestRepo) SyncStatusForProjectContacts(ctx context.Context, projectContactIDs []string) (map[string]string, error) {
	out := map[string]string{}
	if len(projectContactIDs) == 0 {
		return out, nil
	}
	// The latest request per membership row decides: an open one is PENDING,
	// a failed one FAILED, a succeeded one says nothing (omitted). A PENDING
	// request that was never published (the command publish failed) reads
	// as FAILED: nothing will consume it, and the writes already treat it
	// that way (see service.deferredWrite), so the list must not say
	// "still syncing" for a row the actions say to remove and re-invite.
	rows, err := r.db.Query(ctx, `
		SELECT DISTINCT ON (project_contact_id) project_contact_id,
		       CASE WHEN status = 'PENDING' AND published_on IS NULL THEN 'FAILED' ELSE status END
		FROM salesforce_write_request
		WHERE project_contact_id = ANY($1::uuid[])
		ORDER BY project_contact_id, created_on DESC`, projectContactIDs)
	if err != nil {
		return nil, fmt.Errorf("salesforce write request: sync status: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id, status string
		if err := rows.Scan(&id, &status); err != nil {
			return nil, fmt.Errorf("salesforce write request: scan sync status: %w", err)
		}
		// SUCCEEDED is "in Salesforce like any other"; CANCELLED rows are
		// deactivated memberships, which the lists do not show.
		if status != string(domain.SalesforceWriteRequestSucceeded) && status != string(domain.SalesforceWriteRequestCancelled) {
			out[id] = status
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("salesforce write request: iterate sync status: %w", err)
	}
	return out, nil
}

func (r *salesforceWriteRequestRepo) MarkPublished(ctx context.Context, id string) error {
	if _, err := r.db.Exec(ctx, `
		UPDATE salesforce_write_request SET published_on = NOW(), updated_on = NOW()
		WHERE id = $1 AND published_on IS NULL`, id); err != nil {
		return fmt.Errorf("salesforce write request: mark published: %w", err)
	}
	return nil
}

func (r *salesforceWriteRequestRepo) RecordAttempt(ctx context.Context, id string, cause error) error {
	if _, err := r.db.Exec(ctx, `
		UPDATE salesforce_write_request
		SET attempts = attempts + 1, last_error = $2, last_attempt_on = NOW(), updated_on = NOW()
		WHERE id = $1 AND status = 'PENDING'`, id, truncateSalesforceWriteError(cause)); err != nil {
		return fmt.Errorf("salesforce write request: record attempt: %w", err)
	}
	return nil
}

func (r *salesforceWriteRequestRepo) MarkFailed(ctx context.Context, id string, cause error, retryable bool) error {
	if _, err := r.db.Exec(ctx, `
		UPDATE salesforce_write_request
		SET status = 'FAILED', attempts = attempts + 1, last_error = $2, last_attempt_on = NOW(),
		    completed_on = NOW(), retryable = $3, updated_on = NOW()
		WHERE id = $1 AND status = 'PENDING'`, id, truncateSalesforceWriteError(cause), retryable); err != nil {
		return fmt.Errorf("salesforce write request: mark failed: %w", err)
	}
	return nil
}

func truncateSalesforceWriteError(cause error) *string {
	if cause == nil {
		return nil
	}
	msg := strings.TrimSpace(cause.Error())
	if msg == "" {
		return nil
	}
	// The column limit counts characters; cut on a rune boundary so a
	// multi-byte character spanning the limit cannot turn into invalid
	// UTF-8 that PostgreSQL rejects (which would fail the very UPDATE that
	// records the failure and leave the request PENDING).
	msg = strings.ToValidUTF8(msg, "")
	if r := []rune(msg); len(r) > salesforceWriteRequestErrorChars {
		msg = string(r[:salesforceWriteRequestErrorChars])
	}
	return &msg
}

// insertSalesforceWriteRequestTx records a deferred write inside the
// membership write transaction. The intent is stored as JSON; the row is
// PENDING and unpublished until the service publishes its event.
func insertSalesforceWriteRequestTx(ctx context.Context, tx querier, dw domain.DeferredSalesforceWrite, projectID, projectContactID, email string) (string, error) {
	intent, err := json.Marshal(dw.Intent)
	if err != nil {
		return "", fmt.Errorf("salesforce write request: encode intent: %w", err)
	}
	var id string
	if err := tx.QueryRow(ctx, `
		INSERT INTO salesforce_write_request (operation, project_id, project_contact_id, email, intent)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id`, dw.Operation, projectID, projectContactID, strings.ToLower(strings.TrimSpace(email)), intent).Scan(&id); err != nil {
		return "", fmt.Errorf("salesforce write request: insert: %w", err)
	}
	return id, nil
}

// cancelSalesforceWriteRequestTx closes a FAILED or never-published request as
// CANCELLED in the transaction that deactivates its membership row locally:
// nothing may pick it up afterwards. A published PENDING request is left as
// it is (the caller refuses the deactivation in that case), and a SUCCEEDED
// one is not a candidate.
func cancelSalesforceWriteRequestTx(ctx context.Context, tx querier, id string) error {
	if _, err := tx.Exec(ctx, `
		UPDATE salesforce_write_request
		SET status = 'CANCELLED', completed_on = COALESCE(completed_on, NOW()), updated_on = NOW()
		WHERE id = $1 AND (status = 'FAILED' OR (status = 'PENDING' AND published_on IS NULL))`, id); err != nil {
		return fmt.Errorf("salesforce write request: cancel: %w", err)
	}
	return nil
}

// completeSalesforceWriteRequestTx marks the request SUCCEEDED in the
// transaction that stamped the Salesforce ids. A request that is no longer
// PENDING (a concurrent consumer got there first) is left as it is.
func completeSalesforceWriteRequestTx(ctx context.Context, tx querier, id string) error {
	if _, err := tx.Exec(ctx, `
		UPDATE salesforce_write_request
		SET status = 'SUCCEEDED', attempts = attempts + 1, last_error = NULL, last_attempt_on = NOW(),
		    completed_on = NOW(), updated_on = NOW()
		WHERE id = $1 AND status = 'PENDING'`, id); err != nil {
		return fmt.Errorf("salesforce write request: complete: %w", err)
	}
	return nil
}
