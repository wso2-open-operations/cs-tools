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

import { describe, expect, it, vi } from "vitest";
import { downloadPdfFile } from "@utils/pdf";

const saveMock = vi.fn();
const textMock = vi.fn();
const autoTableMock = vi.fn();

vi.mock("jspdf", () => ({
  default: vi.fn().mockImplementation(function MockJsPDF(this: {
    setFontSize: ReturnType<typeof vi.fn>;
    setTextColor: ReturnType<typeof vi.fn>;
    text: ReturnType<typeof vi.fn>;
    save: ReturnType<typeof vi.fn>;
  }) {
    this.setFontSize = vi.fn();
    this.setTextColor = vi.fn();
    this.text = textMock;
    this.save = saveMock;
  }),
}));

vi.mock("jspdf-autotable", () => ({
  default: (...args: unknown[]) => autoTableMock(...args),
}));

describe("downloadPdfFile", () => {
  it("builds a PDF with title and table then saves", () => {
    downloadPdfFile(
      "report.pdf",
      "Usage report",
      ["Col"],
      [["Value"]],
      { 0: { cellWidth: 40 } },
    );

    expect(textMock).toHaveBeenCalledWith("Usage report", 14, 14);
    expect(autoTableMock).toHaveBeenCalled();
    expect(saveMock).toHaveBeenCalledWith("report.pdf");
  });
});
