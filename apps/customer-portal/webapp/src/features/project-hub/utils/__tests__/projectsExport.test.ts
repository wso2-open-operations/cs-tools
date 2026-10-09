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

import { beforeEach, describe, expect, it, vi } from "vitest";

const csv = vi.hoisted(() => ({
  buildCsvContent: vi.fn<(headers: string[], rows: string[][]) => string>(() => "csv"),
  downloadCsvFile: vi.fn(),
}));
vi.mock("@utils/csv", () => csv);
vi.mock("@utils/pdf", () => ({ downloadPdfFile: vi.fn() }));

import {
  downloadProjectListCsv,
  fetchAllProjectsForExport,
  type AuthFetchFn,
} from "@features/project-hub/utils/projectsExport";

const BASE_URL = "https://api.test";

function setBaseUrl(value: string | undefined): void {
  (window as unknown as { config?: Record<string, string | undefined> }).config = {
    CUSTOMER_PORTAL_BACKEND_BASE_URL: value,
  };
}

function project(i: number) {
  return {
    id: `p${i}`,
    key: `KEY${i}`,
    name: `Project ${i}`,
    closureState: "open",
    startDate: null,
    endDate: null,
    activeChatsCount: 1,
    actionRequiredCount: 2,
    outstandingCount: 835,
  };
}

function page(from: number, count: number, total: number) {
  return {
    ok: true,
    json: async () => ({
      query: "",
      projectsTotal: total,
      casesTotal: 0,
      projects: Array.from({ length: count }, (_, i) => project(from + i)),
      cases: [],
    }),
  } as Response;
}

function bodyOf(call: unknown[]): Record<string, unknown> {
  return JSON.parse((call[1] as RequestInit).body as string);
}

describe("fetchAllProjectsForExport", () => {
  beforeEach(() => {
    setBaseUrl(BASE_URL);
    csv.buildCsvContent.mockClear();
    csv.downloadCsvFile.mockClear();
  });

  it("asks the same endpoint the Projects tables use, projects only, so the counts are real", async () => {
    const authFetch = vi.fn().mockResolvedValue(page(0, 3, 3));

    const rows = await fetchAllProjectsForExport(authFetch as unknown as AuthFetchFn);

    expect(authFetch).toHaveBeenCalledTimes(1);
    const call = authFetch.mock.calls[0];
    // Not /projects/search: that returns no Action Required / Outstanding counts at all.
    expect(call[0]).toBe(`${BASE_URL}/search`);
    expect((call[1] as RequestInit).method).toBe("POST");
    expect(bodyOf(call)).toEqual({
      filters: { types: ["projects"] },
      projectsPagination: { offset: 0, limit: 50 },
    });
    expect(rows).toHaveLength(3);
    expect(rows[0]).toMatchObject({ actionRequiredCount: 2, outstandingCount: 835 });
  });

  it("sends the trimmed search term, and none when it is blank", async () => {
    const withTerm = vi.fn().mockResolvedValue(page(0, 1, 1));
    await fetchAllProjectsForExport(withTerm as unknown as AuthFetchFn, "  choreo  ");
    expect(bodyOf(withTerm.mock.calls[0]).filters).toEqual({
      types: ["projects"],
      searchQuery: "choreo",
    });

    const blank = vi.fn().mockResolvedValue(page(0, 1, 1));
    await fetchAllProjectsForExport(blank as unknown as AuthFetchFn, "   ");
    expect(bodyOf(blank.mock.calls[0]).filters).toEqual({ types: ["projects"] });
  });

  it("walks every page until the total is reached", async () => {
    const authFetch = vi
      .fn()
      .mockResolvedValueOnce(page(0, 50, 120))
      .mockResolvedValueOnce(page(50, 50, 120))
      .mockResolvedValueOnce(page(100, 20, 120));

    const rows = await fetchAllProjectsForExport(authFetch as unknown as AuthFetchFn);

    expect(authFetch).toHaveBeenCalledTimes(3);
    expect(authFetch.mock.calls.map((c) => (bodyOf(c).projectsPagination as { offset: number }).offset)).toEqual([
      0, 50, 100,
    ]);
    expect(rows).toHaveLength(120);
    expect(rows[119].key).toBe("KEY119");
  });

  it("advances by the rows a page really held, so a short page never skips rows", async () => {
    // Asked for 50, got 30 each time: the next offset is 30, then 60, not 50 and 100.
    const authFetch = vi
      .fn()
      .mockResolvedValueOnce(page(0, 30, 70))
      .mockResolvedValueOnce(page(30, 30, 70))
      .mockResolvedValueOnce(page(60, 10, 70));

    const rows = await fetchAllProjectsForExport(authFetch as unknown as AuthFetchFn);

    expect(authFetch.mock.calls.map((c) => (bodyOf(c).projectsPagination as { offset: number }).offset)).toEqual([
      0, 30, 60,
    ]);
    expect(rows.map((r) => r.key)).toEqual(Array.from({ length: 70 }, (_, i) => `KEY${i}`));
  });

  it("stops on an empty page even when the reported total is higher", async () => {
    const authFetch = vi
      .fn()
      .mockResolvedValueOnce(page(0, 50, 500))
      .mockResolvedValueOnce(page(50, 0, 500));

    const rows = await fetchAllProjectsForExport(authFetch as unknown as AuthFetchFn);

    expect(authFetch).toHaveBeenCalledTimes(2);
    expect(rows).toHaveLength(50);
  });

  it("fails when the request fails, and when the backend URL is not configured", async () => {
    const failing = vi.fn().mockResolvedValue({ ok: false, statusText: "Bad Gateway" } as Response);
    await expect(fetchAllProjectsForExport(failing as unknown as AuthFetchFn)).rejects.toThrow(
      "Error fetching projects for export: Bad Gateway",
    );

    setBaseUrl(undefined);
    const never = vi.fn();
    await expect(fetchAllProjectsForExport(never as unknown as AuthFetchFn)).rejects.toThrow(
      "CUSTOMER_PORTAL_BACKEND_BASE_URL is not configured",
    );
    expect(never).not.toHaveBeenCalled();
  });
});

describe("downloadProjectListCsv", () => {
  it("writes the counts it was given, and 0 only when there are none", () => {
    downloadProjectListCsv([
      { key: "A", name: "Alpha", closureState: "open", actionRequiredCount: 2, outstandingCount: 835 },
      { key: "B", name: "Beta" },
    ]);

    const rows = csv.buildCsvContent.mock.calls[0][1];
    expect(rows[0].slice(0, 3)).toEqual(["A", "Alpha", "Open"]);
    expect(rows[0].slice(5)).toEqual(["2", "835"]);
    expect(rows[1].slice(2)).toEqual(["Active", "--", "--", "0", "0"]);
    expect(csv.downloadCsvFile).toHaveBeenCalledTimes(1);
  });

  it("prints the status the way the export always has: title-cased, underscores as spaces", () => {
    csv.buildCsvContent.mockClear();
    downloadProjectListCsv([
      { key: "A", name: "A", closureState: "open" },
      { key: "B", name: "B", closureState: "read_only" },
      { key: "C", name: "C", closureState: "pending_notified" },
      { key: "D", name: "D", closureState: "SUSPENDED" },
      { key: "E", name: "E", closureState: "" },
      { key: "F", name: "F", closureState: null },
    ]);

    const statuses = csv.buildCsvContent.mock.calls[0][1].map((r) => r[2]);
    expect(statuses).toEqual(["Open", "Read Only", "Pending Notified", "Suspended", "Active", "Active"]);
  });
});
