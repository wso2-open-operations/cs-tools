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
	"strings"
	"testing"
)

// The safety switches of acp_closure: each refused combination would
// suspend customers who were never told or send them the same notice on
// every run.
func TestACPConfig(t *testing.T) {
	// real is a production-shaped real run: dry run off, emails on.
	real := func() acpSettings {
		return acpSettings{
			EmailSendEnabled: true,
			AlertsEnabled:    true,
			EmailBaseURL:     "https://email.example.com",
			EmailFromAddress: "no-reply@example.com",
		}
	}
	debug := []string{"qa@example.com"}

	for _, tc := range []struct {
		name         string
		settings     func() acpSettings
		wantErr      string // substring; "" means accepted
		wantDryRun   bool
		wantEmail    bool
		wantWarning  string
		wantWeekends bool
	}{
		{name: "real run with emails", settings: real, wantEmail: true},
		{name: "default dry run, emails off", settings: func() acpSettings {
			s := real()
			s.DryRun, s.EmailSendEnabled = true, false
			return s
		}, wantDryRun: true},
		{name: "real run with emails off is refused", settings: func() acpSettings {
			s := real()
			s.EmailSendEnabled = false
			return s
		}, wantErr: "ACP_DRY_RUN=false needs ACP_EMAIL_SEND_ENABLED=true"},
		{name: "alerts off turns a real run into a dry run without emails", settings: func() acpSettings {
			s := real()
			s.AlertsEnabled = false
			return s
		}, wantDryRun: true, wantWarning: "ALERTS_ENABLED is false"},
		{name: "alerts off with emails off is a dry run, not a refusal", settings: func() acpSettings {
			s := real()
			s.AlertsEnabled, s.EmailSendEnabled = false, false
			return s
		}, wantDryRun: true, wantWarning: "ALERTS_ENABLED is false"},
		{name: "dry run with emails and no debug list is refused", settings: func() acpSettings {
			s := real()
			s.DryRun = true
			return s
		}, wantErr: "without a debug list"},
		{name: "dry run with emails to a debug list", settings: func() acpSettings {
			s := real()
			s.DryRun, s.Debug = true, debug
			return s
		}, wantDryRun: true, wantEmail: true},
		{name: "real run with a debug list and no test project is refused", settings: func() acpSettings {
			s := real()
			s.Debug = debug
			return s
		}, wantErr: "no ACP_TEST_PROJECT_ID"},
		{name: "real run with a debug list on the test project", settings: func() acpSettings {
			s := real()
			s.Debug, s.TestProjectID = debug, "project-1"
			return s
		}, wantEmail: true},
		{name: "emails without EMAIL_BASE_URL are refused", settings: func() acpSettings {
			s := real()
			s.EmailBaseURL = ""
			return s
		}, wantErr: "EMAIL_BASE_URL"},
		{name: "emails without EMAIL_FROM_ADDRESS are refused", settings: func() acpSettings {
			s := real()
			s.EmailFromAddress = ""
			return s
		}, wantErr: "EMAIL_FROM_ADDRESS"},
		{name: "tryout without a test project is a dry run and runs on weekends", settings: func() acpSettings {
			s := real()
			s.Tryout, s.Debug = true, debug
			return s
		}, wantDryRun: true, wantEmail: true, wantWarning: "TRYOUT_TASK is acp_closure", wantWeekends: true},
		{name: "tryout on the test project stays real", settings: func() acpSettings {
			s := real()
			s.Tryout, s.Debug, s.TestProjectID = true, debug, "project-1"
			return s
		}, wantEmail: true, wantWeekends: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg, warnings, err := acpConfig(tc.settings())
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("err = %v, want one containing %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("refused: %v", err)
			}
			if cfg.DryRun != tc.wantDryRun || cfg.EmailEnabled != tc.wantEmail || cfg.RunOnWeekends != tc.wantWeekends {
				t.Errorf("dryRun=%v email=%v weekends=%v, want %v %v %v",
					cfg.DryRun, cfg.EmailEnabled, cfg.RunOnWeekends, tc.wantDryRun, tc.wantEmail, tc.wantWeekends)
			}
			joined := strings.Join(warnings, "\n")
			if (tc.wantWarning == "") != (joined == "") || !strings.Contains(joined, tc.wantWarning) {
				t.Errorf("warnings = %q, want one containing %q", joined, tc.wantWarning)
			}
		})
	}
}
