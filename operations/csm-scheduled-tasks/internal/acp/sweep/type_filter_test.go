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

// typeJSON is the subscriptionType member of a project fixture: "absent" has
// none, "null" has an explicit null, anything else is the value.
func typeJSON(ty string) string {
	switch ty {
	case "absent":
		return ""
	case "null":
		return `,"subscriptionType":null`
	}
	return `,"subscriptionType":"` + ty + `"`
}

// typeListing lists one project per entry of types, p1, p2, ..., each due the
// 90-day notice.
func typeListing(now time.Time, types []string) *mockEntityReader {
	end := now.AddDate(0, 0, 89).Format(time.RFC3339)
	var parts []string
	for i, ty := range types {
		parts = append(parts, fmt.Sprintf(`{"suspensionProcessOverridden": false, "id":"p%d","endDate":"%s"%s}`, i+1, end, typeJSON(ty)))
	}
	body := `{"projects":[` + strings.Join(parts, ",") + `],"total":` + fmt.Sprint(len(types)) + `,"hasMore":false}`
	return &mockEntityReader{searchProjectsFn: func(context.Context, []byte) ([]byte, error) { return []byte(body), nil }}
}

func TestRun_SelectionFilterByProjectType(t *testing.T) {
	now := time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name string
		ty   string
		// want: "acted", "ineligible" or "unknown"
		want string
	}{
		{"subscription", "subscription", "acted"},
		{"managed cloud subscription", "managed_cloud_subscription", "acted"},
		{"development support", "development_support", "acted"},
		{"platformer subscription", "platformer_subscription", "acted"},
		{"professional services", "professional_services", "acted"},
		{"case and spaces", "  Subscription ", "acted"},
		{"evaluation subscription", "evaluation_subscription", "ineligible"},
		{"cloud support", "cloud_support", "ineligible"},
		{"cloud evaluation support", "cloud_evaluation_support", "ineligible"},
		{"internal", "internal", "ineligible"},
		{"no type", "absent", "unknown"},
		{"null type", "null", "unknown"},
		{"empty type", "", "unknown"},
		{"blank type", "   ", "unknown"},
		{"unrecognised type", "some_future_type", "unknown"},
	} {
		for _, scoped := range []bool{false, true} {
			name := tc.name
			if scoped {
				name += " (single project)"
			}
			t.Run(name, func(t *testing.T) {
				reader := typeListing(now, []string{tc.ty})
				id := ""
				if scoped {
					id = "p1"
				}
				end := now.AddDate(0, 0, 89).Format(time.RFC3339)
				reader.getProjectFn = func(context.Context, string) ([]byte, error) {
					return []byte(`{"suspensionProcessOverridden": false, "id":"p1","endDate":"` + end + `"` + typeJSON(tc.ty) + `}`), nil
				}
				ntf, updater := delivering(), &mockProjectUpdater{}
				result, err := Run(context.Background(), reader, CSMDetails(reader), updater, ntf, now, id, nil, 0)
				if err != nil || len(result.Failures) != 0 {
					t.Fatalf("Run() err=%v failures=%d, want nil/0", err, len(result.Failures))
				}
				var wantActed, wantIneligible, wantUnknown int
				switch tc.want {
				case "acted":
					wantActed = 1
				case "ineligible":
					wantIneligible = 1
				case "unknown":
					wantUnknown = 1
				}
				if result.ProjectsActed != wantActed || result.ProjectsFiltered != wantIneligible || result.ProjectsFilteredUnknownType != wantUnknown {
					t.Errorf("acted=%d ineligible=%d unknownType=%d, want %d/%d/%d", result.ProjectsActed, result.ProjectsFiltered, result.ProjectsFilteredUnknownType, wantActed, wantIneligible, wantUnknown)
				}
				if wantActed == 0 && (len(ntf.sent) != 0 || len(updater.calls) != 0) {
					t.Errorf("sent=%d writes=%d, want none for a skipped project", len(ntf.sent), len(updater.calls))
				}
				if wantActed == 1 && len(ntf.sent) == 0 {
					t.Errorf("no notice sent for an eligible type")
				}
			})
		}
	}
}

// Projects with a missing or unrecognised type are skipped without a notice,
// a suspension or a write, are counted, and do not consume ACP_DAILY_LIMIT.
func TestRun_UnknownTypeDoesNotConsumeTheLimit(t *testing.T) {
	now := time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC)
	types := []string{"absent", "", "some_future_type", "subscription", "internal", "managed_cloud_subscription", "subscription"}
	reader := typeListing(now, types)
	end := now.AddDate(0, 0, 89).Format(time.RFC3339)
	reader.getProjectFn = func(_ context.Context, id string) ([]byte, error) {
		var n int
		fmt.Sscanf(id, "p%d", &n)
		return []byte(`{"suspensionProcessOverridden": false, "id":"` + id + `","endDate":"` + end + `"` + typeJSON(types[n-1]) + `}`), nil
	}
	ntf, updater := delivering(), &mockProjectUpdater{}
	result, err := Run(context.Background(), reader, CSMDetails(reader), updater, ntf, now, "", nil, 2)
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	if result.ProjectsFilteredUnknownType != 3 || result.ProjectsFiltered != 1 || result.ProjectsActed != 2 || result.NotEvaluatedByLimit != 1 {
		t.Errorf("unknownType=%d ineligible=%d acted=%d notEvaluated=%d, want 3/1/2/1", result.ProjectsFilteredUnknownType, result.ProjectsFiltered, result.ProjectsActed, result.NotEvaluatedByLimit)
	}
	for _, n := range ntf.sent {
		switch n.ProjectID {
		case "p4", "p6":
		default:
			t.Errorf("notice sent for %s, which is skipped or past the limit", n.ProjectID)
		}
	}
}

func TestSkipIneligible_UnknownTypeIsCounted(t *testing.T) {
	var r Result
	if !skipIneligible(project{SuspensionProcessOverridden: boolPtr(false)}, &r) || r.ProjectsFilteredUnknownType != 1 || r.ProjectsFiltered != 0 {
		t.Errorf("no type: result=%+v, want skipped as unknown type", r)
	}
	if skipIneligible(project{SubscriptionType: sp("subscription"), SuspensionProcessOverridden: boolPtr(false)}, &r) {
		t.Errorf("an eligible type was skipped")
	}
}

// Screening only chooses who to re-read; a screened-in project with no type is
// still skipped from the re-read, and counted.
func TestRunWith_ScreenedInProjectWithNoTypeIsSkipped(t *testing.T) {
	now := time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC)
	reader := typeListing(now, []string{"absent"})
	end := now.AddDate(0, 0, 89).Format(time.RFC3339)
	reader.getProjectFn = func(context.Context, string) ([]byte, error) {
		return []byte(`{"suspensionProcessOverridden": false, "id":"p1","endDate":"` + end + `"}`), nil
	}
	ntf, updater := delivering(), &mockProjectUpdater{}
	result, err := RunWith(context.Background(), reader, CSMDetails(reader), updater, ntf, now, "", nil, Options{Screening: true})
	if err != nil || result.ProjectsFilteredUnknownType != 1 || result.ProjectsActed != 0 || len(ntf.sent) != 0 || len(updater.calls) != 0 {
		t.Errorf("err=%v result=%+v sent=%d writes=%d, want one unknown-type skip and nothing sent", err, result, len(ntf.sent), len(updater.calls))
	}
}
