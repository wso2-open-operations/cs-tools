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

func TestRun_SelectionFilterSkipsIneligibleProjectTypes(t *testing.T) {
	now := time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC)
	end := now.AddDate(0, 0, 89).Format(time.RFC3339)
	var parts []string
	types := []string{"subscription", "evaluation_subscription", "cloud_support", "cloud_evaluation_support", "internal", "managed_cloud_subscription", ""}
	for i, ty := range types {
		sub := ""
		if ty != "" {
			sub = `,"subscriptionType":"` + ty + `"`
		}
		parts = append(parts, fmt.Sprintf(`{"suspensionProcessOverridden": false, "id":"p%d","endDate":"%s"%s}`, i+1, end, sub))
	}
	body := `{"projects":[` + strings.Join(parts, ",") + `],"total":7,"hasMore":false}`
	reader := &mockEntityReader{searchProjectsFn: func(context.Context, []byte) ([]byte, error) { return []byte(body), nil }}
	ntf := delivering()
	result, err := Run(context.Background(), reader, CSMDetails(reader), &mockProjectUpdater{}, ntf, now, "", nil, 0)
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	if result.ProjectsFiltered != 4 || result.ProjectsActed != 3 || len(result.Failures) != 0 {
		t.Errorf("filtered=%d acted=%d failures=%d, want 4/3/0", result.ProjectsFiltered, result.ProjectsActed, len(result.Failures))
	}
	for _, n := range ntf.sent {
		switch n.ProjectID {
		case "p1", "p6", "p7":
		default:
			t.Errorf("notice sent for %s, a filtered project type", n.ProjectID)
		}
	}
}

func TestRun_SelectionFilterAppliesToAScopedRun(t *testing.T) {
	now := time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC)
	end := now.AddDate(0, 0, 89).Format(time.RFC3339)
	reader := &mockEntityReader{getProjectFn: func(context.Context, string) ([]byte, error) {
		return []byte(`{"suspensionProcessOverridden": false, "id":"p1","endDate":"` + end + `","subscriptionType":"internal"}`), nil
	}}
	ntf := delivering()
	result, err := Run(context.Background(), reader, CSMDetails(reader), &mockProjectUpdater{}, ntf, now, "p1", nil, 0)
	if err != nil || result.ProjectsFiltered != 1 || len(ntf.sent) != 0 {
		t.Errorf("err=%v filtered=%d sent=%d, want nil/1/0", err, result.ProjectsFiltered, len(ntf.sent))
	}
}
