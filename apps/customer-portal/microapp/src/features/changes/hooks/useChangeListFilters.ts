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
import {
  ACTION_REQUIRED_CHANGE_REQUEST_STATUS_IDS,
  OUTSTANDING_CHANGE_REQUESTS_STATUS_IDS,
  RESOLVED_CHANGE_REQUEST_STATUS_IDS,
} from "@shared/constants/status.constants";
import type { GetChangeRequestsRequestDto } from "@features/changes/types/change.dto";

export function useChangeListFilters(
  mode: ModeType | undefined,
  filter: string,
  search: string,
): GetChangeRequestsRequestDto["filters"] {
  const resolvedDateRange = useResolvedDateRange(mode);
  const filters: GetChangeRequestsRequestDto["filters"] = {};

  if (mode?.type === "status") {
    switch (mode.status) {
      case "action_required":
        filters.stateKeys = ACTION_REQUIRED_CHANGE_REQUEST_STATUS_IDS;
        break;
      case "outstanding":
        filters.stateKeys = OUTSTANDING_CHANGE_REQUESTS_STATUS_IDS;
        break;
      case "resolved":
        filters.stateKeys = RESOLVED_CHANGE_REQUEST_STATUS_IDS;
        filters.closedStartDate = resolvedDateRange?.closedStartDate;
        filters.closedEndDate = resolvedDateRange?.closedEndDate;
        break;
    }
  }

  if (filter !== "all") filters.stateKeys = [Number(filter)];
  if (search) filters.searchQuery = search;

  return filters;
}
