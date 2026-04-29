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

import type { EntityReference, ModeType, OfStatusModeType } from "@shared/types/common.types";
import type { ItemCardProps } from "@components/support";

const ALLOWED_STATUS_FILTERS: Record<OfStatusModeType["status"], number[]> = {
  action_required: [18, 6],
  outstanding: [1, 10, 6, 1006],
  resolved: [3],
};

export function getAllowedFilters(filters: EntityReference[], mode?: ModeType): EntityReference[] {
  if (!mode) return filters;

  switch (mode.type) {
    case "status": {
      const allowed = ALLOWED_STATUS_FILTERS[mode.status];
      if (!allowed) return filters;
      return filters.filter((filter) => allowed.includes(Number(filter.id)));
    }

    default:
      return filters;
  }
}

export const STATUS_MODE_TYPES: Partial<Record<OfStatusModeType["status"], Exclude<ItemCardProps["type"], "chat">[]>> =
  {
    action_required: ["case", "service", "change", "sra", "engagement"],
    outstanding: ["case", "service", "change", "sra", "engagement"],
    resolved: ["case", "service", "change", "sra", "engagement"],
  };
