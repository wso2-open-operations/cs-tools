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

import type { ChangeRequestStatsResponse } from "@features/operations/types/changeRequests";
import type { ProjectCasesStats } from "@features/support/types/cases";

/**
 * Whether the combined cases + change-request dashboard card is still loading data.
 */
export function computeCrCardIsCardLoading(
  includeCrStats: boolean,
  combinedCasesStats: ProjectCasesStats | undefined,
  changeRequestStats: ChangeRequestStatsResponse | undefined,
  isCombinedCasesLoading: boolean,
  isChangeRequestStatsLoading: boolean,
  isErrorCombinedCases: boolean,
  isErrorChangeRequestStats: boolean,
): boolean {
  switch (includeCrStats) {
    case true:
      return (
        !isErrorCombinedCases &&
        !isErrorChangeRequestStats &&
        ((!combinedCasesStats && isCombinedCasesLoading) ||
          (!changeRequestStats && isChangeRequestStatsLoading))
      );
    case false:
      return (
        !isErrorCombinedCases && isCombinedCasesLoading && !combinedCasesStats
      );
  }
}

/**
 * Whether the combined cases + change-request dashboard card is in an error state.
 */
export function computeCrCardIsCardError(
  includeCrStats: boolean,
  isCardLoading: boolean,
  combinedCasesStats: ProjectCasesStats | undefined,
  changeRequestStats: ChangeRequestStatsResponse | undefined,
  isErrorCombinedCases: boolean,
  isErrorChangeRequestStats: boolean,
): boolean {
  switch (includeCrStats) {
    case true:
      return (
        !isCardLoading &&
        (isErrorCombinedCases ||
          isErrorChangeRequestStats ||
          !combinedCasesStats ||
          !changeRequestStats)
      );
    case false:
      return !isCardLoading && (isErrorCombinedCases || !combinedCasesStats);
  }
}
