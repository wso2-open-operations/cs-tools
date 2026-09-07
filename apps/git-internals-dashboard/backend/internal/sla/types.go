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

// Package sla is the pure SLA engine: no I/O, no database — unit-testable in
// isolation (see engine_test.go). It is a semantics-identical port of v3's
// src/server/db/sla.ts (SPEC §7), the correctness core of the whole system.
// Every status here is *string rather than string so "unset"/null (no event
// yet, no current status) stays distinguishable from the empty-string
// taxonomy status, exactly as TypeScript's `string | null` does.
package sla

import "time"

// SlaState is the headline SLA verdict for an issue.
type SlaState string

const (
	NoSla    SlaState = "NO_SLA"
	Ok       SlaState = "OK"
	AtRisk   SlaState = "AT_RISK"
	Violated SlaState = "VIOLATED"
	Terminal SlaState = "TERMINAL"
)

// Coverage is when the SLA clock is allowed to tick.
//   - Coverage24x7    — around the clock, every day (e.g. P1).
//   - Coverage12x5Ist — business hours only: Mon-Fri, 09:00-21:00 IST (e.g. P2/P3).
//
// Unknown/missing coverage falls back to Coverage24x7 (the historical behavior).
type Coverage string

const (
	Coverage24x7    Coverage = "24x7"
	Coverage12x5Ist Coverage = "12x5_ist"
)

// StatusEvent is the status active starting at OccurredAt.
type StatusEvent struct {
	Status     *string
	OccurredAt time.Time
}

// Config supplies everything ComputeSla needs from taxonomy/budget config,
// kept as plain functions/maps so this package never imports internal/config
// (SPEC §3: sla is a leaf, dependency-free package).
type Config struct {
	Budgets           map[string]float64  // priority -> budget hours
	Coverage          map[string]Coverage // priority -> coverage window; absent => 24x7
	Accrues           func(status *string) bool
	IsTerminal        func(status *string) bool
	PossibleThreshold float64
}

// Result is computeSla's output.
type Result struct {
	BudgetHours    *float64
	ConsumedHours  float64
	RemainingHours *float64
	PctConsumed    *float64
	SlaState       SlaState
	SlaRunning     bool
}
