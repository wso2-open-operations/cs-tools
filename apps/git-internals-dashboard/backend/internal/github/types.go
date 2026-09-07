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

// Package github is the minimal GitHub GraphQL client shared by the seed and
// the incremental sync (SPEC §5, port of v3's src/server/db/github/client.ts):
//
//   - FetchRepoIssues — open issues + issues closed within a lookback window
//   - FetchIssueDetail — per-issue Status-change timeline + current Status,
//     the latter scoped to the configured project id
//
// PRIVACY: these queries deliberately do NOT request titles, assignees, or
// event actors. Labels are requested only so the ingest pipeline can derive
// priority, and are discarded after extraction (SPEC's non-negotiable #1).
//
// Client is defined as an interface so internal/sync and cmd/seed can be
// tested against a stub rather than live GitHub (SPEC: "single GITHUB_TOKEN
// (D3). Define the client as an interface consumed by sync/seed so tests can
// stub it.").
package github

import "context"

// IssueNode is one issue returned by a search query.
type IssueNode struct {
	Number    int
	State     string // "OPEN" | "CLOSED"
	URL       string
	CreatedAt string
	UpdatedAt string
	ClosedAt  *string
	Labels    []string
}

// StatusEvent is one Projects(v2) status-change timeline entry.
type StatusEvent struct {
	CreatedAt      string
	PreviousStatus *string
	Status         *string
}

// ProjectStatus is an issue's current status within one project board.
type ProjectStatus struct {
	ProjectID       string
	Status          *string
	StatusUpdatedAt *string
	ItemCreatedAt   *string // board-add time; assumed to resolve
}

// IssueDetail is one issue's full status timeline plus its current status in
// every project it sits in.
type IssueDetail struct {
	Number          int
	Events          []StatusEvent // ascending by CreatedAt
	ProjectStatuses []ProjectStatus
}

// Client is the GitHub GraphQL surface the sync and seed pipelines need.
type Client interface {
	// SearchAll runs q against GitHub's issue search, paginating until
	// exhausted. Exposed directly for the incremental sync, which composes
	// its own "updated:>=" query rather than FetchRepoIssues's separate
	// open/closed queries.
	SearchAll(ctx context.Context, q string) ([]IssueNode, error)

	// FetchRepoIssues returns every open issue matching issueQuery, plus
	// issues closed within closedLookbackDays, deduplicated by issue number
	// (open wins if both appear).
	FetchRepoIssues(ctx context.Context, owner, name, issueQuery string, closedLookbackDays int) ([]IssueNode, error)

	// FetchIssueDetail returns one issue's status timeline and per-project
	// current status, or (nil, nil) if the issue does not exist.
	FetchIssueDetail(ctx context.Context, owner, name string, number int) (*IssueDetail, error)
}
