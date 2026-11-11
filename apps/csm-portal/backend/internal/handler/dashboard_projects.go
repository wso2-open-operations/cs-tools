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

package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

// errDashboardProject marks a generated project filter naming no project, or
// several: the handler's 422 with the reason (generate_project).
var errDashboardProject = errors.New("unusable project filter")

// errDashboardProjectLookup marks a projects search that failed while the
// names were being checked: the handler's 502.
var errDashboardProjectLookup = errors.New("project lookup failed")

// errMsgGenerateProjectLookup is errDashboardProjectLookup's message.
const errMsgGenerateProjectLookup = "The project names in the dashboard couldn't be checked. Try again."

// projectSearcher is the entity service's POST /projects/search
// (entity.CustomerEntityClient.SearchProjects): a project's name or key
// containing the search query, ignoring case, among the projects the caller
// can see.
type projectSearcher interface {
	SearchProjects(ctx context.Context, body []byte) ([]byte, error)
}

// searchedProject is one project of a projects search.
type searchedProject struct {
	ID   string `json:"id"`
	Key  string `json:"key"`
	Name string `json:"name"`
}

// WithDashboardProjects wraps a generator so that each "project" filter (a
// project's key or name, as the user wrote it) becomes a "projectId" filter
// with the same op ("in", or "notIn" to leave the projects out) on the id of
// the one project each value names (namedResolver.resolveValue: an exact key
// wins, then an exact name); a value naming no project, or several, is an
// errDashboardProject, so a project left out by a name that matches nothing
// never silently leaves out nothing.
func WithDashboardProjects(inner DashboardGenerator, projects projectSearcher) DashboardGenerator {
	return &namedResolvingGenerator{inner: inner, kind: namedKind{
		field: "project", idField: "projectId", noun: "project", plural: "projects",
		unusable: errDashboardProject, lookup: errDashboardProjectLookup,
		search: func(ctx context.Context, written string) ([]namedRecord, bool, error) {
			body, err := json.Marshal(map[string]any{
				"searchQuery": written,
				"pagination":  map[string]int{"limit": namedSearchLimit, "offset": 0},
			})
			if err != nil {
				return nil, false, err
			}
			raw, err := projects.SearchProjects(ctx, body)
			if err != nil {
				return nil, false, err
			}
			var res struct {
				Projects []searchedProject `json:"projects"`
				HasMore  bool              `json:"hasMore"`
			}
			if err := json.Unmarshal(raw, &res); err != nil {
				return nil, false, fmt.Errorf("read the projects search: %w", err)
			}
			records := make([]namedRecord, len(res.Projects))
			for i, p := range res.Projects {
				records[i] = namedRecord{ID: p.ID, Code: p.Key, Name: p.Name}
			}
			return records, res.HasMore, nil
		},
	}}
}
