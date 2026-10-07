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
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
)

// NewChangeRequestFromIssue is what a GitHub issue contributes to a new
// change request. Everything not derived from the issue is left to the
// database's own defaults.
// NewServiceRequestFromIssue is a service request raised from a GitHub issue.
//
// The shape follows servicenow_create_case.yml: a catalog, the issue's own
// title and body, and every "### Field" the template captured, kept as the
// u_-prefixed keys extractFields.js produces. CS0441366 stores exactly that in
// service_request.json_data, so the fields have a home already and do not need
// flattening into a description.
type NewServiceRequestFromIssue struct {
	Subject      string
	Description  string
	GitReference string
	IssueNumber  int
	AccountID    string
	// Catalog is service_request.category: "Generic Requests" for a change,
	// "General Requests" for a plain service request.
	Catalog string
	// SRType is the change class -- "Normal Change" and so on -- carried from
	// the CR/*Change label. Empty for a non-change request.
	SRType string
	// Fields are the template's captured values, stored verbatim.
	Fields    map[string]string
	CreatedBy string
}

type NewChangeRequestFromIssue struct {
	Subject      string
	Description  string
	GitReference string
	// Impact, Likelihood and Type are enum names or "" when the issue carried
	// no label for them -- empty stays NULL rather than defaulting, so "not
	// stated" and "deliberately lowest" remain distinguishable.
	Impact     string
	Likelihood string
	Type       string
	// ProjectID ties the record to a customer project when the repository
	// mapping knows one. Empty leaves it null.
	ProjectID string
	CreatedBy string
	// Catalog, SRType and Fields carry the service-request side of an issue.
	// They are only read when the record turns out to be a service request --
	// see UpdateFromIssue, which decides that from whether a change_request row
	// exists rather than from the caller having to know.
	Catalog string
	SRType  string
	Fields  map[string]string
}

// ErrChangeRequestExists means the issue already has one.
var ErrChangeRequestExists = errors.New("github: a change request already exists for this issue")

// GithubMutationRepository writes change requests on behalf of the sync.
type GithubMutationRepository interface {
	// CreateServiceRequestFromIssue creates the record a GitHub issue actually
	// becomes -- a service request, linked to the issue so the outbound sync
	// picks it up from the moment it exists.
	CreateServiceRequestFromIssue(ctx context.Context, in NewServiceRequestFromIssue) (id, number string, err error)
	// CreateFromIssue creates work_item and change_request together and
	// returns the new id and number.
	CreateFromIssue(ctx context.Context, in NewChangeRequestFromIssue) (id, number string, err error)
	// UpdateFromIssue applies an edited issue to an existing change request.
	UpdateFromIssue(ctx context.Context, id string, in NewChangeRequestFromIssue) error
	// SetState moves a change request, and only if it is not already there --
	// a redundant write would enqueue an outbound push about nothing.
	SetState(ctx context.Context, id, state string) (changed bool, err error)
	// AddComment records a comment relayed from GitHub.
	AddComment(ctx context.Context, changeRequestID, content, createdBy string) error
	// SetAssignee records who owns the change request.
	//
	// Takes a WSO2 user id, not a GitHub login. Resolving one to the other is
	// the caller's problem and there is no mapping for it today -- see
	// UserIDForGithubLogin.
	SetAssignee(ctx context.Context, id, userID string) (changed bool, err error)
	// UserIDForGithubLogin resolves a GitHub account to a WSO2 user, matching
	// on email local-part as the only signal available. Empty when no
	// confident match exists -- a wrong assignee is worse than none.
	UserIDForGithubLogin(ctx context.Context, login string) (string, error)
}

type githubMutationRepository struct {
	db *Scoped
}

// NewGithubMutationRepository constructs the writer.
func NewGithubMutationRepository(db *Scoped) GithubMutationRepository {
	return &githubMutationRepository{db: db}
}

// withGithubSystemIdentity stamps ctx as an internal caller before every
// call this file's and github_sync_repo.go's repositories make through
// Scoped. Both are the GitHub webhook sync -- a system-to-system integration
// authenticated by HMAC, not a customer's own request -- so there is no real
// "viewer" identity to forward here, and treating it as internal is correct
// by construction, the same reasoning already applied to
// CreateChangeRequestFromServiceNow and to the SLA engine's own background
// worker (see NewSLAEngineRepository's doc comment).
//
// This is not optional only for the tables migration 0145 protects: EVERY
// Scoped method requires SOME identity on ctx regardless of whether the
// target table has RLS at all (Scoped.Query/QueryRow/Exec return
// ErrNoCallerIdentity outright otherwise) -- so every method on
// githubMutationRepository/githubSyncRepository needs this stamp, not just
// the ones touching change_request/approval_stage.
func withGithubSystemIdentity(ctx context.Context) context.Context {
	return WithSystemIdentity(ctx)
}

// nullable turns "" into a SQL NULL so an absent value is absent rather than
// an empty string that reads as a real one.
func nullable(v string) any {
	if v == "" {
		return nil
	}
	return v
}

// crvis: GitHub inbound sync under the system identity (M2M-only webhook handlers); no customer identity reaches it
func (r *githubMutationRepository) CreateFromIssue(ctx context.Context, in NewChangeRequestFromIssue) (string, string, error) {
	ctx = withGithubSystemIdentity(ctx)
	var id, number string
	err := r.db.InTx(ctx, func(tx pgx.Tx) error {
		// The work item and its change_request extension are one record split
		// across two tables; a half-written one is worse than none.
		const insertWorkItem = `
			INSERT INTO work_item (id, created_on, updated_on, created_by, updated_by,
			                       number, subject, type, description, project_id)
			VALUES (gen_random_uuid(), NOW(), NOW(), $1, $1,
			        next_github_change_request_number(), $2, 'CHANGE_REQUEST', $3, $4::uuid)
			RETURNING id::text, number`

		if err := tx.QueryRow(ctx, insertWorkItem,
			in.CreatedBy, in.Subject, nullable(in.Description), nullable(in.ProjectID),
		).Scan(&id, &number); err != nil {
			return fmt.Errorf("github: insert work item: %w", err)
		}

		const insertCR = `
			INSERT INTO change_request (id, state, git_reference, impact, likelihood, change_request_type)
			VALUES ($1::uuid, 'NEW', $2,
			        $3::change_request_impact_enum,
			        $4::change_request_likelihood_enum,
			        $5::change_request_type_enum)`
		if _, err := tx.Exec(ctx, insertCR, id, in.GitReference,
			nullable(in.Impact), nullable(in.Likelihood), nullable(in.Type)); err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "23505" {
				return ErrChangeRequestExists
			}
			return fmt.Errorf("github: insert change request: %w", err)
		}
		return nil
	})
	if err != nil {
		return "", "", err
	}
	return id, number, nil
}

// crvis: GitHub inbound sync under the system identity (M2M-only webhook handlers); no customer identity reaches it
func (r *githubMutationRepository) UpdateFromIssue(ctx context.Context, id string, in NewChangeRequestFromIssue) error {
	ctx = withGithubSystemIdentity(ctx)
	return r.db.InTx(ctx, func(tx pgx.Tx) error {
		// COALESCE so a field the issue no longer states keeps its current value
		// rather than being cleared. An edit that drops a label should not wipe
		// what someone set in the portal.
		const updateWorkItem = `
			UPDATE work_item
			SET subject     = $2,
			    description = COALESCE($3, description),
			    updated_on  = NOW(),
			    updated_by  = $4
			WHERE id = $1::uuid`
		if _, err := tx.Exec(ctx, updateWorkItem, id, in.Subject, nullable(in.Description), in.CreatedBy); err != nil {
			return fmt.Errorf("github: update work item: %w", err)
		}

		// WHICH EXTENSION TABLE THIS RECORD LIVES IN DECIDES WHAT ELSE UPDATES.
		// An issue creates a service request, not a change request, so this used to
		// run an UPDATE against change_request that matched no row -- reporting
		// success while service_request.json_data kept whatever the issue said when
		// it was first seen. Editing the issue moved the subject and description and
		// silently left every captured field stale.
		ct, err := tx.Exec(ctx, `
			UPDATE change_request
			SET impact              = COALESCE($2::change_request_impact_enum, impact),
			    likelihood          = COALESCE($3::change_request_likelihood_enum, likelihood),
			    change_request_type = COALESCE($4::change_request_type_enum, change_request_type)
			WHERE id = $1::uuid`, id,
			nullable(in.Impact), nullable(in.Likelihood), nullable(in.Type))
		if err != nil {
			return fmt.Errorf("github: update change request: %w", err)
		}

		if ct.RowsAffected() == 0 {
			// Re-extract from the issue body rather than patching key by key: the
			// body is the source of truth, and a field removed from the template
			// should stop being reported. The two derived keys are not in the body
			// and are re-applied so they survive the rewrite.
			fields := in.Fields
			if fields == nil {
				fields = map[string]string{}
			}
			if in.SRType != "" {
				fields["u_sr_type"] = in.SRType
			}
			if in.GitReference != "" {
				fields["u_github_issue_url"] = in.GitReference
			}
			payload, err := json.Marshal(fields)
			if err != nil {
				return fmt.Errorf("github: encode service request fields: %w", err)
			}
			if _, err := tx.Exec(ctx, `
				UPDATE service_request
				SET category  = COALESCE(NULLIF($2, ''), category),
				    json_data = $3::jsonb
				WHERE id = $1::uuid`, id, in.Catalog, payload); err != nil {
				return fmt.Errorf("github: update service request: %w", err)
			}
		}
		return nil
	})
}

// githubSyncActor is what the rows this sync cancels are stamped with
// (approval_stage_approver.updated_by).
const githubSyncActor = "github-sync"

// crvis: GitHub inbound sync under the system identity (M2M-only webhook handlers); no customer identity reaches it
func (r *githubMutationRepository) SetState(ctx context.Context, id, state string) (bool, error) {
	ctx = withGithubSystemIdentity(ctx)
	// IS DISTINCT FROM so a move to the state it already holds writes nothing:
	// the outbound trigger would otherwise enqueue a push announcing a change
	// that did not happen.
	const query = `
		UPDATE change_request
		SET state = $2::change_request_state_enum
		WHERE id = $1::uuid AND state IS DISTINCT FROM $2::change_request_state_enum`
	changed := false
	err := r.db.InTx(ctx, func(tx pgx.Tx) error {
		// A change waiting on the customer (Customer Approval / Customer Review)
		// moves on only through the customer's own answer, never through a label
		// or an issue event: refused, not skipped, so the sync sees it. Read under
		// the row lock the UPDATE below would take anyway.
		var current *string
		switch err := tx.QueryRow(ctx, `SELECT state::text FROM change_request WHERE id = $1::uuid FOR UPDATE`, id).Scan(&current); {
		case errors.Is(err, pgx.ErrNoRows):
			return nil // no such change request: the UPDATE below would match nothing as well
		case err != nil:
			return err
		}
		if current != nil && customerStageSpecForState(strings.ToUpper(*current)) != nil && !strings.EqualFold(*current, state) {
			return &apierror.ValidationError{Msg: fmt.Sprintf(
				"state %q cannot be set from the GitHub sync: the change request is in %s, which only the customer's own answer (given in the Customer Portal) can move it out of",
				state, strings.ToLower(*current))}
		}
		tag, err := tx.Exec(ctx, query, id, state)
		if err != nil {
			return err
		}
		changed = tag.RowsAffected() > 0
		if !changed {
			return nil
		}
		// A state written from outside the approval flow leaves the approvals
		// of the state it left actionable just like a PATCH would (a closed
		// issue closing the change): cancel them in the same transaction.
		return reconcileStaleApprovers(ctx, tx, id, githubSyncActor)
	})
	if err != nil {
		return false, fmt.Errorf("github: set state %s on %s: %w", state, id, err)
	}
	return changed, nil
}

func (r *githubMutationRepository) AddComment(ctx context.Context, changeRequestID, content, createdBy string) error {
	ctx = withGithubSystemIdentity(ctx)
	const query = `
		INSERT INTO comment (id, created_on, created_by, type, work_item_id, content)
		VALUES (gen_random_uuid(), NOW(), $1, 'COMMENT', $2::uuid, $3)`
	if _, err := r.db.Exec(ctx, query, createdBy, changeRequestID, content); err != nil {
		return fmt.Errorf("github: add comment to %s: %w", changeRequestID, err)
	}
	return nil
}

func (r *githubMutationRepository) SetAssignee(ctx context.Context, id, userID string) (bool, error) {
	if userID == "" {
		// Nobody to assign to. A routing gap worth seeing, not a write.
		return false, nil
	}
	ctx = withGithubSystemIdentity(ctx)
	const query = `
		UPDATE work_item
		SET assigned_to_id = $2::uuid, updated_on = NOW()
		WHERE id = $1::uuid AND assigned_to_id IS DISTINCT FROM $2::uuid`
	tag, err := r.db.Exec(ctx, query, id, userID)
	if err != nil {
		return false, fmt.Errorf("github: assign %s: %w", id, err)
	}
	return tag.RowsAffected() > 0, nil
}

// UserIDForGithubLogin implements GithubMutationRepository.
//
// THERE IS NO GITHUB IDENTITY ON THE USER TABLE, so this matches a login
// against the local part of an email address and nothing else. That is a
// heuristic: "nimalp" matches nimalp@wso2.com, and a GitHub account whose
// login bears no relation to the address matches nothing.
//
// Deliberately returns empty rather than guessing when more than one user
// matches. Assigning a change request to the wrong engineer is worse than
// leaving it unassigned, because the wrong one has no reason to look at it.
func (r *githubMutationRepository) UserIDForGithubLogin(ctx context.Context, login string) (string, error) {
	if login == "" {
		return "", nil
	}
	ctx = withGithubSystemIdentity(ctx)
	const query = `
		SELECT id::text FROM "user"
		WHERE lower(split_part(email, '@', 1)) = lower($1)
		  AND COALESCE(email, '') <> ''
		LIMIT 2`
	rows, err := r.db.Query(ctx, query, login)
	if err != nil {
		return "", fmt.Errorf("github: resolve login %q: %w", login, err)
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return "", fmt.Errorf("github: scan user: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return "", err
	}
	if len(ids) != 1 {
		// Nobody, or ambiguous. Either way there is no confident answer.
		return "", nil
	}
	return ids[0], nil
}

var _ = pgx.ErrNoRows

// errAlreadyExists reports that a concurrent writer created the record first.
// Not an API error: the caller turns it into the same "already exists" answer
// a caller retrying after a timeout gets.
var errAlreadyExists = errors.New("github: record already exists for this issue")

// ErrAlreadyExists exposes that sentinel to the service layer.
func ErrAlreadyExists() error { return errAlreadyExists }

// isUniqueViolation reports whether err is Postgres' unique_violation (23505).
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

// workItemByIssue returns the id of the work item already holding this issue.
func (r *githubMutationRepository) workItemByIssue(ctx context.Context, accountID string, issue int) (string, error) {
	ctx = withGithubSystemIdentity(ctx)
	const q = `SELECT id::text FROM work_item
	           WHERE account_id = NULLIF($1, '')::uuid AND github_issue_number = $2`
	var id string
	if err := r.db.QueryRow(ctx, q, accountID, issue).Scan(&id); err != nil {
		return "", fmt.Errorf("github: look up work item for issue %d: %w", issue, err)
	}
	return id, nil
}


// CreateServiceRequestFromIssue implements GithubMutationRepository.
func (r *githubMutationRepository) CreateServiceRequestFromIssue(ctx context.Context, in NewServiceRequestFromIssue) (string, string, error) {
	ctx = withGithubSystemIdentity(ctx)
	var id, number string
	err := r.db.InTx(ctx, func(tx pgx.Tx) error {
		// github_issue_number is set in the same statement that creates the row.
		// Writing it afterwards would leave a window where the record exists and
		// nothing about it syncs -- and the trigger fires on the INSERT, so the
		// link has to be there by then or the first event is lost.
		const insertWorkItem = `
			INSERT INTO work_item (id, created_on, updated_on, created_by, updated_by,
			                       number, wso2_id, subject, type, description,
			                       account_id, github_issue_number)
			VALUES (gen_random_uuid(), NOW(), NOW(), $1, $1,
			        next_github_service_request_number(),
			        -- Required for SERVICE_REQUEST by work_item_wso2_id_required_by_type.
			        next_github_service_request_wso2_id(),
			        $2, 'SERVICE_REQUEST', $3,
			        NULLIF($4, '')::uuid, $5)
			RETURNING id::text, number`

		if err := tx.QueryRow(ctx, insertWorkItem,
			in.CreatedBy, in.Subject, nullable(in.Description), in.AccountID, in.IssueNumber,
		).Scan(&id, &number); err != nil {
			// A concurrent delivery for the same issue got here first. GitHub sends
			// an issue as several events (opened, then labeled), so this is the
			// ordinary case rather than an exotic one: report the record that won
			// instead of failing, and let the caller treat it as already existing.
			if isUniqueViolation(err) {
				existing, lookupErr := r.workItemByIssue(ctx, in.AccountID, in.IssueNumber)
				if lookupErr != nil {
					return lookupErr
				}
				id, number = existing, ""
				return errAlreadyExists
			}
			return fmt.Errorf("github: insert service request work item: %w", err)
		}

		fields := in.Fields
		if fields == nil {
			fields = map[string]string{}
		}
		if in.SRType != "" {
			fields["u_sr_type"] = in.SRType
		}
		if in.GitReference != "" {
			fields["u_github_issue_url"] = in.GitReference
		}
		payload, err := json.Marshal(fields)
		if err != nil {
			return fmt.Errorf("github: encode service request fields: %w", err)
		}

		const insertSR = `
			INSERT INTO service_request (id, state, category, json_data)
			VALUES ($1::uuid, 'OPEN', NULLIF($2, ''), $3::jsonb)`
		if _, err := tx.Exec(ctx, insertSR, id, in.Catalog, payload); err != nil {
			return fmt.Errorf("github: insert service request: %w", err)
		}
		return nil
	})
	if err != nil && !errors.Is(err, errAlreadyExists) {
		return "", "", err
	}
	if errors.Is(err, errAlreadyExists) {
		return id, "", errAlreadyExists
	}
	return id, number, nil
}
