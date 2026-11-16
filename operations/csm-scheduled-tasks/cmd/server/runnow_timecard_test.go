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
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/notify"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/registry"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/timecardreport"
)

func TestParseRunNowArgsTimeCardFlags(t *testing.T) {
	base := "--run-now=time_card_report"
	tests := []struct {
		name string
		args []string
		want *runNowArgs
	}{
		{"no flags", []string{base},
			&runNowArgs{Task: "time_card_report"}},
		{"project key alone", []string{base, "--project-key=PRJ-1"},
			&runNowArgs{Task: "time_card_report", Options: registry.RunNowOptions{ProjectKey: "PRJ-1"}, Given: []string{"--project-key"}}},
		{"to with project key", []string{base, "--project-key=PRJ-1", "--to=jane.doe@example.com,john.roe@example.com"},
			&runNowArgs{Task: "time_card_report", Options: registry.RunNowOptions{ProjectKey: "PRJ-1", Recipients: []string{"jane.doe@example.com", "john.roe@example.com"}},
				Given: []string{"--project-key", "--to"}}},
		{"to alone", []string{base, "--to=jane.doe@example.com"},
			&runNowArgs{Task: "time_card_report", Options: registry.RunNowOptions{Recipients: []string{"jane.doe@example.com"}}, Given: []string{"--to"}}},
		{"cc alone", []string{base, "--cc=ops@example.com"},
			&runNowArgs{Task: "time_card_report", Options: registry.RunNowOptions{Cc: []string{"ops@example.com"}}, Given: []string{"--cc"}}},
		{"everything", []string{base, "--project-key= PRJ-1 ", "--to=jane.doe@example.com", "--cc=ops@example.com"},
			&runNowArgs{Task: "time_card_report", Options: registry.RunNowOptions{ProjectKey: "PRJ-1", Recipients: []string{"jane.doe@example.com"}, Cc: []string{"ops@example.com"}},
				Given: []string{"--project-key", "--to", "--cc"}}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseRunNowArgs(tc.args)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got %+v, want %+v", got, tc.want)
			}
		})
	}

	var many []string
	for i := 0; i < timecardreport.MaxRecipients+1; i++ {
		many = append(many, "user"+strings.Repeat("x", i)+"@example.com")
	}
	fifty := strings.Join(many[:timecardreport.MaxRecipients], ",")
	if _, err := parseRunNowArgs([]string{base, "--to=" + fifty}); err != nil {
		t.Errorf("exactly %d addresses must pass: %v", timecardreport.MaxRecipients, err)
	}

	bad := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{"to without run-now", []string{"--to=jane.doe@example.com"}, "only valid together with --run-now"},
		{"project key without run-now", []string{"--project-key=PRJ-1"}, "only valid together with --run-now"},
		{"invalid email", []string{base, "--to=not-an-address"}, "not a plain email"},
		{"one bad among good", []string{base, "--to=jane.doe@example.com,nope"}, "not a plain email"},
		{"display name", []string{base, "--to=Jane<jane.doe@example.com>"}, "not a plain email"},
		{"empty to", []string{base, "--to="}, "empty entry"},
		{"trailing comma", []string{base, "--to=jane.doe@example.com,"}, "empty entry"},
		{"too many", []string{base, "--to=" + strings.Join(many, ",")}, "at most 50"},
		{"invalid cc", []string{base, "--cc=nope"}, "--cc entry"},
		{"too many cc", []string{base, "--cc=" + strings.Join(many, ",")}, "at most 50"},
		{"empty project key", []string{base, "--project-key="}, "must be one project key"},
		{"project key list", []string{base, "--project-key=PRJ-1,PRJ-2"}, "must be one project key"},
		{"project key with space", []string{base, "--project-key=PRJ 1"}, "must be one project key"},
		{"to and recipients", []string{base, "--to=jane.doe@example.com", "--recipients=john.roe@example.com"}, "cannot be combined"},
	}
	for _, tc := range bad {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parseRunNowArgs(tc.args)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("err = %v, want containing %q", err, tc.wantErr)
			}
		})
	}

	// For every other task --cc still needs --recipients, as before.
	if _, err := parseRunNowArgs([]string{"--run-now=stale_cases_report", "--cc=ops@example.com"}); err == nil || !strings.Contains(err.Error(), "--cc needs --recipients") {
		t.Errorf("--cc alone on another task: err = %v", err)
	}
}

// The flags are refused for any task that does not declare them, with a
// message that names the flag and the task.
func TestResolveRunNowTimeCardFlagsRefusedElsewhere(t *testing.T) {
	var s, h int
	var o registry.RunNowOptions
	tasks := resolveFixture(&s, &h, &o)
	tasks = append(tasks,
		registry.Task{Name: "stale_cases_report", Handler: func(context.Context) error { return nil },
			RunNowNarrow: func(registry.RunNowOptions) (func(context.Context) error, []any, error) { return nil, nil, nil },
			RunNowFlags:  reportRunNowFlags})
	for _, task := range []string{"simple_task", "hooked_task", "stale_cases_report"} {
		for _, f := range []string{"--to", "--project-key"} {
			_, _, err := resolveRunNow(tasks, runNowArgs{Task: task, Given: []string{f}})
			if want := f + " is not supported by task " + task; err == nil || !strings.Contains(err.Error(), want) {
				t.Errorf("%s %s: err = %v, want containing %q", task, f, err, want)
			}
		}
	}
}

type captureSender struct{ name string }

func (captureSender) SendEmail(context.Context, []string, []string, string, string) error { return nil }

func TestTimeCardRunNowHook(t *testing.T) {
	normal, raw := &captureSender{"normal"}, &captureSender{"raw"}
	configured := []string{"PRJ-1", "PRJ-2"}
	var gotEmail notify.Sender
	var gotCfg timecardreport.Config
	hook := timeCardRunNow(normal, raw, configured, func(email notify.Sender, cfg timecardreport.Config) func(context.Context) error {
		gotEmail, gotCfg = email, cfg
		return func(context.Context) error { return nil }
	})

	// Nothing given: the schedule's own projects and sender, nobody overridden.
	if _, _, err := hook(registry.RunNowOptions{}); err != nil {
		t.Fatal(err)
	}
	if gotEmail != normal || !reflect.DeepEqual(gotCfg, timecardreport.Config{ProjectKeys: configured}) {
		t.Errorf("no flags: sender %v cfg %+v", gotEmail, gotCfg)
	}

	// --project-key alone: that project only, still the derived contacts.
	if _, _, err := hook(registry.RunNowOptions{ProjectKey: "PRJ-9"}); err != nil {
		t.Fatal(err)
	}
	if gotEmail != normal || !reflect.DeepEqual(gotCfg, timecardreport.Config{ProjectKeys: []string{"PRJ-9"}}) {
		t.Errorf("project key: sender %v cfg %+v", gotEmail, gotCfg)
	}

	// --to with --project-key: that project to the list, through the raw sender.
	_, settings, err := hook(registry.RunNowOptions{ProjectKey: "PRJ-9", Recipients: []string{"jane.doe@example.com"}, Cc: []string{"ops@example.com"}})
	if err != nil {
		t.Fatal(err)
	}
	if gotEmail != raw || !reflect.DeepEqual(gotCfg, timecardreport.Config{ProjectKeys: []string{"PRJ-9"}, To: []string{"jane.doe@example.com"}, Cc: []string{"ops@example.com"}}) {
		t.Errorf("to+project key: sender %v cfg %+v", gotEmail, gotCfg)
	}
	// The WARN names counts and the double-send risk, never an address.
	logged := fmt.Sprint(settings)
	if strings.Contains(logged, "@") || !strings.Contains(logged, "no ledger row") {
		t.Errorf("settings = %v", settings)
	}

	// --to alone: every configured project to the list.
	if _, _, err := hook(registry.RunNowOptions{Recipients: []string{"jane.doe@example.com"}}); err != nil {
		t.Fatal(err)
	}
	if gotEmail != raw || !reflect.DeepEqual(gotCfg.ProjectKeys, configured) || len(gotCfg.To) != 1 {
		t.Errorf("to alone: sender %v cfg %+v", gotEmail, gotCfg)
	}
}
