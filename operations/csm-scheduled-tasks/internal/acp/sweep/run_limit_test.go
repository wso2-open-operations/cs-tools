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

package sweep

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
)

// listedForLimit lists one project per entry of acting, in that order: true is
// due the 90-day notice at now, false is not due any window (end date a year
// out).
func listedForLimit(now time.Time, acting []bool) *mockEntityReader {
	var parts []string
	for i, a := range acting {
		end := now.AddDate(1, 0, 0)
		if a {
			end = now.AddDate(0, 0, 89)
		}
		parts = append(parts, fmt.Sprintf(`{"id":"p%d","endDate":"%s","closureState":"Open"}`, i+1, end.Format(time.RFC3339)))
	}
	body := `{"projects":[` + strings.Join(parts, ",") + `],"total":` + fmt.Sprint(len(acting)) + `,"hasMore":false}`
	return &mockEntityReader{searchProjectsFn: func(context.Context, []byte) ([]byte, error) { return []byte(body), nil }}
}

func TestRun_LimitStopsAfterNActingProjectsInOrder(t *testing.T) {
	now := time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name            string
		acting          []bool
		limit           int
		wantActed       int
		wantEvaluated   int
		wantNotEval     int
		wantNotifiedIDs []string
	}{
		{"no cap", []bool{true, true, true}, 0, 3, 3, 0, []string{"p1", "p2", "p3"}},
		{"cap takes the first", []bool{true, true, true}, 2, 2, 2, 1, []string{"p1", "p2"}},
		{"non-acting projects do not consume the cap", []bool{false, true, false, true, true}, 2, 2, 4, 1, []string{"p2", "p4"}},
		{"cap above the acting count", []bool{true, false}, 5, 1, 2, 0, []string{"p1"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reader := listedForLimit(now, tc.acting)
			ntf := delivering()
			result, err := Run(context.Background(), reader, CSMDetails(reader), &mockProjectUpdater{}, ntf, now, "", nil, tc.limit)
			if err != nil {
				t.Fatalf("Run() error = %v, want nil: the cap is not a failure", err)
			}
			if result.ProjectsActed != tc.wantActed || result.ProjectsEvaluated != tc.wantEvaluated || result.NotEvaluatedByLimit != tc.wantNotEval {
				t.Errorf("acted=%d evaluated=%d notEvaluated=%d, want %d/%d/%d", result.ProjectsActed, result.ProjectsEvaluated, result.NotEvaluatedByLimit, tc.wantActed, tc.wantEvaluated, tc.wantNotEval)
			}
			var got []string
			seen := map[string]bool{}
			for _, n := range ntf.sent {
				if !seen[n.ProjectID] {
					seen[n.ProjectID] = true
					got = append(got, n.ProjectID)
				}
			}
			if strings.Join(got, ",") != strings.Join(tc.wantNotifiedIDs, ",") {
				t.Errorf("notified %v, want %v", got, tc.wantNotifiedIDs)
			}
			if len(result.Failures) != 0 {
				t.Errorf("failures = %+v, want none", result.Failures)
			}
		})
	}
}

// A dry run counts the same way, so it shows what a capped real run does.
func TestRun_LimitCountsInADryRun(t *testing.T) {
	now := time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC)
	reader := listedForLimit(now, []bool{true, true, true})
	result, err := Run(context.Background(), reader, CSMDetails(reader), &DryRunProjectUpdater{}, &mockNotifier{}, now, "", nil, 1)
	if err != nil || result.ProjectsActed != 1 || result.NotEvaluatedByLimit != 2 {
		t.Errorf("err=%v acted=%d notEvaluated=%d, want nil/1/2", err, result.ProjectsActed, result.NotEvaluatedByLimit)
	}
}
