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

import { render } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import AllCasesCsvExportButton from "../AllCasesCsvExportButton";

const { caseListCsvExportButtonMock } = vi.hoisted(() => ({
  caseListCsvExportButtonMock: vi.fn(() => null),
}));

vi.mock("@features/support/components/list-export/CaseListCsvExportButton", () => ({
  default: caseListCsvExportButtonMock,
}));

describe("AllCasesCsvExportButton", () => {
  it("renders list-export button with all-cases defaults", () => {
    render(
      <AllCasesCsvExportButton
        projectId="project-1"
        caseSearchRequest={{ filters: {} } as never}
      />,
    );

    expect(caseListCsvExportButtonMock).toHaveBeenCalledWith(
      expect.objectContaining({
        projectId: "project-1",
        filenamePrefix: "cases",
        exportVariant: "allCases",
      }),
      undefined,
    );
  });
});
