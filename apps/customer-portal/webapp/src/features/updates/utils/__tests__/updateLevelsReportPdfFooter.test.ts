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

import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  generateUpdateLevelsReportPdf,
  getUpdateLevelsReportData,
} from "@features/updates/utils/updateLevelsReportPdf";

const captured = vi.hoisted(() => ({ pdfs: [] as string[] }));

const companyNameMock = vi.hoisted(() => vi.fn(() => "Choreo Labs"));

// The company name is fixed in code; a long one is faked here to test shortening.
vi.mock("@config/brandingConfig", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@config/brandingConfig")>()),
  companyName: companyNameMock,
}));

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

const reportData = getUpdateLevelsReportData({
  productName: "WSO2 API Manager",
  productVersion: "4.4.0",
  startLevel: 1,
  endLevel: 2,
  data: {
    "2": {
      updateType: "regular",
      updateDescriptionLevels: [
        {
          updateLevel: 2,
          productName: "WSO2 API Manager",
          productVersion: "4.4.0",
          channel: "full",
          updateType: "regular",
          updateNumber: 100,
          description: "Fix.",
          instructions: "N/A",
          bugFixes: "[]",
          filesAdded: "[]",
          filesModified: "[]",
          filesRemoved: "[]",
          bundlesInfoChanges: null,
          dependantReleases: null,
          timestamp: 1705939200000,
          securityAdvisories: [],
        },
      ],
    },
  } as never,
});

const pageText = /\(WSO2 API Manager 4\.4\.0 - Update Summary {2}\| {2}Page 1 of \d+\) Tj/;

describe("generateUpdateLevelsReportPdf footer", () => {
  beforeEach(() => {
    captured.pdfs.length = 0;
  });

  afterEach(() => {
    companyNameMock.mockReturnValue("Choreo Labs");
  });

  it("prints the company's support line next to the page text", async () => {
    await generateUpdateLevelsReportPdf(reportData);

    const pdf = captured.pdfs[0];
    expect(pdf).toContain("(Choreo Labs Support 24x7) Tj");
    expect(pdf).toMatch(pageText);
  });

  it("shortens a company name too long to fit beside the page text", async () => {
    const longName =
      "Example International Technology and Business Services Corporation of the Americas";
    companyNameMock.mockReturnValue(longName);
    await generateUpdateLevelsReportPdf(reportData);

    const pdf = captured.pdfs[0];
    expect(pdf).toMatch(/\(Example[^)]*\.\.\. Support 24x7\) Tj/);
    expect(pdf).not.toContain(longName);
    expect(pdf).toMatch(pageText);
  });
});
