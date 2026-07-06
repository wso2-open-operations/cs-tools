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

import { describe, expect, it } from "vitest";
import {
  ENGAGEMENTS_LIST_ENTITY_LABEL,
  ENGAGEMENTS_PAGE_SIZE,
  ENGAGEMENTS_SEARCH_PLACEHOLDER,
  ENGAGEMENTS_SORT_OPTIONS,
  ENGAGEMENTS_STAT_CARDS_CONFIG,
  ENGAGEMENTS_STAT_LABEL_ACTIVE,
  ENGAGEMENTS_STAT_LABEL_COMPLETED,
  ENGAGEMENTS_STAT_LABEL_ON_HOLD,
} from "@features/engagements/constants/engagements";
import { EngagementsSortField } from "@features/engagements/types/engagements";

describe("engagements constants", () => {
  it("exports list and search copy", () => {
    expect(ENGAGEMENTS_PAGE_SIZE).toBe(10);
    expect(ENGAGEMENTS_LIST_ENTITY_LABEL).toBe("engagements");
    expect(ENGAGEMENTS_SEARCH_PLACEHOLDER).toBe(
      "Search engagements by ID, title, or description...",
    );
  });

  it("exports sort options aligned with API fields", () => {
    expect(ENGAGEMENTS_SORT_OPTIONS.map((o) => o.value)).toEqual([
      EngagementsSortField.UpdatedOn,
      EngagementsSortField.CreatedOn,
      EngagementsSortField.State,
    ]);
  });

  it("exports stat card labels and config keys", () => {
    expect(ENGAGEMENTS_STAT_LABEL_ACTIVE).toBe("Outstanding Engagements");
    expect(ENGAGEMENTS_STAT_LABEL_COMPLETED).toBe("Completed");
    expect(ENGAGEMENTS_STAT_LABEL_ON_HOLD).toBe("On Hold");
    expect(ENGAGEMENTS_STAT_CARDS_CONFIG.map((c) => c.key)).toEqual([
      "active",
      "completed",
      "onHold",
    ]);
  });
});
