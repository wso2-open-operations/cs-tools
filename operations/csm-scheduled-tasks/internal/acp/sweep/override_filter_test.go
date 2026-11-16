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

func boolPtr(b bool) *bool { return &b }

// overrideListing lists projects due the 90-day notice, one per entry of
// flags: "true", "false" or "absent" for suspensionProcessOverridden.
func overrideListing(now time.Time, flags []string) *mockEntityReader {
	end := now.AddDate(0, 0, 89).Format(time.RFC3339)
	var parts []string
	for i, f := range flags {
		field := ""
		if f != "absent" {
			field = `,"suspensionProcessOverridden":` + f
		}
		parts = append(parts, fmt.Sprintf(`{"id":"p%d","endDate":"%s"%s}`, i+1, end, field))
	}
	body := `{"projects":[` + strings.Join(parts, ",") + `],"total":` + fmt.Sprint(len(flags)) + `,"hasMore":false}`
	return &mockEntityReader{searchProjectsFn: func(context.Context, []byte) ([]byte, error) { return []byte(body), nil }}
}

func TestRun_OverrideFlagFilter(t *testing.T) {
	now := time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name                                                string
		flags                                               []string
		limit                                               int
		wantActed, wantOverridden, wantUnknown, wantNotEval int
		wantNotified                                        []string
	}{
		{"false is evaluated", []string{"false"}, 0, 1, 0, 0, 0, []string{"p1"}},
		{"true is skipped", []string{"true"}, 0, 0, 1, 0, 0, nil},
		{"absent is skipped as unknown", []string{"absent"}, 0, 0, 0, 1, 0, nil},
		{"mixed, counted separately", []string{"true", "false", "absent", "false", "true"}, 0, 2, 2, 1, 0, []string{"p2", "p4"}},
		{"skipped projects do not use the cap", []string{"true", "absent", "false", "false", "false"}, 1, 1, 1, 1, 2, []string{"p3"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reader := overrideListing(now, tc.flags)
			ntf, updater := delivering(), &mockProjectUpdater{}
			result, err := Run(context.Background(), reader, CSMDetails(reader), updater, ntf, now, "", nil, tc.limit)
			if err != nil || len(result.Failures) != 0 {
				t.Fatalf("Run = %+v, %v", result, err)
			}
			if result.ProjectsActed != tc.wantActed || result.ProjectsFilteredOverridden != tc.wantOverridden ||
				result.ProjectsFilteredOverrideUnknown != tc.wantUnknown || result.NotEvaluatedByLimit != tc.wantNotEval {
				t.Errorf("acted=%d overridden=%d unknown=%d notEvaluated=%d, want %d/%d/%d/%d", result.ProjectsActed,
					result.ProjectsFilteredOverridden, result.ProjectsFilteredOverrideUnknown, result.NotEvaluatedByLimit,
					tc.wantActed, tc.wantOverridden, tc.wantUnknown, tc.wantNotEval)
			}
			seen := map[string]bool{}
			var got []string
			for _, n := range ntf.sent {
				if !seen[n.ProjectID] {
					seen[n.ProjectID] = true
					got = append(got, n.ProjectID)
				}
			}
			if strings.Join(got, ",") != strings.Join(tc.wantNotified, ",") {
				t.Errorf("notified %v, want %v", got, tc.wantNotified)
			}
			if len(tc.wantNotified) == 0 && (len(updater.calls) != 0 || len(updater.checks) != 0) {
				t.Errorf("writes=%d probes=%d, want none for skipped projects", len(updater.calls), len(updater.checks))
			}
		})
	}
}

// The single-project path decides from the same fresh read.
func TestRun_OverrideFlagFilterSingleProject(t *testing.T) {
	now := time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC)
	end := now.AddDate(0, 0, 89).Format(time.RFC3339)
	for _, tc := range []struct {
		flag                      string
		wantActed, wantOv, wantUn int
	}{{`,"suspensionProcessOverridden":false`, 1, 0, 0}, {`,"suspensionProcessOverridden":true`, 0, 1, 0}, {``, 0, 0, 1}} {
		reader := &mockEntityReader{getProjectFn: func(context.Context, string) ([]byte, error) {
			return []byte(`{"id":"p1","endDate":"` + end + `"` + tc.flag + `}`), nil
		}}
		ntf, updater := delivering(), &mockProjectUpdater{}
		result, err := Run(context.Background(), reader, CSMDetails(reader), updater, ntf, now, "p1", nil, 0)
		if err != nil || result.ProjectsActed != tc.wantActed || result.ProjectsFilteredOverridden != tc.wantOv || result.ProjectsFilteredOverrideUnknown != tc.wantUn {
			t.Errorf("%q: err=%v result=%+v", tc.flag, err, result)
		}
		if tc.wantActed == 0 && (len(ntf.sent) != 0 || len(updater.calls) != 0) {
			t.Errorf("%q: sent=%d writes=%d, want none", tc.flag, len(ntf.sent), len(updater.calls))
		}
	}
}

// The flag is read from the fresh re-read, not the listing: a project listed
// as not overridden but overridden since is skipped.
func TestRun_OverrideFlagComesFromTheReRead(t *testing.T) {
	now := time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC)
	reader := overrideListing(now, []string{"false"})
	end := now.AddDate(0, 0, 89).Format(time.RFC3339)
	reader.getProjectFn = func(context.Context, string) ([]byte, error) {
		return []byte(`{"id":"p1","endDate":"` + end + `","suspensionProcessOverridden":true}`), nil
	}
	ntf := delivering()
	result, err := Run(context.Background(), reader, CSMDetails(reader), &mockProjectUpdater{}, ntf, now, "", nil, 0)
	if err != nil || result.ProjectsFilteredOverridden != 1 || len(ntf.sent) != 0 {
		t.Errorf("err=%v result=%+v sent=%d, want skipped from the re-read", err, result, len(ntf.sent))
	}
}
