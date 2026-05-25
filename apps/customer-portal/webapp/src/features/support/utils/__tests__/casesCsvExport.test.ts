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
import type { CaseListItem } from "@features/support/types/cases";
import {
  ALL_CASES_CSV_HEADERS,
  buildAllCasesListCsv,
  buildCaseListExportCsv,
  CASE_LIST_EXPORT_CSV_HEADERS,
  mapCaseListExportCsvRows,
  mapCasesToCsvRows,
  resolveCaseCsvShortDescription,
  resolveCaseCsvTypeLabel,
} from "@features/support/utils/casesCsvExport";

describe("resolveCaseCsvShortDescription", () => {
  it("uses plain title when present", () => {
    expect(
      resolveCaseCsvShortDescription({
        title: "Login issue",
        description: "<p>ignored</p>",
      } as CaseListItem),
    ).toBe("Login issue");
  });
});

describe("mapCasesToCsvRows", () => {
  it("maps case fields to CSV columns", () => {
    const rows = mapCasesToCsvRows([
      {
        number: "CS-1",
        internalId: "INT-1",
        title: "Short desc",
        description: "",
        status: { id: "1", label: "Open" },
        severity: { id: "10", label: "Level 1 - Critical" },
        createdBy: "user@example.com",
        assignedEngineer: { id: "2", label: "Engineer A" },
        createdOn: "2026-01-15 10:00:00",
        updatedOn: "2026-01-16 11:00:00",
      } as CaseListItem,
    ]);

    expect(rows[0][0]).toBe("CS-1");
    expect(rows[0][1]).toBe("INT-1");
    expect(rows[0][2]).toBe("Open");
    expect(rows[0][3]).toBe("Short desc");
    expect(rows[0][5]).toBe("user@example.com");
    expect(rows[0][6]).toBe("Engineer A");
  });
});

describe("resolveCaseCsvTypeLabel", () => {
  it("prefers engagement type label", () => {
    expect(
      resolveCaseCsvTypeLabel({
        engagementType: { id: "1", label: "Consultancy" },
        type: { id: "2", label: "Other" },
      } as CaseListItem),
    ).toBe("Consultancy");
  });
});

describe("mapCaseListExportCsvRows", () => {
  it("includes type column instead of severity", () => {
    const rows = mapCaseListExportCsvRows([
      {
        number: "CS-2",
        title: "Title",
        type: { id: "1", label: "Service Request" },
      } as CaseListItem,
    ]);
    expect(rows[0][4]).toBe("Service Request");
  });
});

describe("buildAllCasesListCsv", () => {
  it("includes required headers", () => {
    const csv = buildAllCasesListCsv([]);
    expect(csv.startsWith(ALL_CASES_CSV_HEADERS.join(","))).toBe(true);
  });
});

describe("buildCaseListExportCsv", () => {
  it("includes type column headers", () => {
    const csv = buildCaseListExportCsv([]);
    expect(csv.startsWith(CASE_LIST_EXPORT_CSV_HEADERS.join(","))).toBe(true);
  });
});
