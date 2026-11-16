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
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

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
	// DailyLimit is ACP_DAILY_LIMIT as set, parsed by acpConfig.
	DailyLimit string
	// Screening is ACP_SCREENING and FullSweepWeekday ACP_SCREENING_FULL_SWEEP_WEEKDAY
	// as set, parsed by acpConfig.
	Screening, FullSweepWeekday string
	// Repair is ACP_REPAIR and RepairMaxEvents ACP_REPAIR_MAX_EVENTS as set.
	Repair, RepairMaxEvents string
	// SalesEntityReads is ACP_SALES_ENTITY_READS as set.
	SalesEntityReads string

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
		DailyLimit:             strings.TrimSpace(os.Getenv("ACP_DAILY_LIMIT")),
		Screening:              strings.TrimSpace(os.Getenv("ACP_SCREENING")),
		FullSweepWeekday:       strings.TrimSpace(os.Getenv("ACP_SCREENING_FULL_SWEEP_WEEKDAY")),
		Repair:                 strings.TrimSpace(os.Getenv("ACP_REPAIR")),
		RepairMaxEvents:        strings.TrimSpace(os.Getenv("ACP_REPAIR_MAX_EVENTS")),
		SalesEntityReads:       strings.TrimSpace(os.Getenv("ACP_SALES_ENTITY_READS")),
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
// first case above. A dry run writes nothing, so no notice is recorded as
// sent; once alerts are back, the next real run sends the notice that is due
// then. Only the narrowest window reached is sent, not every window passed
// (closure.dueWindow): after a pause longer than a window, a project that
// crossed the 15- and 7-day windows meanwhile gets neither, and one whose end
// date passed gets the day-0 notice and is suspended in the same run.
//
// ACP_SCREENING (on or off, default on) and ACP_SCREENING_FULL_SWEEP_WEEKDAY
// (Mon to Sun, default Monday) configure hybrid evaluation; anything else is
// refused.
//
// ACP_DAILY_LIMIT caps the projects one run acts on (acp.Config.DailyLimit); a
// malformed or negative value is refused.
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
	limit, err := parseACPDailyLimit(s.DailyLimit)
	if err != nil {
		return acp.Config{}, nil, err
	}
	cfg.DailyLimit = limit
	if cfg.Screening, err = parseACPScreening(s.Screening); err != nil {
		return acp.Config{}, nil, err
	}
	if cfg.FullSweepWeekday, err = parseACPWeekday(s.FullSweepWeekday); err != nil {
		return acp.Config{}, nil, err
	}
	if cfg.Repair, err = parseACPOnOff("ACP_REPAIR", s.Repair); err != nil {
		return acp.Config{}, nil, err
	}
	if cfg.RepairMaxEvents, err = parseACPRepairMax(s.RepairMaxEvents); err != nil {
		return acp.Config{}, nil, err
	}
	reads, err := parseACPOnOff("ACP_SALES_ENTITY_READS", s.SalesEntityReads)
	if err != nil {
		return acp.Config{}, nil, err
	}
	cfg.NoSalesEntityReads = !reads
	var warnings []string
	if cfg.NoSalesEntityReads {
		warnings = append(warnings, "ACP_SALES_ENTITY_READS is off: acp_closure makes no sales-entity-service read and decides only from CSM's copy, with no live confirmation; screening, the weekly full sweep and the repair of CSM's copy are disabled by this mode")
	}
	if cfg.Screening && !cfg.NoSalesEntityReads && !s.Tryout &&
		(cfg.FullSweepWeekday == time.Saturday || cfg.FullSweepWeekday == time.Sunday) {
		warnings = append(warnings, "ACP_SCREENING_FULL_SWEEP_WEEKDAY is a weekend day: acp_closure does nothing on a Colombo weekend, so the weekly full sweep never runs and every scheduled run screens; choose Mon to Fri")
	}
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

// parseACPDailyLimit reads ACP_DAILY_LIMIT: a plain non-negative integer, 0 or
// unset meaning no cap. Anything else refuses to start: a typo that silently
// lifted the cap would let a first real run email and suspend every project.
func parseACPDailyLimit(v string) (int, error) {
	if v == "" {
		return 0, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 0 {
		return 0, fmt.Errorf("ACP_DAILY_LIMIT=%q must be a non-negative integer (0 = no cap)", v)
	}
	return n, nil
}

// parseACPScreening reads ACP_SCREENING: "on" (the default, also when unset
// or empty) or "off", any case.
func parseACPScreening(v string) (bool, error) {
	switch strings.ToLower(v) {
	case "", "on":
		return true, nil
	case "off":
		return false, nil
	}
	return false, fmt.Errorf("ACP_SCREENING=%q must be off or on", v)
}

// parseACPWeekday reads ACP_SCREENING_FULL_SWEEP_WEEKDAY: Mon to Sun (or the
// full day name), any case; unset is Monday.
func parseACPWeekday(v string) (time.Weekday, error) {
	if v == "" {
		return time.Monday, nil
	}
	for d := time.Sunday; d <= time.Saturday; d++ {
		name := strings.ToLower(d.String())
		if l := strings.ToLower(v); l == name || l == name[:3] {
			return d, nil
		}
	}
	return 0, fmt.Errorf("ACP_SCREENING_FULL_SWEEP_WEEKDAY=%q must be Mon, Tue, Wed, Thu, Fri, Sat or Sun", v)
}

// parseACPOnOff reads an on/off setting that defaults to on: "on" (also unset
// or empty) or "off", any case; anything else is refused.
func parseACPOnOff(name, v string) (bool, error) {
	switch strings.ToLower(v) {
	case "", "on":
		return true, nil
	case "off":
		return false, nil
	}
	return false, fmt.Errorf("%s=%q must be on or off", name, v)
}

// defaultACPRepairMaxEvents bounds the repair events of one run.
const defaultACPRepairMaxEvents = 200

// parseACPRepairMax reads ACP_REPAIR_MAX_EVENTS: a non-negative integer, 0
// meaning repair off, default 200 when unset; anything else is refused.
func parseACPRepairMax(v string) (int, error) {
	if v == "" {
		return defaultACPRepairMaxEvents, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 0 {
		return 0, fmt.Errorf("ACP_REPAIR_MAX_EVENTS=%q must be a non-negative integer (0 = repair off)", v)
	}
	return n, nil
}
