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

// This file holds the pure, DB/network-free planning logic that turns a
// resolved SN kb_knowledge row (plus already-resolved user/case ids) into
// the exact row values to insert -- kept separate from migrate.go's actual
// DB IO so it can be unit tested without a database at all.
package main

import (
	"fmt"
	"time"
)

// articlePlan is the fully-resolved set of values to INSERT into
// kb_articles for one chain's "latest=true" row.
type articlePlan struct {
	SourceSysID      string
	KnowledgeBaseID  string
	Title            string
	Body             string
	State            string
	AuthorID         string
	UpdatedBy        *string
	SourceCaseID     *string
	RejectionComment *string
	GeneratedWithAI  bool
	AIGeneratedBy    *string
	HelpfulCount     int
	Rating           *float64
	UseCount         int
	ViewCount        int
	SubmittedAt      *time.Time
	PublishedAt      *time.Time
	RetiredAt        *time.Time
}

// historyPlan is the fully-resolved set of values to INSERT into
// kb_article_history for one chain row visited by walkVersionChain.
type historyPlan struct {
	SourceSysID     string
	Title           string
	Body            string
	State           string
	ChangedBy       string
	GeneratedWithAI bool
	AIGeneratedBy   *string
}

// nilIfEmpty returns nil for an empty string, else a pointer to it -- for
// optional TEXT columns fed from SN choice/reference fields that come back
// as "" rather than actually absent.
func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// buildArticlePlan maps one "latest=true" kb_knowledge row plus its
// already-resolved knowledge_base_id/author_id/updated_by/source_case_id
// into the exact kb_articles row to insert. Returns an error only for an
// unrecognised workflow_state (collapseWorkflowState) -- every other input
// is assumed already validated/resolved by the caller.
func buildArticlePlan(row snKnowledge, knowledgeBaseID, authorID string, updatedBy, sourceCaseID *string) (articlePlan, error) {
	state, err := collapseWorkflowState(row.WorkflowState)
	if err != nil {
		return articlePlan{}, fmt.Errorf("article %s: %w", row.SysID, err)
	}
	d := computeArticleDefaults(state, row.CreatedOn, row.UpdatedOn, row.RejectReason)

	return articlePlan{
		SourceSysID:      row.SysID,
		KnowledgeBaseID:  knowledgeBaseID,
		Title:            row.ShortDescription,
		Body:             row.Text,
		State:            state,
		AuthorID:         authorID,
		UpdatedBy:        updatedBy,
		SourceCaseID:     sourceCaseID,
		RejectionComment: d.RejectionComment,
		GeneratedWithAI:  row.GeneratedWithNowAssist,
		AIGeneratedBy:    nilIfEmpty(row.GeneratedBy),
		HelpfulCount:     row.HelpfulCount,
		Rating:           row.Rating,
		UseCount:         row.UseCount,
		ViewCount:        row.ViewCount,
		SubmittedAt:      d.SubmittedAt,
		PublishedAt:      d.PublishedAt,
		RetiredAt:        d.RetiredAt,
	}, nil
}

// historyChangedBySource picks which SN sys_user reference a given
// kb_article_history row's changed_by is resolved from.
//
// Not explicitly pinned down by the migration spec -- an interpretation
// flagged in this tool's return summary rather than guessed silently: each
// chain row (like the latest row) carries its own author/revised_by pair,
// and revised_by is the more specific "who touched this particular
// revision" signal, so it wins when present; a row that was never revised
// past its own creation (revised_by empty) falls back to its author, since
// changed_by is NOT NULL and must resolve to *someone*.
func historyChangedBySource(row snKnowledge) string {
	if row.RevisedBy != "" {
		return row.RevisedBy
	}
	return row.Author
}

// buildHistoryPlan maps one historical kb_knowledge chain row plus its
// already-resolved changed_by into the exact kb_article_history row to
// insert. Returns an error only for an unrecognised workflow_state.
func buildHistoryPlan(row snKnowledge, changedBy string) (historyPlan, error) {
	state, err := collapseWorkflowState(row.WorkflowState)
	if err != nil {
		return historyPlan{}, fmt.Errorf("history row %s: %w", row.SysID, err)
	}
	return historyPlan{
		SourceSysID:     row.SysID,
		Title:           row.ShortDescription,
		Body:            row.Text,
		State:           state,
		ChangedBy:       changedBy,
		GeneratedWithAI: row.GeneratedWithNowAssist,
		AIGeneratedBy:   nilIfEmpty(row.GeneratedBy),
	}, nil
}
