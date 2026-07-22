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

import { render, screen, within } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import type { UseQueryResult } from "@tanstack/react-query";
import type { BeChangeRequestDetail } from "@api/backend/types";

const navigateMock = vi.fn();
const useGetChangeRequestMock = vi.fn();

vi.mock("react-router", () => ({
  useParams: () => ({ id: "chg-1" }),
}));
vi.mock("@hooks/useNavTransition", () => ({
  useNavTransition: () => navigateMock,
}));
vi.mock("@features/csm-operations/api/useGetChangeRequest", () => ({
  useGetChangeRequest: () => useGetChangeRequestMock(),
}));
vi.mock("@features/csm-operations/api/usePatchChangeRequest", () => ({
  usePatchChangeRequest: () => ({ isPending: false, mutate: vi.fn() }),
}));
vi.mock("@context/error-banner/ErrorBannerContext", () => ({
  useErrorBanner: () => ({ showError: vi.fn() }),
}));
vi.mock("@features/csm-operations/components/ChangeRequestApprovals", () => ({
  default: () => null,
}));

// Imported after the mocks above so the module picks them up.
import CsmChangeRequestDetailPage from "@features/csm-operations/pages/CsmChangeRequestDetailPage";

const BASE_CR: BeChangeRequestDetail = {
  id: "chg-1",
  number: "CHG0009988",
  subject: "Upgrade the gateway cluster",
  case: { id: "case-1", name: "CASE0001234" },
  createdOn: "2026-01-01T00:00:00Z",
};

function mockQueryResult(
  overrides: Partial<UseQueryResult<BeChangeRequestDetail | null, Error>>,
): void {
  useGetChangeRequestMock.mockReturnValue({
    data: null,
    isLoading: false,
    isError: false,
    error: null,
    ...overrides,
  });
}

describe("CsmChangeRequestDetailPage", () => {
  it("renders the linked case as a clickable reference to the case route", () => {
    mockQueryResult({ data: BASE_CR });
    render(<CsmChangeRequestDetailPage />);

    screen
      .getByText("CASE0001234")
      .closest('[role="button"]')
      ?.dispatchEvent(new MouseEvent("click", { bubbles: true }));

    expect(navigateMock).toHaveBeenCalledWith("/cases/case-1");
  });

  it("renders a dash for the linked case when there is no case reference", () => {
    mockQueryResult({ data: { ...BASE_CR, case: null } });
    render(<CsmChangeRequestDetailPage />);
    const linkedCaseCell = screen.getByText("Linked case").parentElement!;
    expect(within(linkedCaseCell).getByText("—")).toBeInTheDocument();
  });
});
