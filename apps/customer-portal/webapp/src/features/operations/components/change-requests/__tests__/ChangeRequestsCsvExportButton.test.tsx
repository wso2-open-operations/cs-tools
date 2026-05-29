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

import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import ChangeRequestsCsvExportButton from "@features/operations/components/change-requests/ChangeRequestsCsvExportButton";
import { SortOrder } from "@/types/common";
import { ChangeRequestSortField } from "@features/operations/types/changeRequests";

vi.mock("@context/error-banner/ErrorBannerContext", () => ({
  useErrorBanner: () => ({ showError: vi.fn() }),
}));

describe("ChangeRequestsCsvExportButton", () => {
  it("renders download results button", () => {
    render(
      <ChangeRequestsCsvExportButton
        projectId="proj-1"
        searchRequest={{
          sortBy: { field: ChangeRequestSortField.CreatedOn, order: SortOrder.DESC },
        }}
        prefetchedItems={[]}
      />,
    );
    expect(screen.getByRole("button", { name: /^export$/i })).toBeInTheDocument();
  });
});
