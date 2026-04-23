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

import {
  CircleAlert,
  CircleCheck,
  Clock,
} from "@wso2/oxygen-ui-icons-react";
import type { SupportStatConfig } from "@features/support/constants/supportConstants";
import {
  EngagementsSortField,
  type EngagementsSortOption,
  type EngagementsStatKey,
} from "@features/engagements/types/engagements";

/** Page size for engagements infinite list (must match API `limit` in useGetProjectCases). */
export const ENGAGEMENTS_PAGE_SIZE = 10;

/** Search input placeholder on the engagements list. */
export const ENGAGEMENTS_SEARCH_PLACEHOLDER =
  "Search engagements by ID, title, or description...";

/** Entity label for list results and empty states. */
export const ENGAGEMENTS_LIST_ENTITY_LABEL = "engagements";

/** Entity name passed to stat grid error / loading copy. */
export const ENGAGEMENTS_STAT_GRID_ENTITY_NAME = "engagement statistics";

/** Stat card labels (Engagements overview). */
export const ENGAGEMENTS_STAT_LABEL_ACTIVE = "Outstanding Engagements";
export const ENGAGEMENTS_STAT_LABEL_COMPLETED = "Completed";
export const ENGAGEMENTS_STAT_LABEL_ON_HOLD = "On Hold";

/** Stat card layout for {@link EngagementsStatCards} (icons + keys + labels). */
export const ENGAGEMENTS_STAT_CARDS_CONFIG: SupportStatConfig<EngagementsStatKey>[] =
  [
    {
      icon: Clock,
      iconColor: "info",
      key: "active",
      label: ENGAGEMENTS_STAT_LABEL_ACTIVE,
    },
    {
      icon: CircleCheck,
      iconColor: "success",
      key: "completed",
      label: ENGAGEMENTS_STAT_LABEL_COMPLETED,
    },
    {
      icon: CircleAlert,
      iconColor: "warning",
      key: "onHold",
      label: ENGAGEMENTS_STAT_LABEL_ON_HOLD,
    },
  ];

/** Sort dropdown options (value ↔ API field). */
export const ENGAGEMENTS_SORT_OPTIONS: readonly EngagementsSortOption[] = [
  { value: EngagementsSortField.CreatedOn, label: "Created on" },
  { value: EngagementsSortField.UpdatedOn, label: "Updated on" },
  { value: EngagementsSortField.State, label: "Status" },
];
