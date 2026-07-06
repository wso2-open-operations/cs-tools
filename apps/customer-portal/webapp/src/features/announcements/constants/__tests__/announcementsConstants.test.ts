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
  ANNOUNCEMENT_CASE_STATE_ALLOWED_VALUES,
  ANNOUNCEMENT_DETAILS_DESCRIPTION_HEADING,
  ANNOUNCEMENTS_PAGE_SIZE,
  ANNOUNCEMENTS_PAGE_TITLE,
  ANNOUNCEMENTS_SEARCH_PLACEHOLDER,
  ANNOUNCEMENTS_SORT_FIELD_OPTIONS,
} from "@features/announcements/constants/announcementsConstants";
import { AnnouncementSortField } from "@features/announcements/types/announcements";

describe("announcementsConstants", () => {
  it("exports page copy and pagination", () => {
    expect(ANNOUNCEMENTS_PAGE_TITLE).toBe("Announcements");
    expect(ANNOUNCEMENTS_SEARCH_PLACEHOLDER).toBe("Search announcements...");
    expect(ANNOUNCEMENTS_PAGE_SIZE).toBe(10);
  });

  it("exports sort field options", () => {
    expect(ANNOUNCEMENTS_SORT_FIELD_OPTIONS.map((o) => o.value)).toEqual([
      AnnouncementSortField.UpdatedOn,
      AnnouncementSortField.State,
    ]);
  });

  it("exports filter and detail constants", () => {
    expect(ANNOUNCEMENT_CASE_STATE_ALLOWED_VALUES).toEqual(["1", "3"]);
    expect(ANNOUNCEMENT_DETAILS_DESCRIPTION_HEADING).toBe("Description");
  });
});
