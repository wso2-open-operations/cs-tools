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

package main

import (
	"fmt"
	"time"
)

// Target kb_article_state_enum values (migrations/000020_create_kb_articles.up.sql).
const (
	stateDraft         = "draft"
	statePendingReview = "pending_review"
	statePublished     = "published"
	stateRetired       = "retired"
)

// workflowStateCollapse maps ServiceNow kb_knowledge.workflow_state raw
// values (confirmed live against the real sys_choice list on this field) to
// this platform's kb_article_state_enum. u_state is a separate,
// AI-review-outcome field (rejected/modified/not_modified) and is
// deliberately not read or mapped here at all.
var workflowStateCollapse = map[string]string{
	"draft":              stateDraft,
	"review":             statePendingReview,
	"scheduled_publish":  statePublished,
	"published":          statePublished,
	"pending_retirement": stateRetired,
	"retired":            stateRetired,
	"outdated":           stateRetired,
}

// collapseWorkflowState maps a raw SN workflow_state value to the target
// enum. An unrecognised value is a row-level error (per the spec's general
// rule for unresolvable data): the caller should skip that specific
// kb_knowledge row rather than guess at a state, since guessing wrong could
// violate a CHECK constraint or silently mis-categorize a live article.
func collapseWorkflowState(raw string) (string, error) {
	if collapsed, ok := workflowStateCollapse[raw]; ok {
		return collapsed, nil
	}
	return "", fmt.Errorf("unrecognised workflow_state %q", raw)
}

// articleDefaults holds the constraint-satisfying default values computed
// for a single kb_articles row (or, for generated_with_ai/ai_generated_by,
// also reused per kb_article_history row -- see computeHistoryAIFields)
// from its collapsed state and the source SN row's own timestamps/fields.
// Mirrors kb_articles' CHECK constraints exactly (migrations/000020,
// 000026-000028):
//   - chk_submitted_at_on_or_after_pending: submitted_at required unless draft
//   - chk_published_at_on_published: published_at required when published
//   - chk_retired_at_on_retired: retired_at required when retired
//   - chk_rejection_comment_only_after_reject: rejection_comment only allowed
//     when state = draft
type articleDefaults struct {
	SubmittedAt      *time.Time
	PublishedAt      *time.Time
	RetiredAt        *time.Time
	RejectionComment *string
}

// computeArticleDefaults derives the CHECK-constraint-satisfying default
// fields for a kb_articles row given its already-collapsed state and the
// originating SN row's sys_created_on/sys_updated_on/u_reject_reason.
//
// Per the spec (Step 5):
//   - submitted_at = sys_created_on when NOT draft, else nil
//   - published_at = sys_updated_on when published, else nil
//   - retired_at   = sys_updated_on when retired, else nil
//   - rejection_comment = u_reject_reason when draft (and non-empty), else nil
//     -- even if SN's u_reject_reason carries a stale non-empty value on a
//     non-draft row, it is intentionally dropped, not migrated.
func computeArticleDefaults(state string, createdOn, updatedOn time.Time, rejectReason string) articleDefaults {
	var d articleDefaults
	if state != stateDraft {
		t := createdOn
		d.SubmittedAt = &t
	}
	if state == statePublished {
		t := updatedOn
		d.PublishedAt = &t
	}
	if state == stateRetired {
		t := updatedOn
		d.RetiredAt = &t
	}
	if state == stateDraft && rejectReason != "" {
		r := rejectReason
		d.RejectionComment = &r
	}
	return d
}
