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

const save = vi.fn();
const addElementPages = vi.fn();
const pages: string[] = [];

vi.mock("jspdf", () => ({
  default: class {
    save(name: string) {
      save(name);
    }
  },
}));
vi.mock("@features/csm-reports/components/reportPdfCapture", () => ({
  addElementPages: (...args: unknown[]) => addElementPages(...args),
}));
vi.mock("../assets/wso2-logo-orange.png", () => ({ default: "logo.png" }));

import { buildSlaReportSheets, slaReportFileName, slaReportPDF } from "@features/csm-reports/components/SlaReportPDF";
import type { SlaReportDetails } from "@features/csm-reports/api/reportTypes";

const metric = (over = {}) => ({ elapsed: "2 Hours", target: "4 Hours", met: 1, missed: 2, running: 3, ...over });

function report(over: Partial<SlaReportDetails> = {}): SlaReportDetails {
  return {
    projectName: "Example Project",
    projectKey: "EXP",
    periodStart: "2026-10-01",
    periodEnd: "2026-11-11",
    summary: [
      {
        label: "Incident - Critical (P1)",
        caseType: "incident",
        severity: "S1",
        cases: 4,
        response: metric(),
        workaround: metric({ elapsed: "—", target: "Not applicable", met: 0, missed: 0, running: 0 }),
        resolution: metric({ met: 0, missed: 1, running: 0 }),
      },
    ],
    cases: [
      {
        caseId: "c1",
        caseNumber: "CS0001",
        caseType: "Incident",
        severity: "Critical (P1)",
        state: "Open",
        opened: "2026-10-05T08:00:00Z",
        response: { elapsed: "1 Hour", status: "Met" },
        workaround: { elapsed: "—", status: "" },
        resolution: { elapsed: "3 Days", status: "Missed" },
      },
    ],
    ...over,
  };
}

describe("buildSlaReportSheets", () => {
  it("lays out a front page, the summary, the cases and a thank-you page", () => {
    const sheets = buildSlaReportSheets(report());
    expect(sheets.map((s) => s.className.split(" ")[0])).toEqual(["front-page", "sla-pdf-body", "sla-pdf-body", "thank-page"]);
    expect(sheets.every((s) => s.classList.contains("pdf-sheet"))).toBe(true);
  });

  it("puts the project, key, period and date on the front page", () => {
    const front = buildSlaReportSheets(report(), new Date(2026, 10, 11))[0];
    expect(front.querySelector(".header-title")?.textContent).toBe("Example Project - SLA Report");
    expect(front.textContent).toContain("Project key: EXP");
    expect(front.textContent).toContain("1 Oct 2026 to 11 Nov 2026");
    expect(front.textContent).toContain("Generated on:");
    expect(front.querySelector("img")?.getAttribute("src")).toBe("logo.png");
    expect(front.style.height).toBe("790px");
  });

  it("shows the figures at a glance, summed over the three clocks", () => {
    const summary = buildSlaReportSheets(report())[1];
    const tiles = Array.from(summary.querySelectorAll(".sla-pdf-tile")).map((t) => [
      t.querySelector(".sla-pdf-tile-label")?.textContent,
      t.querySelector(".sla-pdf-tile-value")?.textContent,
    ]);
    // 1 case in the list; met 1+0+0, missed 2+0+1, running 3+0+0
    expect(tiles).toEqual([
      ["Cases", "1"],
      ["Met", "1"],
      ["Missed", "3"],
      ["Running", "3"],
    ]);
  });

  it("shows each summary cell with its percentile, target and counts", () => {
    const summary = buildSlaReportSheets(report())[1];
    const row = summary.querySelector(".sla-pdf-summary tbody tr") as HTMLElement;
    expect(row.textContent).toContain("Incident - Critical (P1)");
    expect(row.textContent).toContain("2 Hours");
    expect(row.textContent).toContain("Target: 4 Hours");
    expect(row.textContent).toContain("1 met · 2 missed · 3 running");
    expect(row.textContent).toContain("Target: Not applicable");
  });

  it("lists every case with a coloured status for each clock that has one", () => {
    const cases = buildSlaReportSheets(report())[2];
    const row = cases.querySelector(".sla-pdf-cases tbody tr") as HTMLElement;
    expect(row.textContent).toContain("CS0001");
    expect(row.textContent).toContain("5 Oct 2026");
    expect(row.querySelector(".sla-pdf-status.met")?.textContent).toBe("Met");
    expect(row.querySelector(".sla-pdf-status.missed")?.textContent).toBe("Missed");
    // the workaround clock has no status, so no pill
    expect(row.querySelectorAll(".sla-pdf-status")).toHaveLength(2);
  });

  it("says so on both tables when no case was opened", () => {
    const sheets = buildSlaReportSheets(report({ summary: [], cases: [] }));
    expect(sheets[1].textContent).toContain("No cases were opened in this period.");
    expect(sheets[2].textContent).toContain("No cases were opened in this period.");
  });

  it("escapes names and values so they can never be read as markup", () => {
    const evil = '<img src=x onerror="alert(1)">';
    const sheets = buildSlaReportSheets(
      report({
        projectName: evil,
        cases: [{ ...report().cases[0], caseNumber: evil, severity: evil, state: evil }],
      }),
    );
    for (const el of sheets) {
      expect(el.querySelector("img[onerror]")).toBeNull();
    }
    expect(sheets[0].querySelector(".header-title")?.textContent).toContain(evil);
    expect(sheets[2].querySelector(".sla-pdf-cases tbody tr")?.textContent).toContain(evil);
  });

  it("ignores an unknown clock status instead of printing it as markup or a colour", () => {
    const cases = buildSlaReportSheets(
      report({ cases: [{ ...report().cases[0], response: { elapsed: "1 Hour", status: "Hacked" as never } }] }),
    )[2];
    expect(cases.querySelector(".sla-pdf-status.hacked")).toBeNull();
    expect(cases.querySelectorAll(".sla-pdf-status")).toHaveLength(1);
  });
});

describe("slaReportFileName", () => {
  it("names the file for the project and the period", () => {
    expect(slaReportFileName(report())).toBe("SLA_Report_EXP_2026-10-01_to_2026-11-11.pdf");
  });

  it("keeps the name to safe characters", () => {
    expect(slaReportFileName(report({ projectKey: "A/B C:*" }))).toBe("SLA_Report_A_B_C___2026-10-01_to_2026-11-11.pdf");
    expect(slaReportFileName(report({ projectKey: "" }))).toBe("SLA_Report_project_2026-10-01_to_2026-11-11.pdf");
  });
});

describe("slaReportPDF", () => {
  beforeEach(() => {
    save.mockReset();
    addElementPages.mockReset();
    pages.length = 0;
    addElementPages.mockImplementation((_pdf: unknown, el: HTMLElement) => {
      pages.push(el.className.split(" ")[0]);
      return Promise.resolve(true);
    });
  });

  it("captures each sheet in turn and saves one PDF", async () => {
    await slaReportPDF(report());
    expect(pages).toEqual(["front-page", "sla-pdf-body", "sla-pdf-body", "thank-page"]);
    expect(save).toHaveBeenCalledWith("SLA_Report_EXP_2026-10-01_to_2026-11-11.pdf");
    const opts = addElementPages.mock.calls[0][3] as { width: number; pageCssHeight: number };
    expect(opts).toMatchObject({ width: 1120, pageCssHeight: 790 });
  });

  it("removes its capture frame, even when a capture fails", async () => {
    addElementPages.mockRejectedValueOnce(new Error("canvas"));
    await expect(slaReportPDF(report())).rejects.toThrow("canvas");
    expect(document.querySelector("iframe")).toBeNull();
    expect(save).not.toHaveBeenCalled();
  });

  it("paints the dividers in the accent colour, and refuses one that is not a colour", async () => {
    let divider = "";
    addElementPages.mockImplementation((_pdf: unknown, el: HTMLElement) => {
      divider = el.querySelector<HTMLElement>(".section-divider")?.style.borderTopColor ?? divider;
      return Promise.resolve(true);
    });
    await slaReportPDF(report(), "#123456");
    expect(divider).toBe("rgb(18, 52, 86)");
    divider = "";
    await slaReportPDF(report(), "red; background:url(x)");
    expect(divider).toBe("rgb(250, 123, 63)");
  });
});
