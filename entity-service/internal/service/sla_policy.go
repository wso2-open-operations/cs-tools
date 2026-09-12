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
// KIND, either express or implied. See the License for the
// specific language governing permissions and limitations
// under the License.

package service

import (
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// SLA clock type names — shared with csm-notification-service's own
// internal/slaengine, which treats ClockType as a caller-defined string
// (see domain.SLAClock's own doc comment), not a fixed enum. These three
// are the only ones this service ever registers.
const (
	slaClockTypeResponse   = "response"
	slaClockTypeWorkaround = "workaround"
	slaClockTypeResolution = "resolution"
)

// slaDurations maps a case's raw severity to the SLA durations WSO2's
// Enterprise Support Plan defines for each applicable clock type
// (https://wso2.com/licenses/support-policy/6.0) — CATASTROPHIC/CRITICAL/
// HIGH/MEDIUM map 1:1 to the policy's S0-S3, LOW to S4 ("Queries"). S4 has
// no fixed Workaround/Resolution SLA ("best efforts") — its entry
// deliberately has only a "response" duration, so publishSLAClockRegister
// (sn_case_service.go) never registers those two clocks for a LOW-severity
// case at all, rather than inventing a fallback duration the policy doesn't
// define.
//
// MEDIUM's "resolution" duration approximates the policy's "1 Business
// Week" as a flat 7*24h — not full business-hours accounting — but see
// slaAvoidWeekendClockTypes below for the one adjustment this service does
// make: the computed due date is rolled off a weekend.
var slaDurations = map[domain.CaseSeverity]map[string]time.Duration{
	domain.CaseSeverityCatastrophic: {
		slaClockTypeResponse:   15 * time.Minute,
		slaClockTypeWorkaround: 4 * time.Hour,
		slaClockTypeResolution: 48 * time.Hour,
	},
	domain.CaseSeverityCritical: {
		slaClockTypeResponse:   1 * time.Hour,
		slaClockTypeWorkaround: 24 * time.Hour,
		slaClockTypeResolution: 48 * time.Hour,
	},
	domain.CaseSeverityHigh: {
		slaClockTypeResponse:   4 * time.Hour,
		slaClockTypeWorkaround: 48 * time.Hour,
		slaClockTypeResolution: 72 * time.Hour,
	},
	domain.CaseSeverityMedium: {
		slaClockTypeResponse:   6 * time.Hour,
		slaClockTypeWorkaround: 72 * time.Hour,
		slaClockTypeResolution: 7 * 24 * time.Hour, // "1 Business Week" — see slaAvoidWeekendClockTypes
	},
	domain.CaseSeverityLow: {
		slaClockTypeResponse: 24 * time.Hour, // "1 Business Day"; no Workaround/Resolution SLA at this severity
	},
}

// slaAvoidWeekendClockTypes names the clock types, for MEDIUM severity only,
// whose due date must not land on a weekend — currently just "resolution"
// ("1 Business Week"). Threaded onto the published sla.clock.register event
// as AvoidWeekendDueDate so csm-notification-service's slaengine — which
// alone knows the real startedAt/dueAt at consume time — can roll the due
// date forward past Sat/Sun. Every other severity's durations are plain
// calendar-hour SLAs with no "business" qualifier, so nothing else needs
// this adjustment.
var slaAvoidWeekendClockTypes = map[domain.CaseSeverity][]string{
	domain.CaseSeverityMedium: {slaClockTypeResolution},
}
