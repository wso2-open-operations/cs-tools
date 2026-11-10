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

/**
 * Alert mute rules: a one-time window during which SRE alerts matching every
 * condition are muted. A muted alert is still recorded by alert-core but
 * creates no incident and pages nobody; one still firing when the rule ends
 * becomes an incident then. Stored by entity-service (`/alert-mute-rules`).
 */

/** The alert fields a condition can match, as alert-core names them. */
export type AlertMuteRuleField =
  | "service"
  | "environment"
  | "source"
  | "metric_name"
  | "severity"
  | "category"
  | "unique_identifier";

/** field = value. Values match ignoring case; environments also match their aliases (prod = production). */
export interface AlertMuteRuleCondition {
  field: AlertMuteRuleField;
  value: string;
}

export interface AlertMuteRule {
  id: string;
  /** "AMR0000001". */
  number: string;
  name: string;
  /** Optional context, up to 1024 characters. */
  description: string;
  /** IANA zone the start and end were entered in, e.g. "Asia/Colombo". */
  timeZone: string;
  /** UTC instants (RFC 3339). */
  startOn: string;
  endOn: string;
  /** Always includes `service` and `environment`. */
  conditions: AlertMuteRuleCondition[];
  createdBy: string;
  createdOn: string;
  updatedBy: string;
  updatedOn: string;
  /** Set when an engineer ended the rule before its end time. */
  endedBy: string | null;
  endedOn: string | null;
}

/** Where a rule is in its life, derived from its times (never stored). */
export type AlertMuteRuleState = "scheduled" | "active" | "expired" | "ended";

/** The list's state filter. "expired" also covers rules ended early. */
export type AlertMuteRuleStateFilter = "all" | "active" | "scheduled" | "expired";

/** What create and edit send. */
export interface AlertMuteRuleInput {
  name: string;
  description: string;
  timeZone: string;
  startOn: string;
  endOn: string;
  conditions: AlertMuteRuleCondition[];
}

export interface AlertMuteRuleSearchPayload {
  filters: { states?: Array<Exclude<AlertMuteRuleStateFilter, "all">>; searchQuery?: string };
  pagination: { offset: number; limit: number };
}

export interface AlertMuteRuleSearchResponse {
  alertMuteRules: AlertMuteRule[];
  total: number;
  offset: number;
  limit: number;
}
