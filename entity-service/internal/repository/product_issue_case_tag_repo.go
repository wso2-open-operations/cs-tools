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

	"github.com/jackc/pgx/v5"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/validate"
)

// CaseTagAttachment is one tag row attached to a case.
type CaseTagAttachment struct {
	TagID     string
	CreatedBy string
}

// CaseTagRuleState is what the case tag rule decides from, read under the
// case's lock.
type CaseTagRuleState struct {
	// HasProductLink: the case has at least one AFFECTS link from a product
	// issue, active or not. Without one the rule never touches the case.
	HasProductLink bool
	// ActiveLinks counts the active AFFECTS links from product issues.
	ActiveLinks int
	// OpenIssues are the numbers of the linked (active) product issues whose
	// effective status is not RESOLVED or CLOSED, in number order.
	OpenIssues []string
	// Tags are the attachments of the tag, matched by name ignoring case.
	Tags []CaseTagAttachment
	// Recorded: case_tag_automation holds a row for the tag.
	Recorded bool
}

// CaseTagRuleTx is the rule's access to the database while it holds the case's
// lock; every call runs in the lock's transaction.
type CaseTagRuleTx interface {
	// State reads the case's state for the tag.
	State(ctx context.Context, tagLabel string) (CaseTagRuleState, error)
	// RecordAdd records that the system added the tag (an existing record is refreshed).
	RecordAdd(ctx context.Context, tagLabel string) error
	// ClearRecord forgets the record, if any.
	ClearRecord(ctx context.Context, tagLabel string) error
}

// ProductIssueCaseTagRepository backs the case tag rule (migration 0308).
type ProductIssueCaseTagRepository interface {
	// WithCaseLock runs fn holding a transaction-scoped advisory lock on the
	// case, so two runs of the rule for one case never interleave. fn's error
	// rolls the transaction back (and so any record it made).
	WithCaseLock(ctx context.Context, caseID string, fn func(ctx context.Context, tx CaseTagRuleTx) error) error
	// CaseIDsAffectedBy lists the cases a product issue has an AFFECTS link to,
	// active or not.
	CaseIDsAffectedBy(ctx context.Context, productIssueID string) ([]string, error)
}

type productIssueCaseTagRepo struct{ db *Scoped }

// NewProductIssueCaseTagRepository constructs a ProductIssueCaseTagRepository.
func NewProductIssueCaseTagRepository(db *Scoped) ProductIssueCaseTagRepository {
	return &productIssueCaseTagRepo{db: db}
}

// caseTagRuleLockQuery takes the per-case lock of the rule.
const caseTagRuleLockQuery = `SELECT pg_advisory_xact_lock(hashtextextended('case-tag-rule:' || $1::text, 0))`

// The effective status of a linked product issue, the same rule the SLA engine
// uses (productIssueState): CLOSED when GitHub says the issue is closed; open
// again when the board still says RESOLVED/CLOSED but a GitHub reopen is newer
// than the latest board move; else the board status. NULL (not on a board, or
// no GitHub row) counts as open.
const caseTagRuleOpenIssuesQuery = `
	SELECT pi.number
	  FROM work_item_link l
	  JOIN work_item pi ON pi.id = l.from_id AND pi.type = 'PRODUCT_ISSUE'
	  LEFT JOIN github_issue_link g ON g.work_item_id = pi.id
	 WHERE l.to_id = $1::uuid AND l.kind = 'AFFECTS' AND l.is_active
	   AND (
	        COALESCE(CASE WHEN g.github_state = 'CLOSED' THEN 'CLOSED' ELSE g.board_status END, 'OPEN')
	            NOT IN ('RESOLVED', 'CLOSED')
	     OR (g.github_state = 'OPEN' AND g.board_status IN ('RESOLVED', 'CLOSED')
	         AND (SELECT MAX(e.occurred_at) FROM work_item_status_event e
	               WHERE e.work_item_id = pi.id AND e.raw_status IS NULL AND e.to_status = 'REOPENED')
	             > COALESCE((SELECT MAX(e.occurred_at) FROM work_item_status_event e
	                          WHERE e.work_item_id = pi.id AND e.raw_status IS NOT NULL), '-infinity'::timestamptz)))
	 ORDER BY pi.number`

func (r *productIssueCaseTagRepo) WithCaseLock(ctx context.Context, caseID string, fn func(ctx context.Context, tx CaseTagRuleTx) error) error {
	// The rule's own writes (the record, the reads of internal-only product issue
	// rows) run as the system, whoever triggered it.
	ctx = WithSystemIdentity(ctx)
	return r.db.InTx(ctx, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, caseTagRuleLockQuery, caseID); err != nil {
			return fmt.Errorf("lock case for tag rule: %w", err)
		}
		return fn(ctx, caseTagRuleTx{tx: tx, caseID: caseID})
	})
}

func (r *productIssueCaseTagRepo) CaseIDsAffectedBy(ctx context.Context, productIssueID string) ([]string, error) {
	if !validate.IsUUID(productIssueID) {
		return nil, nil
	}
	ctx = WithSystemIdentity(ctx)
	rows, err := r.db.Query(ctx, `
		SELECT DISTINCT l.to_id::TEXT FROM work_item_link l
		 WHERE l.from_id = $1::uuid AND l.kind = 'AFFECTS' ORDER BY 1`, productIssueID)
	if err != nil {
		return nil, fmt.Errorf("cases affected by product issue: %w", err)
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan case id: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

type caseTagRuleTx struct {
	tx     pgx.Tx
	caseID string
}

func (c caseTagRuleTx) State(ctx context.Context, tagLabel string) (CaseTagRuleState, error) {
	var st CaseTagRuleState
	err := c.tx.QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM work_item_link l JOIN work_item pi ON pi.id = l.from_id AND pi.type = 'PRODUCT_ISSUE'
		                WHERE l.to_id = $1::uuid AND l.kind = 'AFFECTS'),
		       (SELECT count(*) FROM work_item_link l JOIN work_item pi ON pi.id = l.from_id AND pi.type = 'PRODUCT_ISSUE'
		         WHERE l.to_id = $1::uuid AND l.kind = 'AFFECTS' AND l.is_active),
		       EXISTS (SELECT 1 FROM case_tag_automation WHERE case_id = $1::uuid AND tag_label = $2)`,
		c.caseID, tagLabel).Scan(&st.HasProductLink, &st.ActiveLinks, &st.Recorded)
	if err != nil {
		return st, fmt.Errorf("read case tag rule state: %w", err)
	}
	if st.HasProductLink {
		numbers, err := c.tx.Query(ctx, caseTagRuleOpenIssuesQuery, c.caseID)
		if err != nil {
			return st, fmt.Errorf("read open product issues: %w", err)
		}
		for numbers.Next() {
			var n string
			if err := numbers.Scan(&n); err != nil {
				numbers.Close()
				return st, fmt.Errorf("scan product issue number: %w", err)
			}
			st.OpenIssues = append(st.OpenIssues, n)
		}
		numbers.Close()
		if err := numbers.Err(); err != nil {
			return st, fmt.Errorf("read open product issues: %w", err)
		}
	}
	tags, err := c.tx.Query(ctx, `
		SELECT t.id::TEXT, wit.created_by
		  FROM work_item_tag wit JOIN tag t ON t.id = wit.tag_id
		 WHERE wit.work_item_id = $1::uuid AND LOWER(t.name) = LOWER($2)
		 ORDER BY wit.created_on, t.id`, c.caseID, tagLabel)
	if err != nil {
		return st, fmt.Errorf("read case tags: %w", err)
	}
	defer tags.Close()
	for tags.Next() {
		var a CaseTagAttachment
		var by *string
		if err := tags.Scan(&a.TagID, &by); err != nil {
			return st, fmt.Errorf("scan case tag: %w", err)
		}
		if by != nil {
			a.CreatedBy = *by
		}
		st.Tags = append(st.Tags, a)
	}
	return st, tags.Err()
}

func (c caseTagRuleTx) RecordAdd(ctx context.Context, tagLabel string) error {
	if _, err := c.tx.Exec(ctx, `
		INSERT INTO case_tag_automation (case_id, tag_label) VALUES ($1::uuid, $2)
		ON CONFLICT (case_id, tag_label) DO UPDATE SET added_on = NOW()`, c.caseID, tagLabel); err != nil {
		return fmt.Errorf("record automatic tag: %w", err)
	}
	return nil
}

func (c caseTagRuleTx) ClearRecord(ctx context.Context, tagLabel string) error {
	if _, err := c.tx.Exec(ctx, `DELETE FROM case_tag_automation WHERE case_id = $1::uuid AND tag_label = $2`, c.caseID, tagLabel); err != nil {
		return fmt.Errorf("clear automatic tag record: %w", err)
	}
	return nil
}
