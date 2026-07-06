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
import { SortOrder } from "@/types/common";
import { CaseType } from "@features/support/constants/supportConstants";
import {
  buildAnnouncementCaseSearchRequest,
  buildAnnouncementsFilterSelectOptions,
  formatAnnouncementDateDisplay,
  formatAnnouncementsClearFiltersButtonLabel,
  getAnnouncementTotalPages,
  isAnnouncementDescriptionEffectivelyEmpty,
  normalizeAnnouncementDescriptionHtml,
  resolveAnnouncementListFilterOptions,
} from "@features/announcements/utils/announcements";
import { AnnouncementFilterMetadataKey } from "@features/announcements/types/announcements";

describe("buildAnnouncementCaseSearchRequest", () => {
  it("sets announcement case type and optional filters", () => {
    const req = buildAnnouncementCaseSearchRequest(
      { statusIds: ["5"] },
      " hello ",
      SortOrder.ASC,
    );
    expect(req.filters?.caseTypes).toEqual([CaseType.ANNOUNCEMENT]);
    expect(req.filters?.statusIds).toEqual([5]);
    expect(req.filters?.searchQuery).toBe("hello");
    expect(req.sortBy?.field).toBe("updatedOn");
    expect(req.sortBy?.order).toBe(SortOrder.ASC);
  });

  it("omits search and status when empty", () => {
    const req = buildAnnouncementCaseSearchRequest({}, "", SortOrder.DESC);
    expect(req.filters?.searchQuery).toBeUndefined();
    expect(req.filters?.statusIds).toBeUndefined();
  });
});

describe("getAnnouncementTotalPages", () => {
  it("returns at least one page", () => {
    expect(getAnnouncementTotalPages(0, 10)).toBe(1);
  });

  it("ceil-divides total by page size", () => {
    expect(getAnnouncementTotalPages(25, 10)).toBe(3);
  });
});

describe("normalizeAnnouncementDescriptionHtml", () => {
  it("replaces n placeholder tags with br", () => {
    expect(normalizeAnnouncementDescriptionHtml("<n/>x")).toContain("<br />");
  });
});

describe("isAnnouncementDescriptionEffectivelyEmpty", () => {
  it("returns true for whitespace-only html", () => {
    expect(isAnnouncementDescriptionEffectivelyEmpty("<p>   </p>")).toBe(true);
  });
});

describe("formatAnnouncementsClearFiltersButtonLabel", () => {
  it("includes count in parentheses", () => {
    expect(formatAnnouncementsClearFiltersButtonLabel(3)).toBe(
      "Clear Filters (3)",
    );
  });
});

describe("resolveAnnouncementListFilterOptions", () => {
  it("filters case states to allowed values only", () => {
    const options = resolveAnnouncementListFilterOptions(
      {
        id: "statusIds",
        filterKey: "statusIds",
        metadataKey: AnnouncementFilterMetadataKey.CaseStates,
        useLabelAsValue: false,
      },
      {
        caseStates: [
          { id: "1", label: "Open" },
          { id: "2", label: "Closed" },
          { id: "3", label: "Published" },
        ],
      } as never,
    );
    expect(options.map((o) => o.value)).toEqual(["1", "3"]);
  });
});

describe("buildAnnouncementsFilterSelectOptions", () => {
  it("maps severity labels for display", () => {
    const options = buildAnnouncementsFilterSelectOptions(
      {
        id: "severityId",
        filterKey: "severityId",
        metadataKey: AnnouncementFilterMetadataKey.Severities,
        useLabelAsValue: false,
      } as never,
      {
        severities: [{ id: "10", label: "1 - Critical" }],
      } as never,
    );
    expect(options[0]?.label).toBeTruthy();
    expect(options[0]?.value).toBe("10");
  });
});

describe("formatAnnouncementDateDisplay", () => {
  it("returns placeholder for missing date", () => {
    expect(formatAnnouncementDateDisplay(null)).toBe("--");
  });

  it("formats valid ISO timestamps", () => {
    const formatted = formatAnnouncementDateDisplay("2024-06-15T10:30:00Z");
    expect(formatted).not.toBe("--");
  });
});
