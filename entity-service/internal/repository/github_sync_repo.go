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

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// RepoMapping is a GitHub repository and the account it belongs to.
//
// KEYED BY ACCOUNT, NOT PRODUCT. ServiceNow's github.dispatch.config was a map
// from account name to {owner, repo, credential}, so different customers use
// different repositories and different tokens. An earlier guess keyed this on
// product, which cannot express that.
type RepoMapping struct {
	AccountID   string
	AccountName string
	// CredentialRef names this account's token in the platform secret store.
	// A reference, never the secret itself.
	CredentialRef string
	// Owner and Repository are where this account's issues live. The inbound
	// caller already knows them -- it is holding the webhook that named them --
	// but RepoForAccount starts from a case and has to be told.
	Owner      string
	Repository string
}

// GithubChangeRequest is the slice of a change request the sync reads.
type GithubChangeRequest struct {
	ID     string
	Number string
	State  string
}

// ErrDeliverySeen means this webhook delivery has already been handled.
var ErrDeliverySeen = errors.New("github: delivery already processed")

// GithubSyncRepository is the database side of the GitHub change-request sync.
type GithubSyncRepository interface {
	// RepoMapping resolves a repository to its product and team. Not found is
	// a nil mapping and no error: a repository we do not map is one we do not
	// handle, which is ordinary rather than exceptional.
	RepoMapping(ctx context.Context, owner, repository string) (*RepoMapping, error)
	// RepoForAccount is RepoMapping's other direction: which repository an
	// account's issues are filed in. Needed when the caller has a case rather
	// than a webhook -- filing an issue starts from the account, not the repo.
	// Not found is (nil, nil); an inactive mapping counts as not found.
	RepoForAccount(ctx context.Context, accountID string) (*RepoMapping, error)
	// CaseByIssueNumber finds the work item an issue is linked to, within one
	// account. Comments relayed from GitHub go here, not onto the change
	// request: github_comment_to_sn.yml PATCHes the case, and the outbound
	// trigger only watches case comments -- attaching them to the change
	// request instead left a conversation nobody could reply to.
	// Not found is ("", nil).
	CaseByIssueNumber(ctx context.Context, accountID string, issueNumber int) (string, error)
	// AccountForCase resolves the case's owning account.
	AccountForCase(ctx context.Context, caseID string) (string, error)
	// SetCaseGithubIssueNumber links a case to the issue filed for it.
	//
	// THIS IS WHAT OPENS GATE 2. Until a case carries an issue number the
	// outbound triggers enqueue nothing for it, so this single write is what
	// switches on sync for that case -- see migration 000069.
	SetCaseGithubIssueNumber(ctx context.Context, caseID string, issueNumber int) (changed bool, err error)
	// ChangeRequestByGitReference finds the change request linked to an issue.
	ChangeRequestByGitReference(ctx context.Context, issueURL string) (*GithubChangeRequest, error)
	// ClaimDelivery records a delivery, returning ErrDeliverySeen if another
	// attempt already recorded it.
	ClaimDelivery(ctx context.Context, deliveryID, event, action string) error
	// ReleaseDelivery removes the claim so GitHub's retry can be processed.
	ReleaseDelivery(ctx context.Context, deliveryID string) error
	// LinkDelivery records which change request a delivery resolved to.
	LinkDelivery(ctx context.Context, deliveryID, changeRequestID string) error
}

type githubSyncRepository struct {
	db *pgxpool.Pool
}

// NewGithubSyncRepository constructs the GitHub sync reader/writer.
func NewGithubSyncRepository(db *pgxpool.Pool) GithubSyncRepository {
	return &githubSyncRepository{db: db}
}

func (r *githubSyncRepository) RepoMapping(ctx context.Context, owner, repository string) (*RepoMapping, error) {
	// Lower-cased on both sides: GitHub routes case-insensitively while
	// preserving the case a repository was created with, so "Choreo" and
	// "choreo" are the same repository. ServiceNow compared exactly and every
	// mismatch fell through to a literal "NULL" assignment group.
	const query = `
		SELECT a.id::text,
		       a.name,
		       COALESCE(gr.credential_ref, ''),
		       gr.owner,
		       gr.repository
		FROM account_github_repo gr
		JOIN account a ON a.id = gr.account_id
		WHERE lower(gr.owner) = lower($1)
		  AND lower(gr.repository) = lower($2)
		  AND gr.is_active`

	var m RepoMapping
	err := r.db.QueryRow(ctx, query, owner, repository).Scan(&m.AccountID, &m.AccountName, &m.CredentialRef, &m.Owner, &m.Repository)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("github: repo mapping for %s/%s: %w", owner, repository, err)
	}
	return &m, nil
}

func (r *githubSyncRepository) ChangeRequestByGitReference(ctx context.Context, issueURL string) (*GithubChangeRequest, error) {
	const query = `
		SELECT cr.id::text, wi.number, COALESCE(cr.state::text, '')
		FROM change_request cr
		JOIN work_item wi ON wi.id = cr.id
		WHERE cr.git_reference = $1`

	var cr GithubChangeRequest
	err := r.db.QueryRow(ctx, query, issueURL).Scan(&cr.ID, &cr.Number, &cr.State)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("github: change request for %s: %w", issueURL, err)
	}
	return &cr, nil
}

func (r *githubSyncRepository) ClaimDelivery(ctx context.Context, deliveryID, event, action string) error {
	const query = `
		INSERT INTO github_webhook_delivery (delivery_id, event, action)
		VALUES ($1, $2, NULLIF($3, ''))
		ON CONFLICT (delivery_id) DO NOTHING`

	tag, err := r.db.Exec(ctx, query, deliveryID, event, action)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrDeliverySeen
		}
		return fmt.Errorf("github: claim delivery %s: %w", deliveryID, err)
	}
	// DO NOTHING reports zero rows when the row was already there, which is
	// the ordinary way a redelivery arrives -- not an error condition.
	if tag.RowsAffected() == 0 {
		return ErrDeliverySeen
	}
	return nil
}

func (r *githubSyncRepository) ReleaseDelivery(ctx context.Context, deliveryID string) error {
	_, err := r.db.Exec(ctx, `DELETE FROM github_webhook_delivery WHERE delivery_id = $1`, deliveryID)
	if err != nil {
		return fmt.Errorf("github: release delivery %s: %w", deliveryID, err)
	}
	return nil
}

func (r *githubSyncRepository) LinkDelivery(ctx context.Context, deliveryID, changeRequestID string) error {
	const query = `
		UPDATE github_webhook_delivery
		SET change_request_id = $2::uuid
		WHERE delivery_id = $1`
	if _, err := r.db.Exec(ctx, query, deliveryID, changeRequestID); err != nil {
		return fmt.Errorf("github: link delivery %s: %w", deliveryID, err)
	}
	return nil
}

// RepoForAccount implements GithubSyncRepository.
func (r *githubSyncRepository) RepoForAccount(ctx context.Context, accountID string) (*RepoMapping, error) {
	const query = `
		SELECT agr.account_id::text, a.name, COALESCE(agr.credential_ref, ''),
		       agr.owner, agr.repository
		FROM account_github_repo agr
		JOIN account a ON a.id = agr.account_id
		WHERE agr.account_id = $1::uuid AND agr.is_active`

	var m RepoMapping
	err := r.db.QueryRow(ctx, query, accountID).Scan(
		&m.AccountID, &m.AccountName, &m.CredentialRef, &m.Owner, &m.Repository)
	if errors.Is(err, pgx.ErrNoRows) {
		// No mapping, or an inactive one. Both mean the same thing to a
		// caller: this account does not file issues anywhere.
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("github: repo for account %s: %w", accountID, err)
	}
	return &m, nil
}

// AccountForCase implements GithubSyncRepository.
func (r *githubSyncRepository) AccountForCase(ctx context.Context, caseID string) (string, error) {
	const query = `SELECT COALESCE(account_id::text, '') FROM work_item WHERE id = $1::uuid`
	var accountID string
	err := r.db.QueryRow(ctx, query, caseID).Scan(&accountID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("github: account for case %s: %w", caseID, err)
	}
	return accountID, nil
}

// SetCaseGithubIssueNumber implements GithubSyncRepository.
func (r *githubSyncRepository) SetCaseGithubIssueNumber(ctx context.Context, caseID string, issueNumber int) (bool, error) {
	// IS DISTINCT FROM, matching SetState: re-linking a case to the issue it
	// already points at should write nothing.
	const query = `
		UPDATE work_item
		SET github_issue_number = $2
		WHERE id = $1::uuid AND github_issue_number IS DISTINCT FROM $2`
	tag, err := r.db.Exec(ctx, query, caseID, issueNumber)
	if err != nil {
		return false, fmt.Errorf("github: link case %s to issue %d: %w", caseID, issueNumber, err)
	}
	return tag.RowsAffected() > 0, nil
}

// CaseByIssueNumber implements GithubSyncRepository.
func (r *githubSyncRepository) CaseByIssueNumber(ctx context.Context, accountID string, issueNumber int) (string, error) {
	// Scoped to the account as well as the issue number: two accounts can each
	// have an issue #23, in different repositories.
	const query = `
		SELECT wi.id::text
		FROM work_item wi
		WHERE wi.github_issue_number = $2 AND wi.account_id = $1::uuid`
	var id string
	err := r.db.QueryRow(ctx, query, accountID, issueNumber).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("github: case for issue %d: %w", issueNumber, err)
	}
	return id, nil
}
