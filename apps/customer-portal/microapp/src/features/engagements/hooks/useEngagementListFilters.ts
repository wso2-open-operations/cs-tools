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
import { useStatusFilters } from "@shared/hooks/useStatusFilters";
import {
  ACTION_REQUIRED_CASE_STATUS_IDS,
  OUTSTANDING_CASE_STATUS_IDS,
  RESOLVED_CASE_STATUS_IDS,
} from "@shared/constants/status.constants";
import type { GetCasesRequestDto } from "@features/cases/types/case.dto";

export function useEngagementListFilters(
  mode: ModeType | undefined,
  filter: string,
  search: string,
): GetCasesRequestDto["filters"] {
  return useStatusFilters(mode, filter, search, {
    actionRequired: ACTION_REQUIRED_CASE_STATUS_IDS,
    outstanding: OUTSTANDING_CASE_STATUS_IDS,
    resolved: RESOLVED_CASE_STATUS_IDS,
  });
}
