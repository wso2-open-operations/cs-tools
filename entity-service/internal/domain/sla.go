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

package domain

import (
	"encoding/json"
	"time"
)

// This file is the admin-config surface of the SLA module (SLA SPEC.md) --
// the field registry, calendars, definitions and notification rules
// migrations 0245-0250 back. It does not yet cover the matching engine or
// the live sla_instance read API (POST /sla/instances/search): those depend
// on the field resolver registry and the threshold-crossing worker, which
// are their own, separately-scoped follow-up (see this change's own plan
// file).

// SLAWorkItemType is one row of sla_work_item_type -- whether a work item
// type may carry SLAs at all (R3, R16). Developer-seeded; Enabled is the
// only field this API lets a caller change.
type SLAWorkItemType struct {
	WorkItemType string `json:"workItemType"`
	Enabled      bool   `json:"enabled"`
}

// SLAFieldCatalogEntry is one developer-seeded field a work item type
// exposes to SLA conditions (R5), joined with its admin enablement flag
// (R16) -- the shape GET /sla/field-catalog actually returns, since an
// admin never needs the two halves separately.
type SLAFieldCatalogEntry struct {
	ID           string   `json:"id"`
	WorkItemType string   `json:"workItemType"`
	FieldKey     string   `json:"fieldKey"`
	DataType     string   `json:"dataType"`
	EnumValues   []string `json:"enumValues,omitempty"`
	DisplayLabel string   `json:"displayLabel"`
	IsStateField bool     `json:"isStateField"`
	Enabled      bool     `json:"enabled"`
}

// UpdateSLAFieldEnablementRequest is PATCH /sla/field-catalog/{id}/enablement's
// body -- the only admin-writable thing about a catalog entry (R16).
type UpdateSLAFieldEnablementRequest struct {
	Enabled bool `json:"enabled"`
}

// SLAWorkingDay is one day of an sla_calendar's working_week (R23). Null
// StartTime/EndTime mean the day is never a working day.
type SLAWorkingDay struct {
	Weekday   int     `json:"weekday"` // 0 = Sunday .. 6 = Saturday (time.Weekday numbering)
	IsWorking bool    `json:"isWorking"`
	StartTime *string `json:"startTime"` // "HH:MM", local to the calendar (R26)
	EndTime   *string `json:"endTime"`
}

// SLAHoliday is one named holiday date or range on an sla_calendar (R24).
type SLAHoliday struct {
	ID        string `json:"id"`
	StartDate string `json:"startDate"` // "YYYY-MM-DD"
	EndDate   string `json:"endDate"`
	Name      string `json:"name"`
}

// SLACalendar is an sla_calendar row plus its holiday list. Exactly two
// exist (24x7, WSO2 Working Days, seeded by migration 0246) -- this API has
// no create/retire route, only GET/PATCH on the two seeded rows (see this
// change's own plan file, "Calendars are two seeded rows").
type SLACalendar struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	IsRetired   bool            `json:"isRetired"`
	WorkingWeek []SLAWorkingDay `json:"workingWeek"`
	Holidays    []SLAHoliday    `json:"holidays"`
	CreatedOn   time.Time       `json:"createdOn"`
	UpdatedOn   time.Time       `json:"updatedOn"`
}

// UpdateSLACalendarWorkingWeekRequest is PATCH /sla/calendars/{id}'s body --
// replaces the whole 7-day working_week array in one call (R23).
type UpdateSLACalendarWorkingWeekRequest struct {
	WorkingWeek []SLAWorkingDay `json:"workingWeek"`
}

// AddSLAHolidayRequest is POST /sla/calendars/{id}/holidays' body (R24).
type AddSLAHolidayRequest struct {
	StartDate string `json:"startDate"`
	EndDate   string `json:"endDate"`
	Name      string `json:"name"`
}

// SLADefinition is an sla_definition row (SLA SPEC.md section 6.1/6.1.1/6.1.2).
// The four condition fields are predicate trees -- {"all"|"any": [...]} |
// {"not": ...} | {"field","op","value"} (R4) -- carried as json.RawMessage,
// validated against the enabled field catalog in the service layer, never
// interpreted at the handler/repository layer.
type SLADefinition struct {
	ID                         string          `json:"id"`
	Name                       string          `json:"name"`
	Description                *string         `json:"description,omitempty"`
	WorkItemType               string          `json:"workItemType"`
	Status                     string          `json:"status"` // draft | published | retired (R14)
	CalendarID                 string          `json:"calendarId"`
	TimezoneSource             string          `json:"timezoneSource"` // definition | account (R26)
	DefinitionTimezone         *string         `json:"definitionTimezone,omitempty"`
	AccountTimezoneFallback    *string         `json:"accountTimezoneFallback,omitempty"`
	StartMode                  string          `json:"startMode"` // now | retroactive | carry_over (R12)
	TargetLabel                string          `json:"targetLabel"`
	DurationSeconds            int64           `json:"durationSeconds"` // the target, in working time (R25)
	PauseReason                *string         `json:"pauseReason,omitempty"`
	StartCondition             json.RawMessage `json:"startCondition"`
	PauseCondition             json.RawMessage `json:"pauseCondition,omitempty"`
	StopCondition              json.RawMessage `json:"stopCondition,omitempty"`
	CancelCondition            json.RawMessage `json:"cancelCondition,omitempty"`
	ThresholdPercentages       []int           `json:"thresholdPercentages"` // always includes 100 (R39)
	AlsoCompletesDefinitionIDs []string        `json:"alsoCompletesDefinitionIds,omitempty"`
	Version                    int             `json:"version"`
	CreatedOn                  time.Time       `json:"createdOn"`
	UpdatedOn                  time.Time       `json:"updatedOn"`
	CreatedBy                  string          `json:"createdBy"`
	UpdatedBy                  string          `json:"updatedBy"`
}

// UpsertSLADefinitionRequest is the body of both POST /sla/definitions
// (create, always lands in draft -- R14) and PATCH /sla/definitions/{id}
// (edit, status untouched -- see SetSLADefinitionStatusRequest for that).
type UpsertSLADefinitionRequest struct {
	Name                       string          `json:"name"`
	Description                *string         `json:"description,omitempty"`
	WorkItemType               string          `json:"workItemType"`
	CalendarID                 string          `json:"calendarId"`
	TimezoneSource             string          `json:"timezoneSource"`
	DefinitionTimezone         *string         `json:"definitionTimezone,omitempty"`
	AccountTimezoneFallback    *string         `json:"accountTimezoneFallback,omitempty"`
	StartMode                  string          `json:"startMode"`
	TargetLabel                string          `json:"targetLabel"`
	DurationSeconds            int64           `json:"durationSeconds"`
	PauseReason                *string         `json:"pauseReason,omitempty"`
	StartCondition             json.RawMessage `json:"startCondition"`
	PauseCondition             json.RawMessage `json:"pauseCondition,omitempty"`
	StopCondition              json.RawMessage `json:"stopCondition,omitempty"`
	CancelCondition            json.RawMessage `json:"cancelCondition,omitempty"`
	ThresholdPercentages       []int           `json:"thresholdPercentages"`
	AlsoCompletesDefinitionIDs []string        `json:"alsoCompletesDefinitionIds,omitempty"`
}

// SetSLADefinitionStatusRequest is PATCH /sla/definitions/{id}/status' body
// -- draft/publish/retire is its own call, deliberately separate from an
// ordinary field edit, so publishing is always a conscious, auditable act
// (R14) rather than a side effect of some other field changing.
type SetSLADefinitionStatusRequest struct {
	Status string `json:"status"`
}

// SearchSLADefinitionsRequest is POST /sla/definitions/search's body.
type SearchSLADefinitionsRequest struct {
	WorkItemType *string    `json:"workItemType,omitempty"`
	Status       *string    `json:"status,omitempty"`
	Pagination   Pagination `json:"pagination"`
}

// SearchSLADefinitionsResponse is that search's response.
type SearchSLADefinitionsResponse struct {
	Definitions []SLADefinition `json:"definitions"`
	Total       int             `json:"total"`
	Limit       int             `json:"limit"`
	Offset      int             `json:"offset"`
}

// SLANotificationRule is an sla_notification_rule row (R46-R51).
type SLANotificationRule struct {
	ID                  string                    `json:"id"`
	DefinitionID        *string                   `json:"definitionId,omitempty"`        // nil = every definition
	ThresholdPercentage *int                      `json:"thresholdPercentage,omitempty"` // nil = any threshold
	Channel             string                    `json:"channel"`                       // email | webhook (R50)
	Recipients          SLANotificationRecipients `json:"recipients"`
	WebhookURL          *string                   `json:"webhookUrl,omitempty"`
	IsActive            bool                      `json:"isActive"`
	CreatedOn           time.Time                 `json:"createdOn"`
	UpdatedOn           time.Time                 `json:"updatedOn"`
}

// SLANotificationRecipients is R49's three recipient kinds -- resolved
// fresh when the event is handled, never frozen here.
type SLANotificationRecipients struct {
	Assignee bool     `json:"assignee"`
	Team     bool     `json:"team"`
	Users    []string `json:"users,omitempty"`
	Groups   []string `json:"groups,omitempty"`
}

// UpsertSLANotificationRuleRequest is the body of both POST and PATCH
// /sla/notification-rules[/{id}].
type UpsertSLANotificationRuleRequest struct {
	DefinitionID        *string                   `json:"definitionId,omitempty"`
	ThresholdPercentage *int                      `json:"thresholdPercentage,omitempty"`
	Channel             string                    `json:"channel"`
	Recipients          SLANotificationRecipients `json:"recipients"`
	WebhookURL          *string                   `json:"webhookUrl,omitempty"`
	IsActive            *bool                     `json:"isActive,omitempty"`
}
