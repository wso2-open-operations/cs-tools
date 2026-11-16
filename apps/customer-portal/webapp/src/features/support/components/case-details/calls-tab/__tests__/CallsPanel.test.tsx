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

import type { ReactElement } from "react";
import { act, render, screen, fireEvent, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { format } from "date-fns";
import { useGetCallRequests } from "@features/support/api/useGetCallRequests";
import useGetUserDetails from "@features/settings/api/useGetUserDetails";
import {
  CALL_REQUEST_STATE_CANCELLED,
  CALL_REQUEST_STATE_CONCLUDED,
  CALL_REQUEST_STATE_CUSTOMER_REJECTED,
  CALL_REQUEST_STATE_PENDING_ON_WSO2,
} from "@features/support/constants/supportConstants";
import CallsPanel from "@case-details-calls/CallsPanel";

vi.mock("@features/support/api/useGetCallRequests");
vi.mock("@features/support/api/usePostCallRequest", () => ({
  usePostCallRequest: () => ({
    mutate: vi.fn(),
    isPending: false,
  }),
}));

const mockPatchMutate = vi.fn();
vi.mock("@features/support/api/usePatchCallRequest", () => ({
  usePatchCallRequest: () => ({
    mutate: mockPatchMutate,
    isPending: false,
  }),
}));

vi.mock("@features/settings/api/useGetUserDetails");

vi.mock("@features/settings/api/usePatchUserMe", () => ({
  usePatchUserMe: () => ({
    mutate: vi.fn(),
    isPending: false,
  }),
}));

// entity-service's real GET /projects/{id}/filters returns callRequestStates
// with its own string enum ids ("pending_on_wso2", ...), never the
// ServiceNow-era numeric stateKey as a string -- a prior version of this mock
// used numeric-looking ids ("1", "2", "4", ...), which let
// CallsPanel.tsx's old `Number(found.id)` bug pass every test here while
// silently sending `stateKey: NaN` against the real backend (a real,
// reported 400 "Invalid request payload." on every customer Approve/Reject).
// Keep this mock shaped like the real response so a regression here is
// caught again. A plain vi.fn() (not a fixed object) so individual tests can
// override severityBasedAllocationTime via mockReturnValueOnce.
const mockUseGetProjectFilters = vi.fn((): {
  data: {
    callRequestStates: { id: string; label: string }[];
    severityBasedAllocationTime?: Record<string, number>;
  };
  isLoading: boolean;
  isError: boolean;
} => ({
  data: {
    callRequestStates: [
      { id: "pending_on_customer", label: "Pending on Customer" },
      { id: "pending_on_wso2", label: "Pending on WSO2" },
      { id: "scheduled", label: "Scheduled" },
      { id: "customer_rejected", label: "Customer Rejected" },
      { id: "notes_pending", label: "Notes Pending" },
    ],
  },
  isLoading: false,
  isError: false,
}));
vi.mock("@api/useGetProjectFilters", () => ({
  default: () => mockUseGetProjectFilters(),
}));

vi.mock("@components/header/UserProfileModal", () => ({
  default: ({ open }: { open: boolean }) =>
    open ? <div data-testid="user-profile-modal" /> : null,
}));

function createTestQueryClient() {
  return new QueryClient({ defaultOptions: { queries: { retry: false } } });
}

function renderWithProviders(ui: ReactElement) {
  const queryClient = createTestQueryClient();
  return render(
    <QueryClientProvider client={queryClient}>{ui}</QueryClientProvider>,
  );
}

const mockProjectId = "project-1";
const mockCaseId = "case-1";

describe("CallsPanel", () => {
  beforeEach(() => {
    mockPatchMutate.mockClear();
    mockUseGetProjectFilters.mockClear();
    vi.mocked(useGetCallRequests).mockClear();
    vi.mocked(useGetUserDetails).mockReturnValue({
      data: { timeZone: "America/New_York" },
      refetch: vi
        .fn()
        .mockResolvedValue({ data: { timeZone: "America/New_York" } }),
      isLoading: false,
      isError: false,
    } as unknown as ReturnType<typeof useGetUserDetails>);
  });

  it("should render loading state", () => {
    vi.mocked(useGetCallRequests).mockReturnValue({
      isPending: true,
      data: undefined,
      isError: false,
      refetch: vi.fn(),
      fetchNextPage: vi.fn(),
      hasNextPage: false,
      isFetchingNextPage: false,
      isFetchNextPageError: false,
    } as unknown as ReturnType<typeof useGetCallRequests>);

    renderWithProviders(<CallsPanel
        projectId={mockProjectId}
        caseId={mockCaseId}
        caseStatusLabel="Work In Progress"
      />);
    expect(screen.getByTestId("calls-list-skeleton")).toBeInTheDocument();
  });

  // The case's Calls count includes Canceled call requests, so the list must
  // show them too: the search is sent without a state filter, which returns
  // every state.
  it("does not filter the call request search by state, so Canceled calls are listed", () => {
    mockUseGetProjectFilters.mockReturnValueOnce({
      data: {
        callRequestStates: [
          { id: "pending_on_customer", label: "Pending on Customer" },
          { id: "pending_on_wso2", label: "Pending on WSO2" },
          { id: "canceled", label: "Canceled" },
        ],
      },
      isLoading: false,
      isError: false,
    });
    vi.mocked(useGetCallRequests).mockReturnValue({
      isPending: true,
      data: undefined,
      isError: false,
      refetch: vi.fn(),
      fetchNextPage: vi.fn(),
      hasNextPage: false,
      isFetchingNextPage: false,
      isFetchNextPageError: false,
    } as unknown as ReturnType<typeof useGetCallRequests>);

    renderWithProviders(<CallsPanel
        projectId={mockProjectId}
        caseId={mockCaseId}
        caseStatusLabel="Work In Progress"
      />);

    const [, , stateKeys] = vi.mocked(useGetCallRequests).mock.calls[0];
    expect(stateKeys).toBeUndefined();
  });

  const listedCall = {
    isPending: false,
    data: {
      pages: [
        {
          callRequests: [
            {
              id: "cr-1",
              case: { id: "case-1", label: "Case" },
              state: { id: "canceled", label: "Canceled" },
            },
          ],
          totalRecords: 1,
          offset: 0,
          limit: 10,
        },
      ],
    },
    isError: false,
    refetch: vi.fn(),
    fetchNextPage: vi.fn(),
    hasNextPage: false,
    isFetchingNextPage: false,
    isFetchNextPageError: false,
  } as unknown as ReturnType<typeof useGetCallRequests>;

  const lastStateKeys = () =>
    vi.mocked(useGetCallRequests).mock.calls.at(-1)?.[2];

  it("searches every state until a status is chosen, then only that status's states", () => {
    vi.mocked(useGetCallRequests).mockReturnValue(listedCall);

    renderWithProviders(<CallsPanel
        projectId={mockProjectId}
        caseId={mockCaseId}
        caseStatusLabel="Work In Progress"
      />);

    expect(lastStateKeys()).toBeUndefined();

    fireEvent.mouseDown(screen.getByRole("combobox"));
    fireEvent.click(screen.getByRole("option", { name: "Cancelled" }));
    expect(lastStateKeys()).toEqual([CALL_REQUEST_STATE_CANCELLED]);

    fireEvent.mouseDown(screen.getByRole("combobox"));
    fireEvent.click(screen.getByRole("option", { name: "Completed" }));
    expect(lastStateKeys()).toEqual([CALL_REQUEST_STATE_CONCLUDED]);

    fireEvent.mouseDown(screen.getByRole("combobox"));
    fireEvent.click(screen.getByRole("option", { name: "All statuses" }));
    expect(lastStateKeys()).toBeUndefined();
  });

  it("searches every state that is neither Canceled nor Concluded for Pending", () => {
    vi.mocked(useGetCallRequests).mockReturnValue(listedCall);

    renderWithProviders(<CallsPanel
        projectId={mockProjectId}
        caseId={mockCaseId}
        caseStatusLabel="Work In Progress"
      />);

    fireEvent.mouseDown(screen.getByRole("combobox"));
    fireEvent.click(screen.getByRole("option", { name: "Pending" }));

    const keys = lastStateKeys() ?? [];
    expect([...keys].sort()).toEqual([1, 2, 3, 4, 5, 7]);
    expect(keys).not.toContain(CALL_REQUEST_STATE_CANCELLED);
    expect(keys).not.toContain(CALL_REQUEST_STATE_CONCLUDED);
  });

  it("keeps the dropdown, with a status-specific message, when a status has no call requests", () => {
    vi.mocked(useGetCallRequests).mockReturnValue(listedCall);

    renderWithProviders(<CallsPanel
        projectId={mockProjectId}
        caseId={mockCaseId}
        caseStatusLabel="Work In Progress"
      />);

    vi.mocked(useGetCallRequests).mockReturnValue({
      ...listedCall,
      data: {
        pages: [{ callRequests: [], totalRecords: 0, offset: 0, limit: 10 }],
      },
    } as unknown as ReturnType<typeof useGetCallRequests>);

    fireEvent.mouseDown(screen.getByRole("combobox"));
    fireEvent.click(screen.getByRole("option", { name: "Completed" }));

    expect(screen.getByText(/No call requests have this status/i)).toBeInTheDocument();
    expect(screen.getByRole("combobox")).toBeInTheDocument();
  });

  it("disables the dropdown until the first, unfiltered search finishes", () => {
    vi.mocked(useGetCallRequests).mockReturnValue({
      ...listedCall,
      isPending: true,
      data: undefined,
    } as unknown as ReturnType<typeof useGetCallRequests>);

    renderWithProviders(<CallsPanel
        projectId={mockProjectId}
        caseId={mockCaseId}
        caseStatusLabel="Work In Progress"
      />);

    expect(screen.getByRole("combobox")).toHaveAttribute("aria-disabled", "true");
  });

  it("keeps the dropdown enabled when a later search is pending after the first one finished", () => {
    vi.mocked(useGetCallRequests).mockReturnValue(listedCall);

    renderWithProviders(<CallsPanel
        projectId={mockProjectId}
        caseId={mockCaseId}
        caseStatusLabel="Work In Progress"
      />);

    fireEvent.mouseDown(screen.getByRole("combobox"));
    fireEvent.click(screen.getByRole("option", { name: "Cancelled" }));

    // Back to All statuses, whose cached search is gone: pending again.
    vi.mocked(useGetCallRequests).mockReturnValue({
      ...listedCall,
      isPending: true,
      data: undefined,
    } as unknown as ReturnType<typeof useGetCallRequests>);
    fireEvent.mouseDown(screen.getByRole("combobox"));
    fireEvent.click(screen.getByRole("option", { name: "All statuses" }));

    expect(screen.getByRole("combobox")).not.toHaveAttribute("aria-disabled", "true");
  });

  it("shows only the plain empty state, with no dropdown, when the case has no call requests", () => {
    vi.mocked(useGetCallRequests).mockReturnValue({
      ...listedCall,
      data: {
        pages: [{ callRequests: [], totalRecords: 0, offset: 0, limit: 10 }],
      },
    } as unknown as ReturnType<typeof useGetCallRequests>);

    renderWithProviders(<CallsPanel
        projectId={mockProjectId}
        caseId={mockCaseId}
        caseStatusLabel="Work In Progress"
      />);

    expect(screen.queryByRole("combobox")).not.toBeInTheDocument();
    expect(screen.getByText(/No call requests found for this case/i)).toBeInTheDocument();
  });

  it("should render error state", () => {
    vi.mocked(useGetCallRequests).mockReturnValue({
      isPending: false,
      isError: true,
      data: undefined,
      refetch: vi.fn(),
      fetchNextPage: vi.fn(),
      hasNextPage: false,
      isFetchingNextPage: false,
      isFetchNextPageError: false,
    } as unknown as ReturnType<typeof useGetCallRequests>);

    renderWithProviders(<CallsPanel
        projectId={mockProjectId}
        caseId={mockCaseId}
        caseStatusLabel="Work In Progress"
      />);
    expect(
      screen.getByText(/Error loading call requests/i),
    ).toBeInTheDocument();
  });

  it("should render call requests", () => {
    vi.mocked(useGetCallRequests).mockReturnValue({
      isPending: false,
      isError: false,
      refetch: vi.fn(),
      fetchNextPage: vi.fn(),
      hasNextPage: false,
      isFetchingNextPage: false,
      isFetchNextPageError: false,
      data: {
        pages: [
          {
            callRequests: [
              {
                id: "call-1",
                case: { id: "case-1", label: "CS0438719" },
                reason: "Test notes",
                preferredTimes: ["2024-10-29 14:00:00"],
                durationMin: 60,
                number: "CR-TEST",
                scheduleTime: "2024-11-05 14:00:00",
                createdOn: "2024-10-29 10:00:00",
                updatedOn: "2024-10-29 10:00:00",
                state: { id: "pending_on_wso2", label: "Pending on WSO2" },
              },
            ],
          },
        ],
      },
    } as unknown as ReturnType<typeof useGetCallRequests>);

    renderWithProviders(<CallsPanel
        projectId={mockProjectId}
        caseId={mockCaseId}
        caseStatusLabel="Work In Progress"
      />);
    expect(screen.getByText(/Call Request/i)).toBeInTheDocument();
    expect(screen.getByText(/Pending on WSO2/i)).toBeInTheDocument();
    expect(screen.getByText(/Test notes/i)).toBeInTheDocument();
  });

  it("should render empty state", () => {
    vi.mocked(useGetCallRequests).mockReturnValue({
      isPending: false,
      isError: false,
      refetch: vi.fn(),
      fetchNextPage: vi.fn(),
      hasNextPage: false,
      isFetchingNextPage: false,
      isFetchNextPageError: false,
      data: { pages: [{ callRequests: [] }] },
    } as unknown as ReturnType<typeof useGetCallRequests>);

    renderWithProviders(<CallsPanel
        projectId={mockProjectId}
        caseId={mockCaseId}
        caseStatusLabel="Work In Progress"
      />);
    expect(
      screen.getByText(/No call requests found for this case/i),
    ).toBeInTheDocument();
  });

  it("should show delete confirmation modal and call patch on confirm with reason", () => {
    const mockCall = {
      id: "call-1",
      case: { id: "case-1", label: "CS0438719" },
      reason: "Test notes",
      preferredTimes: ["2024-10-29 14:00:00"],
      durationMin: 60,
      number: "CR-TEST",
      scheduleTime: "2024-11-05 14:00:00",
      createdOn: "2024-10-29 10:00:00",
      updatedOn: "2024-10-29 10:00:00",
      state: { id: "pending_on_wso2", label: "Pending on WSO2" },
    };
    vi.mocked(useGetCallRequests).mockReturnValue({
      isPending: false,
      isError: false,
      refetch: vi.fn(),
      fetchNextPage: vi.fn(),
      hasNextPage: false,
      isFetchingNextPage: false,
      isFetchNextPageError: false,
      data: {
        pages: [{ callRequests: [mockCall] }],
      },
    } as unknown as ReturnType<typeof useGetCallRequests>);

    renderWithProviders(<CallsPanel
        projectId={mockProjectId}
        caseId={mockCaseId}
        caseStatusLabel="Work In Progress"
      />);

    fireEvent.click(screen.getByRole("button", { name: /Cancel/i }));

    expect(screen.getByText(/Are you sure you want to cancel/i)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /Go Back/i })).toBeInTheDocument();
    const confirmBtn = screen.getByRole("button", { name: /Confirm/i });
    expect(confirmBtn).toBeInTheDocument();
    expect(confirmBtn).toBeDisabled();

    const reasonInput = screen.getByLabelText(/Reason \*/i);
    fireEvent.change(reasonInput, { target: { value: "No longer needed" } });

    fireEvent.click(screen.getByRole("button", { name: /Confirm/i }));

    expect(mockPatchMutate).toHaveBeenCalledWith(
      expect.objectContaining({
        callRequestId: "call-1",
        cancellationReason: "No longer needed",
        stateKey: CALL_REQUEST_STATE_CANCELLED,
      }),
      expect.any(Object),
    );
  });

  it("should show Approve and Reject buttons for 'Pending on Customer' call", () => {
    vi.mocked(useGetCallRequests).mockReturnValue({
      isPending: false,
      isError: false,
      refetch: vi.fn(),
      fetchNextPage: vi.fn(),
      hasNextPage: false,
      isFetchingNextPage: false,
      isFetchNextPageError: false,
      data: {
        pages: [
          {
            callRequests: [
              {
                id: "call-2",
                case: { id: "case-1", label: "CS0438719" },
                reason: "Customer approval needed",
                preferredTimes: [],
                durationMin: 30,
                number: "CR-TEST",
                scheduleTime: "",
                createdOn: "2024-10-29 10:00:00",
                updatedOn: "2024-10-29 10:00:00",
                state: { id: "3", label: "Pending on Customer" },
              },
            ],
          },
        ],
      },
    } as unknown as ReturnType<typeof useGetCallRequests>);

    renderWithProviders(<CallsPanel
        projectId={mockProjectId}
        caseId={mockCaseId}
        caseStatusLabel="Work In Progress"
      />);
    expect(screen.getByRole("button", { name: /Approve/i })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /Reject/i })).toBeInTheDocument();
  });

  it("should open reject modal when Reject is clicked and patch with derived stateKey (no reason)", () => {
    vi.mocked(useGetCallRequests).mockReturnValue({
      isPending: false,
      isError: false,
      refetch: vi.fn(),
      fetchNextPage: vi.fn(),
      hasNextPage: false,
      isFetchingNextPage: false,
      isFetchNextPageError: false,
      data: {
        pages: [
          {
            callRequests: [
              {
                id: "call-2",
                case: { id: "case-1", label: "CS0438719" },
                reason: "Customer approval needed",
                preferredTimes: [],
                durationMin: 30,
                number: "CR-TEST",
                scheduleTime: "",
                createdOn: "2024-10-29 10:00:00",
                updatedOn: "2024-10-29 10:00:00",
                state: { id: "3", label: "Pending on Customer" },
              },
            ],
          },
        ],
      },
    } as unknown as ReturnType<typeof useGetCallRequests>);

    renderWithProviders(<CallsPanel
        projectId={mockProjectId}
        caseId={mockCaseId}
        caseStatusLabel="Work In Progress"
      />);
    fireEvent.click(screen.getByRole("button", { name: /^Reject$/i }));

    // Reject modal should open with a reason input
    expect(screen.getByText("Reject Call Request")).toBeInTheDocument();
    const reasonInput = screen.getByPlaceholderText(/Enter reason for rejection/i);
    expect(reasonInput).toBeInTheDocument();

    // Enter reason then confirm
    fireEvent.change(reasonInput, { target: { value: "Not available" } });
    fireEvent.click(screen.getAllByRole("button", { name: /^Reject$/i }).at(-1)!);

    expect(mockPatchMutate).toHaveBeenCalledWith(
      expect.objectContaining({
        callRequestId: "call-2",
        reason: "Not available",
        // stateKey is the fixed numeric constant (CALL_REQUEST_STATE_CUSTOMER_REJECTED),
        // not derived from the filter's own string id ("customer_rejected") --
        // the filter match only confirms the project still offers this state.
        stateKey: CALL_REQUEST_STATE_CUSTOMER_REJECTED,
      }),
      expect.any(Object),
    );
  });

  it("should open approve modal when Approve is clicked for 'Pending on Customer'", () => {
    vi.mocked(useGetCallRequests).mockReturnValue({
      isPending: false,
      isError: false,
      refetch: vi.fn(),
      fetchNextPage: vi.fn(),
      hasNextPage: false,
      isFetchingNextPage: false,
      isFetchNextPageError: false,
      data: {
        pages: [
          {
            callRequests: [
              {
                id: "call-2",
                case: { id: "case-1", label: "CS0438719" },
                reason: "Customer approval needed",
                preferredTimes: [],
                durationMin: 30,
                number: "CR-TEST",
                scheduleTime: "",
                createdOn: "2024-10-29 10:00:00",
                updatedOn: "2024-10-29 10:00:00",
                state: { id: "3", label: "Pending on Customer" },
              },
            ],
          },
        ],
      },
    } as unknown as ReturnType<typeof useGetCallRequests>);

    renderWithProviders(<CallsPanel
        projectId={mockProjectId}
        caseId={mockCaseId}
        caseStatusLabel="Work In Progress"
      />);
    fireEvent.click(screen.getByRole("button", { name: /Approve/i }));
    expect(screen.getByText(/Approve Call Request/i)).toBeInTheDocument();
    expect(
      screen.getByText(/Enter preferred time for this call request/i),
    ).toBeInTheDocument();
  });

  it("submits the Approve PATCH with the fixed numeric stateKey, not NaN from the filter's string id", () => {
    // Regression test for a real, reported bug: callRequestStates' own `id`
    // is entity-service's string enum key ("pending_on_wso2"), and the old
    // code derived approveStateKey via Number(found.id) -- always NaN for a
    // string id, which is not `undefined`, so validation incorrectly passed
    // and the PATCH was sent with `stateKey: NaN` (-> JSON null -> the Go
    // backend's int zero value 0), failing every customer Approve with
    // "Invalid request payload."
    vi.mocked(useGetCallRequests).mockReturnValue({
      isPending: false,
      isError: false,
      refetch: vi.fn(),
      fetchNextPage: vi.fn(),
      hasNextPage: false,
      isFetchingNextPage: false,
      isFetchNextPageError: false,
      data: {
        pages: [
          {
            callRequests: [
              {
                id: "call-2",
                case: { id: "case-1", label: "CS0438719" },
                reason: "Customer approval needed",
                preferredTimes: [],
                durationMin: 30,
                number: "CR-TEST",
                scheduleTime: "",
                createdOn: "2024-10-29 10:00:00",
                updatedOn: "2024-10-29 10:00:00",
                state: { id: "pending_on_customer", label: "Pending on Customer" },
              },
            ],
          },
        ],
      },
    } as unknown as ReturnType<typeof useGetCallRequests>);

    renderWithProviders(<CallsPanel
        projectId={mockProjectId}
        caseId={mockCaseId}
        caseStatusLabel="Work In Progress"
      />);
    fireEvent.click(screen.getByRole("button", { name: /Approve/i }));

    const preferredTimeInput = screen
      .getAllByLabelText(/Preferred Time \*/i)
      .find((el) => el.tagName === "INPUT") as HTMLInputElement;
    const future = new Date(Date.now() + 7 * 24 * 60 * 60 * 1000);
    fireEvent.change(preferredTimeInput, {
      target: { value: format(future, "MM/dd/yyyy hh:mm a") },
    });

    fireEvent.click(screen.getAllByRole("button", { name: /^Approve$/i }).at(-1)!);

    expect(mockPatchMutate).toHaveBeenCalledWith(
      expect.objectContaining({
        callRequestId: "call-2",
        stateKey: CALL_REQUEST_STATE_PENDING_ON_WSO2,
      }),
      expect.any(Object),
    );
    const [sentPayload] = mockPatchMutate.mock.calls.at(-1)!;
    expect(Number.isNaN(sentPayload.stateKey)).toBe(false);
  });

  it("should show table pagination and fetch next page when changing page", async () => {
    const mockFetchNextPage = vi.fn();
    const tenCalls = Array.from({ length: 10 }, (_, i) => ({
      id: `call-${i + 1}`,
      case: { id: "case-1", label: "CS0438719" },
      reason: `Notes ${i + 1}`,
      preferredTimes: [] as string[],
      durationMin: 60,
      number: "CR-TEST",
      scheduleTime: "",
      createdOn: "2024-10-29 10:00:00",
      updatedOn: "2024-10-29 10:00:00",
      state: { id: "1", label: "Pending" },
    }));
    vi.mocked(useGetCallRequests).mockReturnValue({
      isPending: false,
      isError: false,
      refetch: vi.fn(),
      fetchNextPage: mockFetchNextPage,
      hasNextPage: true,
      isFetchingNextPage: false,
      isFetchNextPageError: false,
      data: {
        pages: [
          {
            callRequests: tenCalls,
            totalRecords: 25,
            limit: 10,
            offset: 0,
          },
        ],
      },
    } as unknown as ReturnType<typeof useGetCallRequests>);

    renderWithProviders(<CallsPanel
        projectId={mockProjectId}
        caseId={mockCaseId}
        caseStatusLabel="Work In Progress"
      />);

    // ListPagination renders an MUI TablePagination ("1-10 of 25" + prev/next).
    expect(screen.getByText(/1.10 of 25/)).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: /Go to next page/i }));

    await waitFor(() => expect(mockFetchNextPage).toHaveBeenCalled());
  });

  it("should show missing timezone dialog when user has no timezone set", () => {
    vi.mocked(useGetUserDetails).mockReturnValue({
      data: { timeZone: null },
      refetch: vi.fn().mockResolvedValue({ data: { timeZone: null } }),
      isLoading: false,
      isError: false,
    } as unknown as ReturnType<typeof useGetUserDetails>);

    vi.mocked(useGetCallRequests).mockReturnValue({
      isPending: false,
      isError: false,
      refetch: vi.fn(),
      fetchNextPage: vi.fn(),
      hasNextPage: false,
      isFetchingNextPage: false,
      isFetchNextPageError: false,
      data: { pages: [{ callRequests: [] }] },
    } as unknown as ReturnType<typeof useGetCallRequests>);

    renderWithProviders(<CallsPanel
        projectId={mockProjectId}
        caseId={mockCaseId}
        caseStatusLabel="Work In Progress"
      />);
    expect(
      screen.getByRole("region", { name: /time zone required/i }),
    ).toBeInTheDocument();
    expect(
      screen.getByText(/Set your time zone first to request or reschedule a call/i),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: /^Request Call$/ }),
    ).not.toBeInTheDocument();
  });

  it("should block Request Call flow with time zone gate when profile has no timezone", () => {
    vi.mocked(useGetUserDetails).mockReturnValue({
      data: { timeZone: null },
      refetch: vi.fn().mockResolvedValue({ data: { timeZone: null } }),
      isLoading: false,
      isError: false,
    } as unknown as ReturnType<typeof useGetUserDetails>);

    vi.mocked(useGetCallRequests).mockReturnValue({
      isPending: false,
      isError: false,
      refetch: vi.fn(),
      fetchNextPage: vi.fn(),
      hasNextPage: false,
      isFetchingNextPage: false,
      isFetchNextPageError: false,
      data: { pages: [{ callRequests: [] }] },
    } as unknown as ReturnType<typeof useGetCallRequests>);

    renderWithProviders(
      <CallsPanel
        projectId={mockProjectId}
        caseId={mockCaseId}
        caseStatusLabel="Work In Progress"
      />,
    );
    expect(
      screen.getByText(/Set your time zone first to request or reschedule a call/i),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: /^Request Call$/ }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByPlaceholderText(
        /Describe your call request or topics you'd like to discuss/i,
      ),
    ).not.toBeInTheDocument();
  });

  it("should open Request Call modal when button is clicked", () => {
    vi.mocked(useGetCallRequests).mockReturnValue({
      isPending: false,
      isError: false,
      refetch: vi.fn(),
      fetchNextPage: vi.fn(),
      hasNextPage: false,
      isFetchingNextPage: false,
      isFetchNextPageError: false,
      data: { pages: [{ callRequests: [] }] },
    } as unknown as ReturnType<typeof useGetCallRequests>);

    renderWithProviders(<CallsPanel
        projectId={mockProjectId}
        caseId={mockCaseId}
        caseStatusLabel="Work In Progress"
      />);

    fireEvent.click(screen.getByRole("button", { name: /^Request Call$/ }));

    expect(screen.getByRole("dialog")).toBeInTheDocument();
    expect(document.getElementById("preferred-time-0")).toBeTruthy();
    expect(screen.getByLabelText(/^Meeting Duration \*$/i)).toBeInTheDocument();
    expect(
      screen.getByPlaceholderText(
        /Describe your call request or topics you'd like to discuss/i,
      ),
    ).toBeInTheDocument();
  });

  // Regression: entity-service's severityBasedAllocationTime is currently
  // always empty, so the picker's floor used to fall back to one flat
  // conservative default (or, before that fix, an even shorter "now + 1
  // minute") for every severity -- wrong for anything less urgent than
  // Catastrophic/P0. CALL_REQUEST_LEAD_MINUTES_BY_PRIORITY_ID mirrors
  // ServiceNow's own per-priority lead-time table so each severity gets its
  // real minimum, confirmed here against the picker's actual rendered min.
  it("floors the preferred time by the case's own severity (Catastrophic/P0 = 15 minutes)", async () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2026-10-11T09:00:00.000Z"));
    try {
      vi.mocked(useGetCallRequests).mockReturnValue({
        isPending: false,
        isError: false,
        refetch: vi.fn(),
        fetchNextPage: vi.fn(),
        hasNextPage: false,
        isFetchingNextPage: false,
        isFetchNextPageError: false,
        data: { pages: [{ callRequests: [] }] },
      } as unknown as ReturnType<typeof useGetCallRequests>);
      vi.mocked(useGetUserDetails).mockReturnValue({
        data: { timeZone: "UTC" },
        refetch: vi.fn().mockResolvedValue({ data: { timeZone: "UTC" } }),
        isLoading: false,
        isError: false,
      } as unknown as ReturnType<typeof useGetUserDetails>);

      renderWithProviders(<CallsPanel
          projectId={mockProjectId}
          caseId={mockCaseId}
          caseStatusLabel="Work In Progress"
          caseSeverityId="14"
        />);

      fireEvent.click(screen.getByRole("button", { name: /^Request Call$/ }));

      const input = document.getElementById(
        "preferred-time-0",
      ) as HTMLInputElement;
      // The create form is seeded with the floor in a microtask.
      await act(async () => {});
      // 09:00 UTC + 15 minutes (Catastrophic/P0's real ServiceNow offset,
      // not the 300-minute flat default a lower-severity case would still
      // fall back to) = 09:15. The new form starts on that floor, and a time
      // picked before it is moved back up to it.
      expect(input.value).toBe("10/11/2026 09:15 AM");
      fireEvent.change(input, { target: { value: "10/11/2026 08:00 AM" } });
      expect(input.value).toBe("10/11/2026 09:15 AM");
    } finally {
      vi.useRealTimers();
    }
  });

  // Regression: a non-finite allocation from the API (Infinity in
  // particular -- NaN was already excluded) used to still be treated as
  // "known" and returned as-is, skipping CALL_REQUEST_LEAD_MINUTES_BY_PRIORITY_ID
  // entirely. computeMinScheduleDatetimeLocalForTimeZone's own guard only
  // rejects NaN/negative/null/undefined, not Infinity, so a Catastrophic/P0
  // case with a bad API value got an unusable, far-future floor instead of
  // the correct 15 minutes. (A negative value alone would have been caught
  // downstream by that same guard either way -- Infinity is the case that
  // actually depends on CallsPanel.tsx's own check.)
  it("falls through to the priority table when the API's own allocation is invalid (non-finite)", async () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2026-10-11T09:00:00.000Z"));
    try {
      vi.mocked(useGetCallRequests).mockReturnValue({
        isPending: false,
        isError: false,
        refetch: vi.fn(),
        fetchNextPage: vi.fn(),
        hasNextPage: false,
        isFetchingNextPage: false,
        isFetchNextPageError: false,
        data: { pages: [{ callRequests: [] }] },
      } as unknown as ReturnType<typeof useGetCallRequests>);
      vi.mocked(useGetUserDetails).mockReturnValue({
        data: { timeZone: "UTC" },
        refetch: vi.fn().mockResolvedValue({ data: { timeZone: "UTC" } }),
        isLoading: false,
        isError: false,
      } as unknown as ReturnType<typeof useGetUserDetails>);
      mockUseGetProjectFilters.mockReturnValueOnce({
        data: {
          callRequestStates: [],
          severityBasedAllocationTime: { "14": Infinity },
        },
        isLoading: false,
        isError: false,
      });

      renderWithProviders(<CallsPanel
          projectId={mockProjectId}
          caseId={mockCaseId}
          caseStatusLabel="Work In Progress"
          caseSeverityId="14"
        />);

      fireEvent.click(screen.getByRole("button", { name: /^Request Call$/ }));

      const input = document.getElementById(
        "preferred-time-0",
      ) as HTMLInputElement;
      // The create form is seeded with the floor in a microtask.
      await act(async () => {});
      // Must still be 09:15 (the mapped 15-minute Catastrophic/P0 value),
      // not a floor computed from Infinity minutes.
      expect(input.value).toBe("10/11/2026 09:15 AM");
    } finally {
      vi.useRealTimers();
    }
  });
});
