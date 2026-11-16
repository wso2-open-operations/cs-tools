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
import type { ChangeRequestDetails } from "@features/operations/types/changeRequests";
import { generateChangeRequestDetailsPdf } from "@features/operations/utils/changeRequests";

// Real jsPDF, except save(): it is replaced by a capture of the generated PDF so the
// test can read what actually went into the file.
const captured = vi.hoisted(() => ({ pdfs: [] as string[] }));

vi.mock("jspdf", async (importOriginal) => {
  const actual = await importOriginal<typeof import("jspdf")>();
  const RealJsPdf = actual.jsPDF;
  function CapturingJsPdf(...args: ConstructorParameters<typeof RealJsPdf>) {
    const doc = new RealJsPdf(...args);
    doc.save = (() => {
      captured.pdfs.push(doc.output());
      return doc;
    }) as typeof doc.save;
    return doc;
  }
  return { ...actual, jsPDF: CapturingJsPdf };
});

describe("generateChangeRequestDetailsPdf", () => {
  beforeEach(() => {
    captured.pdfs.length = 0;
  });

  it("writes a WSO2 staff assignee as plain text, followed by the circled w", async () => {
    await generateChangeRequestDetailsPdf({
      number: "CHG0041408",
      assignedEngineer: { id: "u1", label: "Vimukthi Perera ⓦ" },
    } as ChangeRequestDetails);

    const pdf = captured.pdfs[0];
    expect(pdf).toMatch(/\(Vimukthi Perera\) Tj/);
    expect(pdf.match(/\(w\) Tj/g)).toHaveLength(1);
    expect(pdf).not.toContain("\u0000");
  });

  it("draws no marker for an assignee who is not WSO2 staff", async () => {
    await generateChangeRequestDetailsPdf({
      number: "CHG0041409",
      assignedEngineer: { id: "u2", label: "Jane Smith" },
    } as ChangeRequestDetails);

    const pdf = captured.pdfs[0];
    expect(pdf).toContain("(Jane Smith) Tj");
    expect(pdf).not.toContain("(w) Tj");
  });

  it("keeps the footer date readable when the browser puts U+202F before PM", async () => {
    const spy = vi
      .spyOn(Date.prototype, "toLocaleString")
      .mockReturnValue(`Oct 14, 2026, 12:27${String.fromCodePoint(0x202f)}PM`);
    try {
      await generateChangeRequestDetailsPdf({ number: "CHG0041410" } as ChangeRequestDetails);
    } finally {
      spy.mockRestore();
    }

    const pdf = captured.pdfs[0];
    expect(pdf).toContain("(Oct 14, 2026, 12:27 PM) Tj");
    expect(pdf).not.toContain("\u0000");
  });

  it("keeps the WSO2 Support 24x7 footer when no company name is configured", async () => {
    await generateChangeRequestDetailsPdf({ number: "CHG0041411" } as ChangeRequestDetails);

    expect(captured.pdfs[0]).toContain("(WSO2 Support 24x7) Tj");
  });

  it("puts the configured company name in the footer", async () => {
    const originalConfig = window.config;
    window.config = { ...originalConfig, CUSTOMER_PORTAL_COMPANY_NAME: "Acme Corp" };
    try {
      await generateChangeRequestDetailsPdf({ number: "CHG0041412" } as ChangeRequestDetails);
    } finally {
      window.config = originalConfig;
    }

    const pdf = captured.pdfs[0];
    expect(pdf).toContain("(Acme Corp Support 24x7) Tj");
    expect(pdf).not.toContain("WSO2 Support 24x7");
  });
});
