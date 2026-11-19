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
	"context"
	"slices"
	"testing"

	"github.com/adhocore/gronx"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/productissuereport"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/registry"
)

type stubReportSource struct{ calls int }

func (s *stubReportSource) Dashboard(context.Context, string, string, []string) (productissuereport.Dashboard, error) {
	s.calls++
	return productissuereport.Dashboard{}, nil
}

func (s *stubReportSource) Recipients(context.Context) ([]productissuereport.Lead, error) {
	s.calls++
	return nil, nil
}

func TestProductIssueReportTasks(t *testing.T) {
	rec := map[string]subCronRecipients{"product_issue_weekly_report": {To: []string{"all@x.invalid"}, Debug: []string{"d@x.invalid"}}}
	tasks, withRec := productIssueReportTasks(&stubReportSource{}, &recordingSender{}, "", true, nil, rec)
	if len(tasks) != 2 || tasks[0].Name != "product_issue_weekly_report" || tasks[1].Name != "product_issue_monthly_report" {
		t.Fatalf("tasks = %+v", tasks)
	}
	if tasks[0].Schedule != "30 3 * * 1" || tasks[1].Schedule != "30 3 1 * *" {
		t.Errorf("schedules = %q %q", tasks[0].Schedule, tasks[1].Schedule)
	}
	for _, tk := range tasks {
		if tk.MaxAttempts != 2 || !gronx.IsValid(tk.Schedule) || tk.Handler == nil || tk.RunNowNarrow == nil ||
			!slices.Contains(tk.RunNowFlags, "--recipients") || !slices.Contains(tk.RunNowFlags, "--cc") || len(tk.To) != 0 {
			t.Errorf("task %s misconfigured: %+v", tk.Name, tk)
		}
	}
	if !slices.Equal(withRec, []string{"product_issue_weekly_report", "product_issue_monthly_report"}) {
		t.Errorf("withRecipients = %v", withRec)
	}

	// A schedule override applies.
	tasks, _ = productIssueReportTasks(&stubReportSource{}, &recordingSender{}, "", true,
		map[string]string{"product_issue_monthly_report": "0 5 2 * *"}, nil)
	if tasks[1].Schedule != "0 5 2 * *" || tasks[0].Schedule != "30 3 * * 1" {
		t.Errorf("override: %q %q", tasks[0].Schedule, tasks[1].Schedule)
	}
}

func TestReportTaskNamesForTheStartupCheck(t *testing.T) {
	on, off := reportTaskNamesFor(true), reportTaskNamesFor(false)
	for _, n := range []string{"product_issue_weekly_report", "product_issue_monthly_report"} {
		if !slices.Contains(on, n) || slices.Contains(off, n) {
			t.Errorf("%s: on=%v off=%v", n, slices.Contains(on, n), slices.Contains(off, n))
		}
	}
	if len(on)-len(off) != 2 {
		t.Errorf("only the two product issue tasks may differ: %d vs %d", len(on), len(off))
	}
}

func TestProductIssueReportRunNowRecipientsReplaceAudience(t *testing.T) {
	raw := &recordingSender{}
	src := &stubReportSource{}
	tasks, _ := productIssueReportTasks(src, raw, "", true, nil, nil)
	// With --recipients the (otherwise empty) configured audience becomes the
	// override list, as a [Test copy].
	h, _, err := tasks[0].RunNowNarrow(registry.RunNowOptions{Recipients: []string{"jane@x.invalid"}})
	if err != nil {
		t.Fatal(err)
	}
	if err := h(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(raw.to) != 1 || raw.to[0] != "jane@x.invalid" || src.calls == 0 {
		t.Errorf("to=%v calls=%d", raw.to, src.calls)
	}
}
