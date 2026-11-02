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
	"errors"
	"os"
	"strings"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/acp"
)

// acpSettings are what acpConfig decides from: the ACP_* settings and the
// component-wide ones they depend on.
type acpSettings struct {
	DryRun                 bool
	EmailSendEnabled       bool
	AllowNonWSO2Recipients bool
	StandingRecipients     []string
	TestProjectID          string
	ExcludedProjectIDs     map[string]bool

	AlertsEnabled    bool
	EmailBaseURL     string
	EmailFromAddress string
	// Debug is acp_closure's "debug" list in SUB_CRON_RECIPIENTS.
	Debug []string
	// Tryout is TRYOUT_TASK=acp_closure.
	Tryout bool
}

// readACPSettings reads acpSettings from the environment.
func readACPSettings(alertsEnabled bool, emailBaseURL string, debug []string) acpSettings {
	return acpSettings{
		// Fails toward a dry run: unset or malformed is true.
		DryRun:                 envBool("ACP_DRY_RUN", true),
		EmailSendEnabled:       envBool("ACP_EMAIL_SEND_ENABLED", false),
		AllowNonWSO2Recipients: envBool("ACP_ALLOW_NON_WSO2_RECIPIENTS", false),
		StandingRecipients:     splitComma(os.Getenv("ACP_STANDING_CC_RECIPIENTS")),
		TestProjectID:          strings.TrimSpace(os.Getenv("ACP_TEST_PROJECT_ID")),
		ExcludedProjectIDs:     acp.ParseProjectIDs(os.Getenv("ACP_EXCLUDED_PROJECT_IDS")),
		AlertsEnabled:          alertsEnabled,
		EmailBaseURL:           emailBaseURL,
		EmailFromAddress:       strings.TrimSpace(os.Getenv("EMAIL_FROM_ADDRESS")),
		Debug:                  debug,
		Tryout:                 strings.TrimSpace(os.Getenv("TRYOUT_TASK")) == acpTaskName,
	}
}

// acpConfig turns s into the acp_closure configuration, or refuses it. The
// warnings describe a setting it changed; the caller logs them. Every
// refusal is a combination that would suspend customers who were never
// told, or send them the same notice on every run:
//
//   - A real run records each notice and suspends on day 0 whatever the
//     notifier did, so it needs emails that really go out: with
//     ACP_EMAIL_SEND_ENABLED off it would record notices nobody received and
//     suspend customers who never got one.
//   - A dry run records nothing, so with emails on every run sends every due
//     notice again (a project 90 days out would get the 90-day notice on
//     about 30 weekdays). Allowed only with a debug list, where the
//     repetition reaches testers and is the point.
//   - With a debug list every notice goes to the testers while the real run
//     records it as delivered and suspends on day 0. Allowed for a real run
//     only when scoped to one test project.
//
// ALERTS_ENABLED=false (this component's switch for all email) turns the
// task into a dry run without emails instead of refusing: refusing would
// stop every other task too, and a real run with emails silenced is the
// first case above. A dry run writes nothing, so nothing is lost; once
// alerts are back, the next real run sends what is due (windows are
// thresholds).
//
// A TRYOUT_TASK=acp_closure not scoped to one test project is a dry run
// (it would otherwise write to every open project), and runs on weekends.
func acpConfig(s acpSettings) (acp.Config, []string, error) {
	cfg := acp.Config{
		DryRun:                 s.DryRun,
		EmailEnabled:           s.EmailSendEnabled,
		AllowNonWSO2Recipients: s.AllowNonWSO2Recipients,
		StandingRecipients:     s.StandingRecipients,
		TestProjectID:          s.TestProjectID,
		ExcludedProjectIDs:     s.ExcludedProjectIDs,
		RunOnWeekends:          s.Tryout,
	}
	var warnings []string
	if s.Tryout && !cfg.DryRun && cfg.TestProjectID == "" {
		warnings = append(warnings, "TRYOUT_TASK is acp_closure without ACP_TEST_PROJECT_ID: running as a dry run, nothing will be written; set ACP_DRY_RUN=true explicitly to silence this")
		cfg.DryRun = true
	}
	if !s.AlertsEnabled && (!cfg.DryRun || cfg.EmailEnabled) {
		warnings = append(warnings, "ALERTS_ENABLED is false: acp_closure runs as a dry run without emails, deciding and logging only (no notices sent, nothing written, no project suspended)")
		cfg.DryRun, cfg.EmailEnabled = true, false
	}

	if cfg.EmailEnabled && s.EmailBaseURL == "" {
		return acp.Config{}, warnings, errors.New("ACP_EMAIL_SEND_ENABLED is true but EMAIL_BASE_URL is not set: no notice could be sent")
	}
	if cfg.EmailEnabled && s.EmailFromAddress == "" {
		return acp.Config{}, warnings, errors.New("ACP_EMAIL_SEND_ENABLED is true but EMAIL_FROM_ADDRESS is not set: customer notices need a From address")
	}
	if !cfg.DryRun && !cfg.EmailEnabled {
		return acp.Config{}, warnings, errors.New("ACP_DRY_RUN=false needs ACP_EMAIL_SEND_ENABLED=true: a real run without emails would record notices nobody received and suspend customers who were never told")
	}
	if cfg.DryRun && cfg.EmailEnabled && len(s.Debug) == 0 {
		return acp.Config{}, warnings, errors.New("ACP_DRY_RUN and ACP_EMAIL_SEND_ENABLED are both true without a debug list for acp_closure in SUB_CRON_RECIPIENTS: a dry run records no notice, so every run would send the same notices again")
	}
	if len(s.Debug) > 0 && !cfg.DryRun && cfg.TestProjectID == "" {
		return acp.Config{}, warnings, errors.New("a debug list for acp_closure is set with ACP_DRY_RUN=false and no ACP_TEST_PROJECT_ID: notices would reach only the debug list while every project is recorded as notified and suspended on day 0")
	}
	return cfg, warnings, nil
}
