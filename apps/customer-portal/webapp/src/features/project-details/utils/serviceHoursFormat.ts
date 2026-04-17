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

import { PROJECT_DETAILS_NOT_AVAILABLE_DISPLAY } from "@features/project-details/constants/projectDetailsConstants";
import { formatServiceHoursDecimalAsHrMin } from "@features/project-details/utils/projectDetails";

/**
 * Formats consumed/total service hours for stat cards (e.g. `1h 30m/4h 0m (38%)`).
 *
 * @param consumed - Consumed hours (optional).
 * @param total - Total allocated hours (optional).
 * @returns Display string or {@link PROJECT_DETAILS_NOT_AVAILABLE_DISPLAY}.
 */
export function formatServiceHoursAllocationDisplay(
  consumed: number | undefined,
  total: number | undefined,
): string {
  if (consumed == null && total == null) {
    return PROJECT_DETAILS_NOT_AVAILABLE_DISPLAY;
  }
  const c = Number(consumed ?? 0);
  const t = Number(total ?? 0);
  const pct = t === 0 ? 0 : Math.round((c / t) * 100);
  return `${formatServiceHoursDecimalAsHrMin(c)}/${formatServiceHoursDecimalAsHrMin(t)} (${pct}%)`;
}
