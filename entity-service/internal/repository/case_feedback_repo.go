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

// GetCaseFeedback/CreateCaseFeedback (CaseRepository, implemented here
// rather than inline in case_repo.go to keep that file from growing
// further) back GET/POST /cases/{id}/feedback on the Postgres data source --
// previously a hardcoded 503 ("case feedback is only supported for the
// ServiceNow data source"), now that work_item_feedback_metric/
// work_item_feedback_metric_option (migration 0127, synced from ServiceNow's
// asmt_metric/asmt_metric_definition) exist and are populated: the five
// "<rating> - Reasons" metric rows are exactly the five emoji choices the
// case feedback form offers, and work_item_feedback_metric_option's rows are
// their per-emoji reason checkboxes ("chips").
//
// work_item_feedback (migration 0102) has no column referencing which emoji/
// metric a submission picked -- only a plain rating (1-5) and rating_label
// (e.g. "Very Satisfied"). This mirrors the ServiceNow-synced shape exactly
// (rating/comment are "patched in" onto a pre-existing survey-instance row,
// per that migration's own doc comment) and is not something this change can
// alter. So the emoji is resolved by NAME, not by a stored foreign key: every
// "<rating> - Reasons" metric's name is, by construction (see the sync
// mapping's own value_map, operations/csm-sync-service's asmt_metric.yaml),
// exactly its clean rating label plus the fixed " - Reasons" suffix -- so
// rating_label + " - Reasons" always resolves back to the one metric row a
// submission's emojiId pointed at, with no ambiguity. caseFeedbackRatingByLabel
// is this package's one copy of that fixed, closed 5-value correspondence
// (also used by ReferenceDataRepository.ListFeedbackEmojis, GET /metadata's
// own feedbackEmojies source, so the two endpoints can never disagree on what
// a "rating" means).
package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
)

// caseFeedbackReasonsSuffix is the fixed suffix every rating's "reasons"
// question metric carries in its own name column, exactly matching the sync
// mapping's value_map keys (e.g. "Very Satisfied - Reasons").
const caseFeedbackReasonsSuffix = " - Reasons"

// caseFeedbackRatingByLabel is the closed, fixed 5-point scale this feedback
// form has always used -- the same five labels the sync mapping's own
// selected_image/unselected_image value_map keys off of (operations/
// csm-sync-service's asmt_metric.yaml), in the standard low-to-high Likert
// order. work_item_feedback_metric carries no numeric rating column of its
// own, so this is the only correspondence available between a metric's name
// and the 1-5 integer work_item_feedback.rating stores.
var caseFeedbackRatingByLabel = map[string]int{
	"Very Dissatisfied": 1,
	"Dissatisfied":      2,
	"Neutral":           3,
	"Satisfied":         4,
	"Very Satisfied":    5,
}

// CaseFeedbackRow is one case's previously-submitted emoji feedback --
// GetCaseFeedback's result.
type CaseFeedbackRow struct {
	ID                 string
	EmojiID            string
	EmojiName          string
	EmojiSelectedImage string
	ChipIDs            []string
	CreatedBy          string
	CreatedOn          string // RFC3339
	AdditionalComment  *string
}

// CreateCaseFeedbackParams is CreateCaseFeedback's input. SubmittedByUserID
// may be empty (work_item_feedback.submitted_by_id is nullable) when the
// caller's token didn't resolve to a known "user" row; ActorEmail is always
// required and becomes the row's created_by/updated_by audit columns, the
// same convention every other writer in this file follows.
type CreateCaseFeedbackParams struct {
	EmojiID           string
	ChipIDs           []string
	AdditionalComment *string
	SubmittedByUserID string
	ActorEmail        string
}

// CaseFeedbackCreated is CreateCaseFeedback's result.
type CaseFeedbackCreated struct {
	ID        string
	CreatedOn string // RFC3339
}

// resolveCaseFeedbackRating maps a work_item_feedback_metric's own name back
// to the (rating, cleanLabel) pair this package's fixed scale assigns it, or
// ok=false when the name doesn't end in caseFeedbackReasonsSuffix or its
// stripped label isn't one of the five known values -- defensive only; every
// row this schema's own sync seeds should always match.
func resolveCaseFeedbackRating(metricName string) (rating int, label string, ok bool) {
	label, ok = strings.CutSuffix(metricName, caseFeedbackReasonsSuffix)
	if !ok {
		return 0, "", false
	}
	rating, ok = caseFeedbackRatingByLabel[label]
	return rating, label, ok
}

// GetCaseFeedback implements CaseRepository. Only a row with a non-NULL
// rating counts as "submitted" -- see FeedbackRepository's own doc comment
// for why (a NULL rating is an issued-but-unanswered survey on the
// ServiceNow-synced side; this Postgres-native path never creates such a
// row at all, since CreateCaseFeedback always supplies a rating, but a
// dual-write/synced environment could still have one).
func (r *caseRepo) GetCaseFeedback(ctx context.Context, caseID string) (CaseFeedbackRow, bool, error) {
	var (
		id, ratingLabel string
		comment         *string
		createdByName   *string
		createdByEmail  *string
		createdBy       string
		createdOn       time.Time
	)
	err := r.db.QueryRow(ctx, `
		SELECT f.id::TEXT, f.rating_label, f.comment, f.created_by,
		       NULLIF(TRIM(CONCAT_WS(' ', u.first_name, u.last_name)), ''), u.email,
		       COALESCE(f.submitted_at, f.created_on)
		FROM work_item_feedback f
		JOIN work_item wi ON wi.id = f.work_item_id
		LEFT JOIN "user" u ON u.id = f.submitted_by_id
		WHERE f.work_item_id = $1 AND f.rating IS NOT NULL`,
		caseID,
	).Scan(&id, &ratingLabel, &comment, &createdBy, &createdByName, &createdByEmail, &createdOn)
	if errors.Is(err, pgx.ErrNoRows) {
		return CaseFeedbackRow{}, false, nil
	}
	if err != nil {
		return CaseFeedbackRow{}, false, fmt.Errorf("get case feedback: %w", err)
	}
	// createdBy (the audit column, an email) is the fallback for a row with
	// no resolvable submitter -- a ServiceNow-synced row with no linked
	// "user", or a submitted_by_id that no longer resolves. See
	// CreateCaseFeedback's own doc comment for why this Postgres-native path
	// always has a real actor and so always prefers the name/email above it.
	if createdByName != nil {
		createdBy = *createdByName
	} else if createdByEmail != nil {
		createdBy = *createdByEmail
	}

	var emojiID, emojiName, emojiImage *string
	err = r.db.QueryRow(ctx, `
		SELECT id::TEXT, name, selected_image
		FROM work_item_feedback_metric
		WHERE name = $1`,
		ratingLabel+caseFeedbackReasonsSuffix,
	).Scan(&emojiID, &emojiName, &emojiImage)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return CaseFeedbackRow{}, false, fmt.Errorf("resolve case feedback emoji: %w", err)
	}

	// A reason's own option_value/reason are kept as recorded with no FK to
	// work_item_feedback_metric_option (see migration 0128's own doc
	// comment), so a since-renamed-or-removed option has no current id to
	// report -- that reason is left out of chips entirely (a known,
	// accepted gap matching this codebase's "flag it, don't fabricate"
	// convention) rather than inventing one.
	rows, err := r.db.Query(ctx, `
		SELECT o.id::TEXT
		FROM work_item_feedback_reason r
		LEFT JOIN work_item_feedback_metric_option o ON o.metric_id = r.metric_id AND o.value = r.option_value
		WHERE r.feedback_id = $1
		ORDER BY r.created_on`,
		id,
	)
	if err != nil {
		return CaseFeedbackRow{}, false, fmt.Errorf("list case feedback reasons: %w", err)
	}
	defer rows.Close()

	chipIDs := []string{}
	for rows.Next() {
		var chipID *string
		if err := rows.Scan(&chipID); err != nil {
			return CaseFeedbackRow{}, false, fmt.Errorf("scan case feedback reason: %w", err)
		}
		if chipID != nil {
			chipIDs = append(chipIDs, *chipID)
		}
	}
	if err := rows.Err(); err != nil {
		return CaseFeedbackRow{}, false, fmt.Errorf("iterate case feedback reasons: %w", err)
	}

	return CaseFeedbackRow{
		ID:                 id,
		EmojiID:            stringOrEmpty(emojiID),
		EmojiName:          strings.TrimSuffix(stringOrEmpty(emojiName), caseFeedbackReasonsSuffix),
		EmojiSelectedImage: stringOrEmpty(emojiImage),
		ChipIDs:            chipIDs,
		CreatedBy:          createdBy,
		CreatedOn:          createdOn.UTC().Format(time.RFC3339),
		AdditionalComment:  comment,
	}, true, nil
}

// CreateCaseFeedback implements CaseRepository.
func (r *caseRepo) CreateCaseFeedback(ctx context.Context, caseID string, params CreateCaseFeedbackParams) (CaseFeedbackCreated, error) {
	return InTxReturning(ctx, r.db, func(tx pgx.Tx) (CaseFeedbackCreated, error) {
		return createCaseFeedbackTx(ctx, tx, caseID, params)
	})
}

func createCaseFeedbackTx(ctx context.Context, tx pgx.Tx, caseID string, params CreateCaseFeedbackParams) (CaseFeedbackCreated, error) {
	// The case must exist, be visible to this caller, and -- the actual
	// point of this feedback form -- already be CLOSED: it is a post-closure
	// satisfaction survey (the portal only ever offers it once a case has
	// closed), not a running commentary on an open one, so a submission
	// against a case that is still open is rejected rather than silently
	// accepted. Reuses the exact same state resolution GetCaseByID/
	// SearchCases use for every case-like type (case_repo.go's
	// caseLikeStateColumn/caseLikeJoins/caseLikeWorkItemTypes) so "closed"
	// can never be defined two different ways between this check and what
	// the case detail page itself shows. RLS on work_item is what enforces
	// "exists, just not yours" -> not found, the same posture every by-id
	// case read already has (see GetCaseByID's own scope parameter).
	var state string
	err := tx.QueryRow(ctx, `
		SELECT `+caseLikeStateColumn+`
		FROM work_item wi
		LEFT JOIN "case" c ON c.id = wi.id
		`+caseLikeJoins+`
		WHERE wi.id = $1 AND wi.type = ANY(`+caseLikeWorkItemTypes+`)`,
		caseID,
	).Scan(&state)
	if errors.Is(err, pgx.ErrNoRows) {
		return CaseFeedbackCreated{}, &apierror.NotFoundError{Msg: "case not found"}
	}
	if err != nil {
		return CaseFeedbackCreated{}, fmt.Errorf("verify case exists: %w", err)
	}
	if state != "CLOSED" {
		return CaseFeedbackCreated{}, &apierror.ConflictError{Msg: "feedback can only be submitted once the case is closed"}
	}

	// is_active AND selected_image IS NOT NULL is the exact definition
	// ListFeedbackEmojis itself uses for "a real catalog emoji" -- matched
	// here so a submission can never be accepted for an id GET /metadata
	// would never have offered as a choice in the first place (e.g. a
	// non-"<rating> - Reasons" row like "Experience"/"Additional Comments",
	// or a malformed future row missing its image).
	var metricName string
	if err := tx.QueryRow(ctx, `SELECT name FROM work_item_feedback_metric WHERE id = $1 AND is_active AND selected_image IS NOT NULL`, params.EmojiID).
		Scan(&metricName); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return CaseFeedbackCreated{}, &apierror.ValidationError{Msg: "emojiId does not refer to a known feedback rating"}
		}
		return CaseFeedbackCreated{}, fmt.Errorf("resolve feedback emoji: %w", err)
	}
	rating, ratingLabel, ok := resolveCaseFeedbackRating(metricName)
	if !ok {
		return CaseFeedbackCreated{}, &apierror.ValidationError{Msg: "emojiId does not refer to a known feedback rating"}
	}

	// Every chip must be one of this SAME emoji's own options -- a chip
	// id belonging to a different rating's question set is rejected, not
	// silently accepted, the same "scope every submitted id" discipline
	// this codebase applies to e.g. deploymentId on attachment updates.
	type chipOption struct {
		id, label string
		value     int
	}
	chips := make([]chipOption, 0, len(params.ChipIDs))
	for _, chipID := range params.ChipIDs {
		var c chipOption
		c.id = chipID
		if err := tx.QueryRow(ctx,
			`SELECT label, value FROM work_item_feedback_metric_option WHERE id = $1 AND metric_id = $2`,
			chipID, params.EmojiID,
		).Scan(&c.label, &c.value); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return CaseFeedbackCreated{}, &apierror.ValidationError{Msg: "chipIds contains an option that does not belong to emojiId: " + chipID}
			}
			return CaseFeedbackCreated{}, fmt.Errorf("resolve feedback chip %s: %w", chipID, err)
		}
		chips = append(chips, c)
	}

	var submittedByID *string
	if params.SubmittedByUserID != "" {
		submittedByID = &params.SubmittedByUserID
	}

	var id string
	var createdOn time.Time
	err = tx.QueryRow(ctx, `
		INSERT INTO work_item_feedback (id, work_item_id, rating, rating_label, comment, submitted_by_id, submitted_at,
		                                 created_on, updated_on, created_by, updated_by)
		VALUES (gen_random_uuid(), $1, $2, $3, $4, $5, NOW(), NOW(), NOW(), $6, $6)
		ON CONFLICT (work_item_id) DO NOTHING
		RETURNING id::TEXT, created_on`,
		caseID, rating, ratingLabel, params.AdditionalComment, submittedByID, params.ActorEmail,
	).Scan(&id, &createdOn)
	if errors.Is(err, pgx.ErrNoRows) {
		return CaseFeedbackCreated{}, &apierror.ConflictError{Msg: "feedback has already been submitted for this case"}
	}
	if err != nil {
		if pgErr := (*pgconn.PgError)(nil); errors.As(err, &pgErr) && pgErr.Code == "23503" {
			return CaseFeedbackCreated{}, &apierror.NotFoundError{Msg: "case not found"}
		}
		return CaseFeedbackCreated{}, fmt.Errorf("create case feedback: %w", err)
	}

	for _, c := range chips {
		if _, err := tx.Exec(ctx, `
			INSERT INTO work_item_feedback_reason (id, feedback_id, metric_id, option_value, reason,
			                                        created_on, updated_on, created_by, updated_by)
			VALUES (gen_random_uuid(), $1, $2, $3, $4, NOW(), NOW(), $5, $5)`,
			id, params.EmojiID, c.value, c.label, params.ActorEmail,
		); err != nil {
			return CaseFeedbackCreated{}, fmt.Errorf("create case feedback reason: %w", err)
		}
	}

	return CaseFeedbackCreated{ID: id, CreatedOn: createdOn.UTC().Format(time.RFC3339)}, nil
}
