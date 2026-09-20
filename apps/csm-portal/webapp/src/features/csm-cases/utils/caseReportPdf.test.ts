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
import type {
  CaseAttachment,
  CaseAuditEntry,
  CaseFeedbackEntry,
  CsmCaseDetail,
  CsmCaseComment,
} from "@features/csm-cases/types/csmCases";

// `jsPDF` mixes most of its API onto each *instance* at construction time
// (a plugin architecture), not onto `jsPDF.prototype` — so `vi.spyOn` can't
// intercept it after the fact. Faking the whole module is the practical way
// to unit-test the report's *content/logic* (every comment gets written,
// HTML gets stripped, the filename is right) without asserting on real PDF
// byte output, which isn't what this module's own correctness hinges on.
const state = vi.hoisted(() => ({
  drawnStrings: [] as string[],
  savedFileNames: [] as string[],
}));

vi.mock("jspdf", () => {
  class FakeJsPDF {
    private pageCount = 1;
    internal = { getNumberOfPages: () => this.pageCount };
    lastAutoTable?: { finalY: number };
    setFontSize() {
      return this;
    }
    setFont() {
      return this;
    }
    setTextColor() {
      return this;
    }
    setDrawColor() {
      return this;
    }
    setFillColor() {
      return this;
    }
    setLineWidth() {
      return this;
    }
    rect() {
      return this;
    }
    splitTextToSize(text: string) {
      return [text];
    }
    text(value: string) {
      state.drawnStrings.push(value);
      return this;
    }
    line() {
      return this;
    }
    setPage() {
      return this;
    }
    addPage() {
      this.pageCount += 1;
      return this;
    }
    save(filename: string) {
      state.savedFileNames.push(filename);
      return this;
    }
  }
  return { jsPDF: FakeJsPDF };
});

// `writeDetailsTable`/`writeCommentsTable` (@utils/pdfReportKit) delegate to
// the real `jspdf-autotable`, which drives the (faked) jsPDF instance through
// far more of its API than this test doubles up above — faking this module
// too, recording each cell's text the same way the fake `text()` does above,
// keeps the same "assert on the text that ended up in the document"
// approach working for the table-based sections.
vi.mock("jspdf-autotable", () => ({
  default: (doc: { lastAutoTable?: { finalY: number } }, options: { body?: unknown[][] }) => {
    for (const row of options.body ?? []) {
      for (const cell of row) state.drawnStrings.push(String(cell));
    }
    doc.lastAutoTable = { finalY: 40 };
  },
}));

const { generateCaseReportPdf } = await import("./caseReportPdf");

function baseCase(overrides: Partial<CsmCaseDetail> = {}): CsmCaseDetail {
  return {
    id: "case-uuid-1",
    caseNumber: "CS-1007",
    subject: "Cluster keeps restarting",
    customer: "Acme Corp",
    accountId: "acc-1",
    projectId: "proj-1",
    projectName: "Acme Platform",
    product: "WSO2 Identity Server",
    severity: "S1",
    state: "work_in_progress",
    assignee: "Jane Doe",
    assigneeIsMe: false,
    slaClockType: "none",
    minutesToBreach: 0,
    createdAt: "2026-07-01T00:00:00Z",
    updatedAt: "2026-07-02T00:00:00Z",
    description: "<p>The cluster <b>restarts</b> every hour.</p>",
    assignmentGroup: "Tier 2",
    ...overrides,
  } as CsmCaseDetail;
}

function comment(overrides: Partial<CsmCaseComment> = {}): CsmCaseComment {
  return {
    id: "c1",
    caseId: "case-uuid-1",
    authorName: "Jane Doe",
    authorRole: "wso2_engineer",
    bodyHtml: "<p>Looking into it now.</p>",
    createdAt: "2026-07-01T01:00:00Z",
    ...overrides,
  };
}

describe("generateCaseReportPdf", () => {
  beforeEach(() => {
    state.drawnStrings = [];
    state.savedFileNames = [];
  });

  it("saves a file named after the case number", () => {
    generateCaseReportPdf(baseCase(), [comment()]);
    expect(state.savedFileNames).toEqual(["case-CS-1007.pdf"]);
  });

  it("strips HTML markup from the description and comment bodies", () => {
    generateCaseReportPdf(baseCase(), [comment()]);
    const joined = state.drawnStrings.join(" ");
    expect(joined).not.toMatch(/<[a-z][\s\S]*>/i);
    expect(joined).toContain("The cluster restarts every hour.");
    expect(joined).toContain("Looking into it now.");
  });

  it("includes every comment, not just the first few", () => {
    const many = Array.from({ length: 80 }, (_, i) =>
      comment({ id: `c${i}`, bodyHtml: `<p>Comment number ${i}</p>` }),
    );
    generateCaseReportPdf(baseCase(), many);
    const joined = state.drawnStrings.join(" ");
    expect(joined).toContain("Comment number 0");
    expect(joined).toContain("Comment number 79");
  });

  it("renders a 'No activity.' placeholder when the case has no comments or audit entries", () => {
    generateCaseReportPdf(baseCase(), []);
    expect(state.drawnStrings.some((s) => s.includes("No activity."))).toBe(true);
  });

  it("excludes a sync-only entry whose body is just the backend's 'Customer comment added' label, and strips that label from a real comment that also carries it", () => {
    const labelOnly = comment({
      id: "label-only",
      authorName: "system",
      bodyHtml: "<p>Customer comment added</p>",
    });
    const labelPlusReal = comment({
      id: "label-plus-real",
      authorName: "Nadeesha Silva",
      bodyHtml: "Customer comment added<p>The fix has been deployed.</p>",
    });
    generateCaseReportPdf(baseCase(), [labelOnly, labelPlusReal]);
    const joined = state.drawnStrings.join(" ");
    expect(joined).toContain("Activity (1 entries)");
    expect(joined).not.toContain("Customer comment added");
    expect(joined).toContain("The fix has been deployed.");
  });

  it("preserves paragraph breaks in a multi-paragraph description instead of running them together", () => {
    // Reported live: a real multi-paragraph description rendered as one
    // run-on wall of text in the PDF while looking correctly paragraph-broken
    // on screen.
    const html = "<p>First paragraph.</p><p>Second paragraph.</p>";
    generateCaseReportPdf(baseCase({ description: html }), []);
    const descriptionCall = state.drawnStrings.find((s) => s.includes("First paragraph."));
    expect(descriptionCall).toBeDefined();
    expect(descriptionCall).toContain("First paragraph.\nSecond paragraph.");
  });

  it("renders the activity trail as a plain list, not a table (no 'When'/'Actor' column header text)", () => {
    // Reported live: a table cell has to fit the single longest wrapped
    // comment, which ballooned the report once a few long comments were
    // involved — this was switched from a `jspdf-autotable` table to a plain
    // list specifically to avoid that.
    generateCaseReportPdf(baseCase(), [comment()]);
    expect(state.drawnStrings).not.toContain("When");
    expect(state.drawnStrings).not.toContain("Actor");
  });

  it("merges the case's audited state/field changes into the same activity list as comments, in chronological order", () => {
    const audit: CaseAuditEntry[] = [
      {
        id: "audit-1",
        kind: "field_change",
        actor: "Jane Doe",
        createdAt: "2026-07-01T00:30:00Z",
        changes: [{ field: "state", fieldLabel: "State", previousValue: "New", newValue: "Work in Progress" }],
      },
    ];
    generateCaseReportPdf(baseCase(), [comment({ createdAt: "2026-07-01T01:00:00Z" })], audit);
    const joined = state.drawnStrings.join(" ");
    expect(joined).toContain("Activity (2 entries)");
    expect(joined).toContain("State: New");
    expect(joined).toContain("Work in Progress");
    expect(joined).toContain("State change");
  });

  it("includes attachments and CSAT feedback in the same merged activity table", () => {
    const attachments: CaseAttachment[] = [
      {
        id: "att-1",
        filename: "screenshot.png",
        size: 2048,
        contentType: "image/png",
        uploadedBy: "Jane Doe",
        uploadedAt: "2026-07-01T00:15:00Z",
      },
    ];
    const feedback: CaseFeedbackEntry[] = [
      {
        id: "fb-1",
        rating: 5,
        ratingLabel: "Very satisfied",
        comment: "Great support!",
        submittedAt: "2026-07-03T00:00:00Z",
        submitterName: "Alex Customer",
      },
    ];
    generateCaseReportPdf(baseCase(), [comment()], [], attachments, feedback);
    const joined = state.drawnStrings.join(" ");
    expect(joined).toContain("Activity (3 entries)");
    expect(joined).toContain("screenshot.png");
    expect(joined).toContain("2.0 KB");
    expect(joined).toContain("Attachment");
    expect(joined).toContain("Very satisfied");
    expect(joined).toContain("Great support!");
    expect(joined).toContain("Feedback");
    expect(joined).toContain("Alex Customer");
  });
});
