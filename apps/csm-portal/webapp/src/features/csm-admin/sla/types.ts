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

import type { PredicateNode } from "./utils/condition";

/**
 * Wire types for the SLA module's admin-config API (entity-service
 * migrations 0245-0250, proxied by apps/csm-portal/backend's PermAdmin-gated
 * /sla/* routes — see internal/handler/sla.go there). Field names mirror
 * entity-service's own domain.SLA* structs exactly (both already camelCase).
 *
 * This covers configuration only — the matching engine, the field-resolver
 * registry, the threshold-crossing worker and the live sla_instance read API
 * are a separate, not-yet-built piece (see the plan this feature was built
 * from), so there is no "SLA status" type here yet.
 */

export interface SlaWorkItemType {
  workItemType: string;
  enabled: boolean;
}

export interface SlaFieldCatalogEntry {
  id: string;
  workItemType: string;
  fieldKey: string;
  dataType: "string" | "enum" | "boolean" | "datetime" | "integer";
  enumValues?: string[];
  displayLabel: string;
  isStateField: boolean;
  enabled: boolean;
}

/** One day of an sla_calendar's working week. weekday follows JS's own
 *  Date.getDay() numbering (0 = Sunday .. 6 = Saturday), matching
 *  entity-service's internal/slacalendar (Go's time.Weekday — the same
 *  numbering), so this needs no translation on either side. */
export interface SlaWorkingDay {
  weekday: number;
  isWorking: boolean;
  startTime: string | null;
  endTime: string | null;
}

export interface SlaHoliday {
  id: string;
  startDate: string;
  endDate: string;
  name: string;
}

export interface SlaCalendar {
  id: string;
  name: string;
  isRetired: boolean;
  workingWeek: SlaWorkingDay[];
  holidays: SlaHoliday[];
  createdOn: string;
  updatedOn: string;
}

export type SlaDefinitionStatus = "draft" | "published" | "retired";
export type SlaTimezoneSource = "definition" | "account";
export type SlaStartMode = "now" | "retroactive" | "carry_over";

/** A predicate condition tree ({"all"|"any"|"not"} over
 *  {"field","op","value"} leaves) — see utils/condition.ts's own
 *  `PredicateNode` for the editor's shape of this same wire type. */
export type SlaCondition = PredicateNode | null;

export interface SlaDefinition {
  id: string;
  name: string;
  description?: string;
  workItemType: string;
  status: SlaDefinitionStatus;
  calendarId: string;
  timezoneSource: SlaTimezoneSource;
  definitionTimezone?: string;
  accountTimezoneFallback?: string;
  startMode: SlaStartMode;
  targetLabel: string;
  durationSeconds: number;
  pauseReason?: string;
  startCondition: SlaCondition;
  pauseCondition?: SlaCondition;
  stopCondition?: SlaCondition;
  cancelCondition?: SlaCondition;
  thresholdPercentages: number[];
  alsoCompletesDefinitionIds?: string[];
  version: number;
  createdOn: string;
  updatedOn: string;
  createdBy: string;
  updatedBy: string;
}

export interface UpsertSlaDefinitionRequest {
  name: string;
  description?: string;
  workItemType: string;
  calendarId: string;
  timezoneSource: SlaTimezoneSource;
  definitionTimezone?: string;
  accountTimezoneFallback?: string;
  startMode: SlaStartMode;
  targetLabel: string;
  durationSeconds: number;
  pauseReason?: string;
  startCondition: SlaCondition;
  pauseCondition?: SlaCondition;
  stopCondition?: SlaCondition;
  cancelCondition?: SlaCondition;
  thresholdPercentages: number[];
  alsoCompletesDefinitionIds?: string[];
}

export interface SearchSlaDefinitionsResponse {
  definitions: SlaDefinition[];
  total: number;
  limit: number;
  offset: number;
}

export type SlaNotificationChannel = "email" | "webhook";

export interface SlaNotificationRecipients {
  assignee: boolean;
  team: boolean;
  users?: string[];
  groups?: string[];
}

export interface SlaNotificationRule {
  id: string;
  definitionId?: string;
  thresholdPercentage?: number;
  channel: SlaNotificationChannel;
  recipients: SlaNotificationRecipients;
  webhookUrl?: string;
  isActive: boolean;
  createdOn: string;
  updatedOn: string;
}

export interface UpsertSlaNotificationRuleRequest {
  definitionId?: string;
  thresholdPercentage?: number;
  channel: SlaNotificationChannel;
  recipients: SlaNotificationRecipients;
  webhookUrl?: string;
  isActive?: boolean;
}
