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
import { generateChangeRequestsSchedulePdf } from "@features/operations/utils/changeRequestsSchedulePdf";

const saveMock = vi.fn();
const textMock = vi.fn();
const autoTableMock = vi.fn();

vi.mock("jspdf", () => ({
  jsPDF: vi.fn().mockImplementation(function MockJsPDF(this: {
    setFontSize: ReturnType<typeof vi.fn>;
    text: ReturnType<typeof vi.fn>;
    save: ReturnType<typeof vi.fn>;
    getNumberOfPages: ReturnType<typeof vi.fn>;
    setPage: ReturnType<typeof vi.fn>;
    getTextWidth: ReturnType<typeof vi.fn>;
    internal: {
      pageSize: { getWidth: ReturnType<typeof vi.fn>; getHeight: ReturnType<typeof vi.fn> };
    };
  }) {
    this.setFontSize = vi.fn();
    this.text = textMock;
    this.save = saveMock;
    this.getNumberOfPages = vi.fn(() => 1);
    this.setPage = vi.fn();
    this.getTextWidth = vi.fn(() => 40);
    this.internal = {
      pageSize: { getWidth: vi.fn(() => 210), getHeight: vi.fn(() => 297) },
    };
  }),
}));

vi.mock("jspdf-autotable", () => ({
  default: (...args: unknown[]) => autoTableMock(...args),
}));

describe("generateChangeRequestsSchedulePdf", () => {
  it("builds schedule PDF and saves", () => {
    generateChangeRequestsSchedulePdf(
      [
        {
          number: "CHG1",
          title: "Patch",
          startDate: "2026-06-01 10:00:00",
          endDate: "2026-06-01 12:00:00",
          state: { id: "1", label: "Scheduled" },
          impact: { id: "3", label: "3 - Low" },
        } as never,
      ],
      {
        awaitingYourAction: 1,
        ongoing: 2,
        completed: 3,
        totalRequests: 6,
      },
    );

    expect(textMock).toHaveBeenCalledWith("Change Requests Schedule", 14, 20);
    expect(autoTableMock).toHaveBeenCalled();
    expect(saveMock).toHaveBeenCalledWith(
      expect.stringMatching(/^Change-Requests-Schedule-\d{4}-\d{2}-\d{2}\.pdf$/),
    );
  });
});
