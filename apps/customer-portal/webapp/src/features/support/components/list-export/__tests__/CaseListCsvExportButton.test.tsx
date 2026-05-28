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

import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { ThemeProvider, createTheme } from "@wso2/oxygen-ui";
import { describe, expect, it, vi } from "vitest";
import CaseListCsvExportButton from "../CaseListCsvExportButton";

const { downloadCaseListCsvMock } = vi.hoisted(() => ({
  downloadCaseListCsvMock: vi.fn(),
}));
vi.mock("@hooks/useAuthApiClient", () => ({ useAuthApiClient: () => vi.fn() }));
vi.mock("@context/error-banner/ErrorBannerContext", () => ({
  useErrorBanner: () => ({ showError: vi.fn() }),
}));
vi.mock("@features/support/api/fetchProjectCaseSearchResults", () => ({
  fetchProjectCaseSearchResults: vi.fn().mockResolvedValue([{ id: "c-1" }]),
}));
vi.mock("@features/support/utils/casesCsvExport", () => ({
  downloadCaseListCsv: downloadCaseListCsvMock,
  downloadCaseListPdf: vi.fn(),
}));

describe("CaseListCsvExportButton", () => {
  it("exports to csv from menu action", async () => {
    render(
      <ThemeProvider theme={createTheme()}>
        <CaseListCsvExportButton
          projectId="p1"
          caseSearchRequest={{ filters: {} } as never}
          filenamePrefix="cases"
        />
      </ThemeProvider>,
    );
    fireEvent.click(screen.getByRole("button", { name: /export/i }));
    fireEvent.click(await screen.findByRole("menuitem", { name: /export to csv/i }));
    await waitFor(() => {
      expect(downloadCaseListCsvMock).toHaveBeenCalled();
    });
  });
});
