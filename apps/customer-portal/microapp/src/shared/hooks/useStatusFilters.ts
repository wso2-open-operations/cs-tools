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

import type { ModeType } from "@shared/types/common.types";
import { useResolvedDateRange } from "@shared/hooks/useResolvedDateRange";

type StatusFiltersConfig = {
  actionRequired: number[];
  outstanding: number[];
  resolved: number[];
};

export function useStatusFilters(
  mode: ModeType | undefined,
  filter: string,
  search: string,
  config: StatusFiltersConfig,
) {
  const resolvedDateRange = useResolvedDateRange(mode);
  const filters: {
    statusIds?: number[];
    closedStartDate?: string;
    closedEndDate?: string;
    searchQuery?: string;
  } = {};

  if (mode?.type === "status") {
    switch (mode.status) {
      case "action_required":
        filters.statusIds = config.actionRequired;
        break;
      case "outstanding":
        filters.statusIds = config.outstanding;
        break;
      case "resolved":
        filters.statusIds = config.resolved;
        filters.closedStartDate = resolvedDateRange?.closedStartDate;
        filters.closedEndDate = resolvedDateRange?.closedEndDate;
        break;
    }
  }

  if (filter !== "all") filters.statusIds = [Number(filter)];
  if (search) filters.searchQuery = search;

  return filters;
}
