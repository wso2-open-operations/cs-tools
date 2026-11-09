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
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import type { ReactElement } from "react";
import { MemoryRouter } from "react-router";

const postMock = vi.fn();
vi.mock("@api/backend/client", () => ({
  useBackendApi: () => ({ post: postMock }),
}));

vi.mock("@context/current-user/CurrentUserContext", () => ({
  useCurrentUser: () => ({ user: { id: "user-1", email: "jane.doe@example.com" }, isLoading: false, isError: false }),
}));

import CasePreviewContent from "@features/csm-cases/components/CasePreviewContent";
import type { CsmCaseRow } from "@features/csm-cases/types/csmCases";

const ROW: CsmCaseRow = {
  id: "case-1",
  caseNumber: "CS-1007",
  wso2CaseId: "ACMESUB-42",
  subject: "Cluster fails to start",
  customer: "Acme Corp",
  accountId: "acct-1",
  projectId: "proj-1",
  projectName: "Acme Project",
  product: "WSO2 Identity Server",
  severity: "S2",
  state: "work_in_progress",
  assignee: "Jane Doe",
  assigneeIsMe: false,
  slaClockType: "first_response",
  minutesToBreach: 120,
  createdAt: "2026-01-01T00:00:00Z",
  updatedAt: "2026-01-02T00:00:00Z",
};

function renderWithProviders(ui: ReactElement): ReturnType<typeof render> {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter>{ui}</MemoryRouter>
    </QueryClientProvider>,
  );
}

describe("CasePreviewContent", () => {
  it("renders the case subject, number, and state/severity chips", () => {
    postMock.mockResolvedValue({ comments: [] });
    renderWithProviders(<CasePreviewContent row={ROW} onClose={vi.fn()} />);

    expect(screen.getByText("Cluster fails to start")).toBeInTheDocument();
    expect(screen.getByText(/ACMESUB-42/)).toBeInTheDocument();
    expect(screen.getByText(/CS-1007/)).toBeInTheDocument();
  });

  it("shows recent comments once loaded", async () => {
    postMock.mockResolvedValue({
      comments: [
        {
          id: "c1",
          content: "Investigating now.",
          type: "comment",
          createdOn: "2026-01-02T00:00:00Z",
          createdBy: { id: "u1", name: "Jane Doe", email: "jane@example.com" },
        },
      ],
    });
    renderWithProviders(<CasePreviewContent row={ROW} onClose={vi.fn()} />);

    await waitFor(() => expect(screen.getByText("Investigating now.")).toBeInTheDocument());
  });

  it("shows 'No comments yet.' when the case has none", async () => {
    postMock.mockResolvedValue({ comments: [] });
    renderWithProviders(<CasePreviewContent row={ROW} onClose={vi.fn()} />);

    await waitFor(() => expect(screen.getByText("No comments yet.")).toBeInTheDocument());
  });

  it("calls onClose when the close (✕) button is clicked", () => {
    postMock.mockResolvedValue({ comments: [] });
    const onClose = vi.fn();
    renderWithProviders(<CasePreviewContent row={ROW} onClose={onClose} />);

    fireEvent.click(screen.getByRole("button", { name: "Close preview" }));
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it("hides the close button when hideCloseButton is set, without affecting 'View full details'", () => {
    postMock.mockResolvedValue({ comments: [] });
    renderWithProviders(<CasePreviewContent row={ROW} onClose={vi.fn()} hideCloseButton />);

    expect(screen.queryByRole("button", { name: "Close preview" })).not.toBeInTheDocument();
    expect(screen.getByRole("link", { name: "View full details" })).toBeInTheDocument();
  });
});
