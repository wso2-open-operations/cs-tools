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

import type { BeOutageStatus, BeOutageType } from "@api/backend/types";

export interface OutageFilters {
  search: string;
  types: BeOutageType[];
  statuses: BeOutageStatus[];
  publishedOnly: boolean;
}

export const DEFAULT_OUTAGE_FILTERS: OutageFilters = {
  search: "",
  types: [],
  statuses: [],
  publishedOnly: false,
};

const TYPE_LABELS: Record<string, string> = {
  outage: "Outage",
  degradation: "Degradation",
  planned: "Planned",
};

/** Falls back to the raw value, title-cased, for a type the FE doesn't
 * curate — the backing choice list is configurable independently of this
 * portal (see `BeOutageType`'s doc comment). */
export function outageTypeLabel(type?: string | null): string {
  if (!type) return "—";
  return TYPE_LABELS[type] ?? type.charAt(0).toUpperCase() + type.slice(1);
}

export function outageTypeColor(
  type?: string | null,
): "default" | "error" | "warning" | "info" {
  switch (type) {
    case "outage":
      return "error";
    case "degradation":
      return "warning";
    case "planned":
      return "info";
    default:
      return "default";
  }
}

const STATUS_LABELS: Record<string, string> = {
  in_progress: "In progress",
  resolved: "Resolved",
};

/** `status` is always derived from `end` (never stored) — see `BeOutage`'s
 * doc comment — so this only needs to cover the two documented values. */
export function outageStatusLabel(status?: string | null): string {
  if (!status) return "—";
  return STATUS_LABELS[status] ?? status;
}

export function outageStatusColor(status?: string | null): "error" | "success" | "default" {
  if (status === "in_progress") return "error";
  if (status === "resolved") return "success";
  return "default";
}
