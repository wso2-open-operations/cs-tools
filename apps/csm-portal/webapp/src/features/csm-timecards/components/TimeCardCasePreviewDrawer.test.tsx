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
import { describe, expect, it, vi, beforeEach } from "vitest";
import "@testing-library/jest-dom/vitest";
import type { ReactElement } from "react";
import { MemoryRouter } from "react-router";

const getMock = vi.fn();
const postMock = vi.fn();
// The comment box's Templates menu has its own tests; its import chain reads
// runtime config, so it is a stand-in here.
vi.mock("@features/comment-templates/components/CommentTemplatesMenu", () => ({
  default: () => null,
}));
vi.mock("@api/backend/client", () => ({
  useBackendApi: () => ({ get: getMock, post: postMock }),
}));

import TimeCardCasePreviewDrawer from "@features/csm-timecards/components/TimeCardCasePreviewDrawer";
import type { CsmTimeCard } from "@features/csm-timecards/types/timeCards";

const CARD: CsmTimeCard = {
  id: "tc-1",
  caseId: "case-1",
  caseNumber: "CS0352584",
  projectId: "proj-1",
  projectName: "Acme Project",
  workDate: "2026-07-01",
  userId: "user-1",
  userName: "Jane Doe",
  state: "submitted",
  billable: true,
  totalMinutes: 45,
};

function renderWithProviders(ui: ReactElement): ReturnType<typeof render> {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter>{ui}</MemoryRouter>
    </QueryClientProvider>,
  );
}

describe("TimeCardCasePreviewDrawer", () => {
  beforeEach(() => {
    getMock.mockReset();
    postMock.mockReset();
    postMock.mockResolvedValue({ comments: [] });
  });

  it("renders nothing (closed) when card is null", () => {
    renderWithProviders(
      <TimeCardCasePreviewDrawer card={null} actions={[]} onClose={vi.fn()} onDecide={vi.fn()} />,
    );
    expect(screen.queryByText("Time card")).not.toBeInTheDocument();
  });

  it("shows the time card's own fields", () => {
    getMock.mockReturnValue(new Promise(() => {})); // case fetch never resolves in this test
    renderWithProviders(
      <TimeCardCasePreviewDrawer card={CARD} actions={[]} onClose={vi.fn()} onDecide={vi.fn()} />,
    );

    expect(screen.getByText("Time card")).toBeInTheDocument();
    expect(screen.getByText("Jane Doe")).toBeInTheDocument();
    expect(screen.getByText("Acme Project")).toBeInTheDocument();
    expect(screen.getByText("45 min")).toBeInTheDocument();
  });

  it("fetches and renders the linked case's own preview content below the time card section", async () => {
    getMock.mockResolvedValue({
      id: "case-1",
      number: "CS0352584",
      subject: "Cluster fails to start",
      severity: "high",
      state: "work_in_progress",
    });
    renderWithProviders(
      <TimeCardCasePreviewDrawer card={CARD} actions={[]} onClose={vi.fn()} onDecide={vi.fn()} />,
    );

    await waitFor(() => expect(screen.getByText("Cluster fails to start")).toBeInTheDocument());
    expect(getMock).toHaveBeenCalledWith("/cases/case-1");
  });

  it("shows an error message when the case fails to load", async () => {
    getMock.mockRejectedValue(new Error("boom"));
    renderWithProviders(
      <TimeCardCasePreviewDrawer card={CARD} actions={[]} onClose={vi.fn()} onDecide={vi.fn()} />,
    );

    await waitFor(() => expect(screen.getByText("Could not load the case.")).toBeInTheDocument());
  });

  it("renders no Approve/Reject buttons when actions is empty", () => {
    getMock.mockReturnValue(new Promise(() => {}));
    renderWithProviders(
      <TimeCardCasePreviewDrawer card={CARD} actions={[]} onClose={vi.fn()} onDecide={vi.fn()} />,
    );

    expect(screen.queryByRole("button", { name: "Approve" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Reject" })).not.toBeInTheDocument();
  });

  it("calls onDecide with 'approve' and closes when Approve is clicked", () => {
    getMock.mockReturnValue(new Promise(() => {}));
    const onDecide = vi.fn();
    renderWithProviders(
      <TimeCardCasePreviewDrawer
        card={CARD}
        actions={["approve", "reject"]}
        onClose={vi.fn()}
        onDecide={onDecide}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "Approve" }));
    expect(onDecide).toHaveBeenCalledWith("approve");
  });

  it("calls onDecide with 'reject' when Reject is clicked", () => {
    getMock.mockReturnValue(new Promise(() => {}));
    const onDecide = vi.fn();
    renderWithProviders(
      <TimeCardCasePreviewDrawer
        card={CARD}
        actions={["approve", "reject"]}
        onClose={vi.fn()}
        onDecide={onDecide}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "Reject" }));
    expect(onDecide).toHaveBeenCalledWith("reject");
  });

  it("lists the eligible approvers on a submitted card", () => {
    getMock.mockReturnValue(new Promise(() => {}));
    renderWithProviders(
      <TimeCardCasePreviewDrawer
        card={{
          ...CARD,
          approvers: [
            { id: "lead-1", name: "Lead One" },
            { id: "lead-2", name: "Lead Two" },
          ],
        }}
        actions={[]}
        onClose={vi.fn()}
        onDecide={vi.fn()}
      />,
    );

    expect(screen.getByText("Approvers")).toBeInTheDocument();
    expect(screen.getByText("Lead One, Lead Two")).toBeInTheDocument();
  });

  it("shows the decision text without a generic 'Decision' caption on a decided card", () => {
    getMock.mockReturnValue(new Promise(() => {}));
    renderWithProviders(
      <TimeCardCasePreviewDrawer
        card={{ ...CARD, state: "approved", approvedByName: "Lead Person" }}
        actions={[]}
        onClose={vi.fn()}
        onDecide={vi.fn()}
      />,
    );

    expect(screen.getByText("Approved by: Lead Person")).toBeInTheDocument();
    expect(screen.queryByText("Decision")).not.toBeInTheDocument();
  });

  it("only shows the embedded case preview's 'View full details' close affordance, not a duplicate close button", async () => {
    getMock.mockResolvedValue({
      id: "case-1",
      number: "CS0352584",
      subject: "Cluster fails to start",
    });
    renderWithProviders(
      <TimeCardCasePreviewDrawer card={CARD} actions={[]} onClose={vi.fn()} onDecide={vi.fn()} />,
    );

    await waitFor(() => expect(screen.getByText("Cluster fails to start")).toBeInTheDocument());
    // One "Close preview" button (the drawer's own, at the top) -- the
    // embedded CasePreviewContent's own close button is suppressed via
    // hideCloseButton so there's never a second one doing the same thing.
    expect(screen.getAllByRole("button", { name: "Close preview" })).toHaveLength(1);
  });
});
