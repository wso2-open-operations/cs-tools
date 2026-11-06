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
import { beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import { MemoryRouter } from "react-router";
import { QueryClient, QueryClientProvider, type UseQueryResult } from "@tanstack/react-query";
import type {
  BeProjectContact,
  BeProjectOnboardingMembership,
  BeProjectOnboardingStepsResponse,
} from "@api/backend/types";

const useSearchProjectContactsMock = vi.fn();
const useGetProjectOnboardingStepsMock = vi.fn();
const onboardingEnabledMock = vi.fn();
const postMock = vi.fn();

vi.mock("@features/csm-projects/api/useSearchProjectContacts", () => ({
  useSearchProjectContacts: () => useSearchProjectContactsMock(),
}));
// The onboarding column is flag-gated; the flag reader and the hook behind
// the column are both mocked so each case can pick flag state and ledger
// state independently of the contacts query.
vi.mock("@config/onboardingStatusConfig", () => ({
  isOnboardingStatusEnabled: () => onboardingEnabledMock(),
}));
vi.mock("@features/csm-projects/api/useGetProjectOnboardingSteps", () => ({
  useGetProjectOnboardingSteps: () => useGetProjectOnboardingStepsMock(),
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

function mockOnboardingResult(
  overrides: Partial<UseQueryResult<BeProjectOnboardingStepsResponse, Error>>,
): void {
  useGetProjectOnboardingStepsMock.mockReturnValue({
    data: undefined,
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

const INTEGRATION_CONTACT: BeProjectContact = {
  id: "30000000-0000-0000-0000-000000000000",
  name: "Integration User",
  email: "integration@example.com",
  registrationState: "invited",
  isCsIntegrationUser: true,
  customerContactPresent: true,
  grantsCaseAccess: false,
};

const JANE_EMAIL_FAILED: BeProjectOnboardingMembership = {
  membershipSfId: "a0X000000000001AAA",
  contactSfId: null,
  // Upper-cased on purpose relative to LINKED_CONTACT's address: matching
  // is by lower-cased email.
  email: "JANE.DOE@example.com",
  projectContactId: null,
  steps: [
    {
      step: "IDENTITY",
      status: "SUCCEEDED",
      attemptCount: 1,
      lastError: null,
      eventType: "CREATED",
      eventModifiedOn: "2026-09-01T10:00:00Z",
      updatedOn: "2026-09-01T10:00:01Z",
    },
    {
      step: "EMAIL",
      status: "FAILED",
      attemptCount: 2,
      lastError: "smtp: 550 mailbox unavailable",
      eventType: "CREATED",
      eventModifiedOn: "2026-09-01T10:00:00Z",
      updatedOn: "2026-09-02T10:00:01Z",
    },
  ],
};

describe("ProjectContactsTab", () => {
  beforeEach(() => {
    // Default every case to the flag being off with an idle onboarding query,
    // which is exactly the pre-flag behaviour the existing cases assert on.
    onboardingEnabledMock.mockReturnValue(false);
    mockOnboardingResult({});
  });

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

  it("shows Admin for a project ADMIN role or a global account admin role", () => {
    postMock.mockResolvedValue({ users: [] });
    mockQueryResult({
      data: [
        {
          ...LINKED_CONTACT,
          name: "Account Admin",
          roles: ["PORTAL_USER", "SECURITY_CONTACT"],
          accountRoles: ["customer", "customer_admin", "external"],
        },
        {
          ...LINKED_CONTACT,
          id: "00000000-0000-0000-0000-000000000001",
          name: "Project Admin",
          email: "project.admin@example.com",
          roles: ["ADMIN"],
        },
      ],
    });
    renderTab();
    expect(screen.queryByText("Customer Admin", { selector: ".MuiChip-label" })).not.toBeInTheDocument();
    expect(screen.getAllByText("Admin", { selector: ".MuiChip-label" })).toHaveLength(2);
    expect(screen.getAllByText("Portal User", { selector: ".MuiChip-label" })).toHaveLength(2);
    expect(screen.getByText("Security User", { selector: ".MuiChip-label" })).toBeInTheDocument();
  });

  it("renders a linked contact as a clickable profile link", () => {
    postMock.mockResolvedValue({ users: [] });
    mockQueryResult({ data: [LINKED_CONTACT] });
    renderTab();
    const link = screen.getByRole("link", { name: "Jane Doe" });
    expect(link).toHaveAttribute("href", `/people/${LINKED_CONTACT.id}`);
    expect(screen.getByText("registered")).toBeInTheDocument();
    expect(screen.getByText("Portal User", { selector: ".MuiChip-label" })).toBeInTheDocument();
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

  it("shows a linked but not yet registered row as Not registered, not as an address mismatch", () => {
    postMock.mockResolvedValue({ users: [] });
    mockQueryResult({ data: [{ ...INVITED_GRANTED_CONTACT, grantsCaseAccess: false }] });
    renderTab();
    expect(screen.getByText("Not registered", { selector: ".MuiChip-label" })).toBeInTheDocument();
    expect(screen.queryByText(/invited under a different address/i)).not.toBeInTheDocument();
  });

  it("shows an integration user as REGISTERED with no access fault", () => {
    postMock.mockResolvedValue({ users: [] });
    mockQueryResult({ data: [INTEGRATION_CONTACT] });
    renderTab();
    expect(screen.getByText("Integration user", { selector: ".MuiChip-label" })).toBeInTheDocument();
    expect(screen.getByText("REGISTERED", { selector: ".MuiChip-label" })).toBeInTheDocument();
    expect(screen.queryByText("No access", { selector: ".MuiChip-label" })).not.toBeInTheDocument();
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

  describe("Onboarding column (CSM_MIGRATION_ONBOARDING_STATUS_ENABLED)", () => {
    it("is not rendered at all while the flag is off", () => {
      postMock.mockResolvedValue({ users: [] });
      onboardingEnabledMock.mockReturnValue(false);
      mockQueryResult({ data: [LINKED_CONTACT] });
      renderTab();
      expect(screen.queryByText("Onboarding")).not.toBeInTheDocument();
      expect(screen.getAllByRole("columnheader")).toHaveLength(5);
    });

    it("shows the worst step status per contact, matched by lower-cased email", () => {
      postMock.mockResolvedValue({ users: [] });
      onboardingEnabledMock.mockReturnValue(true);
      mockQueryResult({ data: [LINKED_CONTACT, ORPHANED_CONTACT] });
      mockOnboardingResult({
        data: { memberships: [JANE_EMAIL_FAILED], total: 1, truncated: false },
      });
      renderTab();

      expect(screen.getByText("Onboarding")).toBeInTheDocument();
      expect(screen.getAllByRole("columnheader")).toHaveLength(6);
      const chip = screen.getByText("Failed: EMAIL", { selector: ".MuiChip-label" });
      expect(chip.closest(".MuiChip-root")).toHaveClass("MuiChip-colorError");
      // John has no membership recorded: his cell is a dash, not a chip.
      const johnRow = screen.getByText("John Smith").closest("tr");
      expect(johnRow).not.toBeNull();
      expect(johnRow).toHaveTextContent("—");
      expect(screen.queryByText("(partial)")).not.toBeInTheDocument();
    });

    it("renders a per-row skeleton while the ledger is loading, without blocking the contacts", () => {
      postMock.mockResolvedValue({ users: [] });
      onboardingEnabledMock.mockReturnValue(true);
      mockQueryResult({ data: [LINKED_CONTACT, ORPHANED_CONTACT] });
      mockOnboardingResult({ isLoading: true });
      const { container } = renderTab();
      expect(screen.getByRole("link", { name: "Jane Doe" })).toBeInTheDocument();
      expect(container.querySelectorAll(".MuiSkeleton-root")).toHaveLength(2);
    });

    it("degrades only the Onboarding cells to Unavailable when the ledger fails", () => {
      postMock.mockResolvedValue({ users: [] });
      onboardingEnabledMock.mockReturnValue(true);
      mockQueryResult({ data: [LINKED_CONTACT] });
      mockOnboardingResult({ isError: true, error: new Error("ledger down") });
      renderTab();
      expect(screen.getByRole("link", { name: "Jane Doe" })).toBeInTheDocument();
      expect(screen.getByText("Unavailable")).toBeInTheDocument();
      // The contacts table's own error state must not fire for an
      // onboarding-only failure.
      expect(screen.queryByText("ledger down")).not.toBeInTheDocument();
    });

    it("marks the column header (partial) when the backend truncated the ledger", () => {
      postMock.mockResolvedValue({ users: [] });
      onboardingEnabledMock.mockReturnValue(true);
      mockQueryResult({ data: [LINKED_CONTACT] });
      mockOnboardingResult({
        data: { memberships: [JANE_EMAIL_FAILED], total: 1, truncated: true },
      });
      renderTab();
      expect(screen.getByText("(partial)")).toBeInTheDocument();
    });
  });
});
