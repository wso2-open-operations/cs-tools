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

export type CaseActivityType = "comment" | "attachment" | "field_change";

/** One field changed within a single audited save-transaction. */
export interface CaseActivityFieldChangeDto {
  field: string;
  fieldLabel: string;
  /** Absent/empty when the field was previously unset (a "set" change). */
  previousValue?: string;
  /** Absent/empty when the field was cleared. */
  newValue?: string;
}

/**
 * One entry from `POST /cases/{id}/activities/search`. This endpoint also returns `comment` and
 * `attachment` typed entries, but those lanes keep reading from their existing, dedicated
 * endpoints (`/cases/{id}/comments/search`, `/attachments/search`) — only `field_change` entries
 * are consumed from here, mirroring the webapp's useCsmCaseActivities.ts (it deliberately avoids
 * a second, divergent read path for comments/attachments).
 */
export interface CaseActivityEntryDto {
  id: string;
  type: CaseActivityType;
  content?: string;
  createdOn: string;
  createdBy?: string;
  createdByFirstName?: string;
  createdByLastName?: string;
  createdByFullName?: string;
  /** Only present on `type === "field_change"` entries. */
  changes?: CaseActivityFieldChangeDto[];
}

export interface CaseActivitiesSearchPayloadDto {
  pagination?: { offset?: number; limit?: number };
  /** Whether the response should include `field_change` entries. */
  includeFieldChanges?: boolean;
}

// openapi.yaml has been wrong about this app's response envelopes before (page limits, response
// shapes — see services/cases.ts) — the webapp's own working implementation reads
// `response.activity` (singular), not `activities` as the spec might claim, so this field name is
// taken from that ground truth rather than the doc.
export interface CaseActivitiesSearchResponseDto {
  activity?: CaseActivityEntryDto[];
  total?: number;
  limit?: number;
  offset?: number;
  hasMore?: boolean;
}
