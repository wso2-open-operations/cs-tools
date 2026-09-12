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
import { MemoryRouter } from "react-router";
import { QueryClient, QueryClientProvider, type UseQueryResult } from "@tanstack/react-query";
import type { BeProjectContact } from "@api/backend/types";

const useSearchProjectContactsMock = vi.fn();
const postMock = vi.fn();

vi.mock("@features/csm-projects/api/useSearchProjectContacts", () => ({
  useSearchProjectContacts: () => useSearchProjectContactsMock(),
}));
// The backend client reads runtime config (`CSM_PORTAL_BACKEND_BASE_URL`) at
// module load, which isn't present under vitest. `QueryErrorState` imports
// `BackendApiError` from it directly, so stub the module with a real class
// (so `instanceof` still works) — same approach as
// CsmChangeRequestDetailPage.test.tsx. UserRefLink resolves an unknown id
// through `POST /users/search` via `useBackendApi`.
vi.mock("@api/backend/client", () => ({
  BackendApiError: class BackendApiError extends Error {
    status: number;
    constructor(status: number, message: string) {
      super(message);
      this.status = status;
    }
  },
  useBackendApi: () => ({ post: postMock }),
}));

// Imported after the mocks above so the module picks them up.
import ProjectContactsTab from "@features/csm-projects/components/ProjectContactsTab";

function mockQueryResult(
  overrides: Partial<UseQueryResult<BeProjectContact[], Error>>,
): void {
  useSearchProjectContactsMock.mockReturnValue({
    data: [],
    isLoading: false,
    isError: false,
    error: null,
    ...overrides,
  });
}

function renderTab(): ReturnType<typeof render> {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter>
        <ProjectContactsTab projectId="proj-1" />
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

const LINKED_CONTACT: BeProjectContact = {
  id: "00000000-0000-0000-0000-000000000000",
  name: "Jane Doe",
  email: "jane.doe@example.com",
  registrationState: "registered",
  notificationsEnabled: true,
  roles: ["viewer"],
};

const ORPHANED_CONTACT: BeProjectContact = {
  // No `id` — this row has no linked contact record.
  name: "John Smith",
  email: "john.smith@example.com",
  registrationState: "pending",
};

// The more common real-world shape: no `id`, no `name`, *and* no `email` —
// nothing to key the row on at all.
const FULLY_BLANK_ORPHANED_CONTACT: BeProjectContact = {
  registrationState: "pending",
};

const GRANTED_CONTACT: BeProjectContact = {
  id: "10000000-0000-0000-0000-000000000000",
  name: "Grant Access",
  email: "grant.access@example.com",
  registrationState: "registered",
  customerContactPresent: true,
  grantsCaseAccess: true,
};

// Still linked and still granted access, just not yet registered — used to
// check the registration chip's "invited" colour independently of access
// status, which no longer varies with registration state at all.
const INVITED_GRANTED_CONTACT: BeProjectContact = {
  id: "20000000-0000-0000-0000-000000000000",
  name: "Invited Not Yet Registered",
  email: "invited@example.com",
  registrationState: "invited",
  customerContactPresent: true,
  grantsCaseAccess: true,
};

describe("ProjectContactsTab", () => {
  it("renders a loading skeleton while the query is pending", () => {
    mockQueryResult({ isLoading: true });
    const { container } = renderTab();
    expect(container.querySelectorAll(".MuiSkeleton-root").length).toBeGreaterThan(0);
  });

  it("renders an error state when the query fails", () => {
    mockQueryResult({ isError: true, error: new Error("boom") });
    renderTab();
    expect(screen.getByText("boom")).toBeInTheDocument();
  });

  it("renders an empty state rather than an empty table when the project has no contacts", () => {
    mockQueryResult({ data: [] });
    renderTab();
    expect(screen.getByText(/No contacts found for this project/i)).toBeInTheDocument();
  });

  it("renders a linked contact as a clickable profile link", () => {
    postMock.mockResolvedValue({ users: [] });
    mockQueryResult({ data: [LINKED_CONTACT] });
    renderTab();
    const link = screen.getByRole("link", { name: "Jane Doe" });
    expect(link).toHaveAttribute("href", `/people/${LINKED_CONTACT.id}`);
    expect(screen.getByText("registered")).toBeInTheDocument();
    expect(screen.getByText("viewer", { selector: ".MuiChip-label" })).toBeInTheDocument();
  });

  it("renders an orphaned contact (no id) without a link, flagged, and without crashing", () => {
    postMock.mockResolvedValue({ users: [] });
    mockQueryResult({ data: [ORPHANED_CONTACT] });
    renderTab();
    expect(screen.getByText("John Smith")).toBeInTheDocument();
    expect(screen.queryByRole("link", { name: "John Smith" })).not.toBeInTheDocument();
    expect(screen.getByText("Orphaned", { selector: ".MuiChip-label" })).toBeInTheDocument();
  });

  it("renders a fully-blank orphaned row (no id, name, or email) with a clearly-marked, always-visible reason rather than a blank row", () => {
    postMock.mockResolvedValue({ users: [] });
    mockQueryResult({ data: [FULLY_BLANK_ORPHANED_CONTACT] });
    renderTab();

    // Never a bare blank cell — the row states plainly that it has no
    // linked contact record...
    expect(screen.getByText("No linked contact record")).toBeInTheDocument();
    // ...and the operationally important consequence is visible inline
    // (not hidden behind a hover-only tooltip).
    expect(
      screen.getByText(/can't see this project's cases/i),
    ).toBeInTheDocument();
    expect(screen.getByText("Orphaned", { selector: ".MuiChip-label" })).toBeInTheDocument();
    expect(
      screen.queryByRole("link", { name: "No linked contact record" }),
    ).not.toBeInTheDocument();
  });

  it("renders both a linked and an orphaned row together without crashing", () => {
    postMock.mockResolvedValue({ users: [] });
    mockQueryResult({ data: [LINKED_CONTACT, ORPHANED_CONTACT] });
    renderTab();
    expect(screen.getByRole("link", { name: "Jane Doe" })).toBeInTheDocument();
    expect(screen.getByText("John Smith")).toBeInTheDocument();
    expect(screen.getByText("Orphaned", { selector: ".MuiChip-label" })).toBeInTheDocument();
  });

  it("shows a granted contact (customerContactPresent + grantsCaseAccess) as Has access, no reason line", () => {
    postMock.mockResolvedValue({ users: [] });
    mockQueryResult({ data: [GRANTED_CONTACT] });
    renderTab();
    expect(screen.getByText("Has access", { selector: ".MuiChip-label" })).toBeInTheDocument();
    expect(screen.queryByText(/can't see this project's cases/i)).not.toBeInTheDocument();
  });

  it("flags a row with grantsCaseAccess false as No access with the no-linked-record reason", () => {
    postMock.mockResolvedValue({ users: [] });
    mockQueryResult({
      data: [{ ...ORPHANED_CONTACT, customerContactPresent: false, grantsCaseAccess: false }],
    });
    renderTab();
    expect(screen.getByText("No access", { selector: ".MuiChip-label" })).toBeInTheDocument();
    expect(
      screen.getByText(/no linked contact record.*can't see this project's cases/i),
    ).toBeInTheDocument();
  });

  // The two "No access" causes are operationally different — one is a failed
  // onboarding to chase, the other a row naming the wrong address — so the
  // reason line has to say which, not just that access is missing.
  it("gives a linked row whose invited address differs from its contact's own the mismatch reason, not the no-linked-record one", () => {
    postMock.mockResolvedValue({ users: [] });
    mockQueryResult({
      data: [{ ...GRANTED_CONTACT, customerContactPresent: true, grantsCaseAccess: false }],
    });
    renderTab();
    expect(screen.getByText("No access", { selector: ".MuiChip-label" })).toBeInTheDocument();
    expect(
      screen.getByText(/invited under a different address than its linked contact's own/i),
    ).toBeInTheDocument();
    expect(screen.queryByText(/no linked contact record/i)).not.toBeInTheDocument();
  });

  // A failed invite has no name (that is only ever known from the contact
  // record) but does carry the address it was invited under, so the row must
  // still be identifiable rather than rendering blank.
  it("renders a failed-invite row by the address it was invited under, with the invite-never-completed reason", () => {
    postMock.mockResolvedValue({ users: [] });
    mockQueryResult({
      data: [
        {
          email: "never.onboarded@example.com",
          registrationState: "INVITED",
          customerContactPresent: false,
          grantsCaseAccess: false,
        },
      ],
    });
    renderTab();
    expect(screen.getAllByText("never.onboarded@example.com").length).toBeGreaterThan(0);
    expect(screen.getByText("No access", { selector: ".MuiChip-label" })).toBeInTheDocument();
    expect(screen.getByText(/the invite never completed/i)).toBeInTheDocument();
    expect(
      screen.queryByText(/invited under a different address/i),
    ).not.toBeInTheDocument();
  });

  it("colours the registration chip by state (registered=success-ish, invited=warning-ish) without changing the visible text", () => {
    postMock.mockResolvedValue({ users: [] });
    mockQueryResult({ data: [GRANTED_CONTACT, INVITED_GRANTED_CONTACT] });
    renderTab();
    const registeredChip = screen.getByText("registered", { selector: ".MuiChip-label" });
    const invitedChip = screen.getByText("invited", { selector: ".MuiChip-label" });
    expect(registeredChip.closest(".MuiChip-root")).toHaveClass("MuiChip-colorSuccess");
    expect(invitedChip.closest(".MuiChip-root")).toHaveClass("MuiChip-colorWarning");
  });
});
