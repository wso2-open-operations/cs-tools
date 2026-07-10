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

import { fireEvent, render, screen, within } from "@testing-library/react";
import { describe, expect, it, vi, beforeEach } from "vitest";
import "@testing-library/jest-dom/vitest";
import CsmAnnouncementsPage from "@features/csm-announcements/pages/CsmAnnouncementsPage";
import { useSearchAnnouncements } from "@features/csm-announcements/api/useSearchAnnouncements";
import type { CsmAnnouncementRow } from "@features/csm-announcements/types/csmAnnouncements";

// The backend client reads runtime config (`CSM_PORTAL_BACKEND_BASE_URL`) at
// module load, which isn't present under vitest. QueryErrorState imports
// `BackendApiError` from it, so stub the module (same approach as
// useCsmCaseActivities.test.ts).
vi.mock("@api/backend/client", () => ({
  BackendApiError: class BackendApiError extends Error {},
  useBackendApi: () => ({ post: vi.fn() }),
}));

vi.mock("@features/csm-announcements/api/useSearchAnnouncements", () => ({
  useSearchAnnouncements: vi.fn(),
}));

// The real project picker fetches from the backend; stub it so the page test
// stays focused on the list + its own state/severity filters.
vi.mock("@features/csm-cases/components/AsyncProjectMultiSelect", () => ({
  default: () => <div data-testid="project-filter" />,
}));

const mockedUseSearch = vi.mocked(useSearchAnnouncements);

const ROW: CsmAnnouncementRow = {
  id: "a-1",
  number: "ANN-1001",
  subject: "Scheduled maintenance on Choreo",
  projectName: "IAM Production",
  state: "open",
  createdBy: "jane@example.com",
  createdAt: "2026-07-01T10:00:00Z",
  updatedAt: "2026-07-02T10:00:00Z",
};

function mockResult(
  overrides: Partial<ReturnType<typeof useSearchAnnouncements>>,
): void {
  mockedUseSearch.mockReturnValue({
    data: undefined,
    isLoading: false,
    isFetching: false,
    isError: false,
    error: null,
    ...overrides,
  } as unknown as ReturnType<typeof useSearchAnnouncements>);
}

beforeEach(() => {
  mockedUseSearch.mockReset();
});

describe("CsmAnnouncementsPage — list states", () => {
  it("renders a row from the search result", () => {
    mockResult({
      data: { announcements: [ROW], total: 1, limit: 20, offset: 0, hasMore: false },
    });
    render(<CsmAnnouncementsPage />);
    expect(screen.getByText("Scheduled maintenance on Choreo")).toBeInTheDocument();
    expect(screen.getByText("ANN-1001")).toBeInTheDocument();
    expect(screen.getByText("IAM Production")).toBeInTheDocument();
  });

  it("shows the empty state when there are no announcements", () => {
    mockResult({
      data: { announcements: [], total: 0, limit: 20, offset: 0, hasMore: false },
    });
    render(<CsmAnnouncementsPage />);
    expect(screen.getByText(/no announcements found/i)).toBeInTheDocument();
  });

  it("surfaces the error message on failure", () => {
    mockResult({ isError: true, error: new Error("boom") });
    render(<CsmAnnouncementsPage />);
    expect(screen.getByText(/failed to load announcements: boom/i)).toBeInTheDocument();
  });
});

describe("CsmAnnouncementsPage — filters default to show-all", () => {
  it("calls the search with no state/severity/project filters on first render", () => {
    mockResult({
      data: { announcements: [ROW], total: 1, limit: 20, offset: 0, hasMore: false },
    });
    render(<CsmAnnouncementsPage />);
    const [filters] = mockedUseSearch.mock.calls[0];
    expect(filters).toEqual(
      expect.objectContaining({ states: [], severities: [], projectIds: [] }),
    );
  });

  it("pushes a picked state into the search filters", () => {
    mockResult({
      data: { announcements: [ROW], total: 1, limit: 20, offset: 0, hasMore: false },
    });
    render(<CsmAnnouncementsPage />);

    fireEvent.mouseDown(screen.getByRole("combobox", { name: /state/i }));
    const listbox = screen.getByRole("listbox");
    fireEvent.click(within(listbox).getByRole("option", { name: /closed/i }));

    // The most recent render's filters carry the selected state.
    const lastCall = mockedUseSearch.mock.calls.at(-1)!;
    expect(lastCall[0].states).toContain("closed");
  });

  it("pushes a picked severity into the search filters", () => {
    mockResult({
      data: { announcements: [ROW], total: 1, limit: 20, offset: 0, hasMore: false },
    });
    render(<CsmAnnouncementsPage />);

    fireEvent.mouseDown(screen.getByRole("combobox", { name: /severity/i }));
    const listbox = screen.getByRole("listbox");
    fireEvent.click(within(listbox).getByRole("option", { name: /catastrophic/i }));

    const lastCall = mockedUseSearch.mock.calls.at(-1)!;
    expect(lastCall[0].severities).toContain("S0");
  });
});
