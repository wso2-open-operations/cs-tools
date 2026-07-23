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
import "@testing-library/jest-dom/vitest";
import type { UseQueryResult } from "@tanstack/react-query";
import type { BeIncidentDetail } from "@api/backend/types";

const navigateMock = vi.fn();
const useGetIncidentMock = vi.fn();

// The backend client reads runtime config (`CSM_PORTAL_BACKEND_BASE_URL`) at
// module load, which isn't present under vitest. The page imports
// `BackendApiError` from it directly, so stub the module (same approach as
// CsmAnnouncementsPage.test.tsx).
vi.mock("@api/backend/client", () => ({
  BackendApiError: class BackendApiError extends Error {},
  useBackendApi: () => ({ get: vi.fn(), patch: vi.fn() }),
}));

vi.mock("react-router", () => ({
  useParams: () => ({ id: "inc-1" }),
}));
vi.mock("@hooks/useNavTransition", () => ({
  useNavTransition: () => navigateMock,
}));
vi.mock("@features/csm-operations/api/useGetIncident", () => ({
  useGetIncident: () => useGetIncidentMock(),
}));
vi.mock("@features/csm-operations/api/usePatchIncident", () => ({
  usePatchIncident: () => ({ isPending: false, mutate: vi.fn() }),
}));
vi.mock("@context/error-banner/ErrorBannerContext", () => ({
  useErrorBanner: () => ({ showError: vi.fn() }),
}));

// Imported after the mocks above so the module picks them up.
import CsmIncidentDetailPage from "@features/csm-operations/pages/CsmIncidentDetailPage";

const BASE_INCIDENT: BeIncidentDetail = {
  id: "inc-1",
  number: "INC0012345",
  openedOn: "2026-01-01T00:00:00Z",
  subject: "Gateway 502s",
  priority: null,
  state: "IN_PROGRESS" as BeIncidentDetail["state"],
  category: null,
  parent: { id: "inc-parent", name: "INC0011111" },
  changeRequest: { id: "chg-1", name: "CHG0009988" },
  problem: { id: "prb-1", name: "PRB0040157" },
  causedBy: { id: "obscure-1", name: "Some obscure record" },
};

function mockQueryResult(
  overrides: Partial<UseQueryResult<BeIncidentDetail | null, Error>>,
): void {
  useGetIncidentMock.mockReturnValue({
    data: null,
    isLoading: false,
    isError: false,
    error: null,
    ...overrides,
  });
}

function clickChip(text: string): void {
  screen
    .getByText(text)
    .closest('[role="button"]')
    ?.dispatchEvent(new MouseEvent("click", { bubbles: true }));
}

describe("CsmIncidentDetailPage", () => {
  it("renders parent incident, change request, and problem as clickable references", () => {
    mockQueryResult({ data: BASE_INCIDENT });
    render(<CsmIncidentDetailPage />);

    clickChip("INC0011111");
    expect(navigateMock).toHaveBeenCalledWith("/operations/incidents/inc-parent");

    clickChip("CHG0009988");
    expect(navigateMock).toHaveBeenCalledWith("/operations/change-requests/chg-1");

    clickChip("PRB0040157");
    expect(navigateMock).toHaveBeenCalledWith("/operations/problems/prb-1");
  });

  it("renders 'Caused by' as plain, non-navigable text since its target type is unconfirmed", () => {
    mockQueryResult({ data: BASE_INCIDENT });
    render(<CsmIncidentDetailPage />);

    const causedByText = screen.getByText("Some obscure record");
    expect(causedByText.closest('[role="button"]')).toBeNull();
  });
});
