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

import type { CaseActivityEntryDto } from "./activity.dto";
import { parseBackendTimestamp } from "@utils/dateTime";

export interface CaseAuditFieldChange {
  field: string;
  fieldLabel: string;
  previousValue?: string;
  newValue?: string;
}

/** A lifecycle/field-change entry in the case's activity feed — distinct from threaded comments. */
export interface CaseAuditEntry {
  id: string;
  actor: string;
  createdOn: Date;
  changes: CaseAuditFieldChange[];
}

/** Best display name off an activity entry's flat author fields — mirrors commentAuthorLabel's
 * fallback chain in case.model.ts, adapted to this endpoint's flat (non-nested) author fields. */
function activityAuthorLabel(entry: CaseActivityEntryDto): string {
  const full = entry.createdByFullName?.trim();
  if (full) return full;
  const composed = [entry.createdByFirstName, entry.createdByLastName]
    .filter((p) => p && p.trim())
    .join(" ")
    .trim();
  if (composed) return composed;
  return entry.createdBy?.trim() || "Unknown";
}

export function toCaseAuditEntry(dto: CaseActivityEntryDto): CaseAuditEntry {
  return {
    id: dto.id,
    actor: activityAuthorLabel(dto),
    createdOn: parseBackendTimestamp(dto.createdOn),
    changes: (dto.changes ?? []).map((c) => ({
      field: c.field,
      fieldLabel: c.fieldLabel,
      previousValue: c.previousValue,
      newValue: c.newValue,
    })),
  };
}
