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

package acp

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
)

// ReopenChecker is the entity-service call the daily reopen check makes:
// POST /acp/reopen-check, one page. Satisfied by *entity.Client.
type ReopenChecker interface {
	ReopenCheck(ctx context.Context, body []byte) ([]byte, error)
}

// reopenCheckPageSize is how many projects one call checks; small, because
// every project costs a few Salesforce calls and a page must finish within
// entity-service's request timeout.
const reopenCheckPageSize = 5

// reopenCheckMaxPages stops a run whose pages never end (an upstream that
// keeps answering with a cursor): 10,000 projects at the page size.
const reopenCheckMaxPages = 2000

type reopenCheckRequest struct {
	Cursor string `json:"cursor,omitempty"`
	Limit  int    `json:"limit"`
}

type reopenCheckResponse struct {
	Checked          int      `json:"checked"`
	Reopened         int      `json:"reopened"`
	Finished         int      `json:"finished"`
	Failed           int      `json:"failed"`
	FailedProjectIDs []string `json:"failedProjectIds"`
	NextCursor       string   `json:"nextCursor"`
	More             bool     `json:"more"`
}

// RunReopenCheck returns the acp_reopen_check handler: entity-service's daily
// safety net for ACP suspensions, page by page while entity-service says
// there is more (a page short of time stops early with more set).
// entity-service reopens what an event missed and finishes a project a person
// set back to Open (its CLAUDE.md, "ACP reopen"); this only drives it. A run
// in which a project failed returns an error naming it (the engine alerts and
// retries; a project already done has nothing left to do).
func RunReopenCheck(client ReopenChecker) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		var total reopenCheckResponse
		cursor := ""
		for page := 0; ; page++ {
			if page >= reopenCheckMaxPages {
				return fmt.Errorf("acp reopen check: still more pages after %d; stopping", reopenCheckMaxPages)
			}
			body, err := json.Marshal(reopenCheckRequest{Cursor: cursor, Limit: reopenCheckPageSize})
			if err != nil {
				return fmt.Errorf("acp reopen check: build request: %w", err)
			}
			raw, err := client.ReopenCheck(ctx, body)
			if err != nil {
				return fmt.Errorf("acp reopen check: page after %q: %w", cursor, err)
			}
			var r reopenCheckResponse
			if err := json.Unmarshal(raw, &r); err != nil {
				return fmt.Errorf("acp reopen check: parse page after %q: %w", cursor, err)
			}
			total.Checked += r.Checked
			total.Reopened += r.Reopened
			total.Finished += r.Finished
			total.Failed += r.Failed
			total.FailedProjectIDs = append(total.FailedProjectIDs, r.FailedProjectIDs...)
			if !r.More && r.NextCursor == "" {
				break
			}
			if r.Checked == 0 && (r.NextCursor == cursor || r.NextCursor == "") {
				return fmt.Errorf("acp reopen check: no progress after %q (entity-service ran out of time before the first project)", cursor)
			}
			cursor = r.NextCursor
		}
		slog.InfoContext(ctx, "acp reopen check: finished",
			"checked", total.Checked, "reopened", total.Reopened, "finished", total.Finished, "failed", total.Failed)
		if total.Failed == 0 {
			return nil
		}
		ids := total.FailedProjectIDs[:min(len(total.FailedProjectIDs), maxFailedIDsInError)]
		more := ""
		if n := len(total.FailedProjectIDs) - len(ids); n > 0 {
			more = fmt.Sprintf(" and %d more", n)
		}
		return fmt.Errorf("acp reopen check: %d project(s) failed: %s%s; see entity-service's log for each error",
			total.Failed, strings.Join(ids, ", "), more)
	}
}
