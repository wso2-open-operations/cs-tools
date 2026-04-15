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
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { useGetCallRequests } from "@features/support/api/useGetCallRequests";
import useGetUserDetails from "@features/settings/api/useGetUserDetails";
import { CALL_REQUEST_STATE_CANCELLED } from "@features/support/constants/supportConstants";
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

vi.mock("@api/useGetProjectFilters", () => ({
  default: () => ({
    data: {
      callRequestStates: [
        { id: "1", label: "Pending" },
        { id: "2", label: "Pending on WSO2" },
        { id: "3", label: "Pending on Customer" },
        { id: "4", label: "Customer Rejected" },
        { id: "5", label: "Scheduled" },
      ],
    },
    isLoading: false,
    isError: false,
  }),
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
                state: { id: "1", label: "Pending on WSO2" },
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
      state: { id: "1", label: "Pending on WSO2" },
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
        // stateKey derived from filter: "Customer Rejected" → id "4" → number 4
        stateKey: 4,
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

  it("should show pagination like attachments and fetch next page when changing page", async () => {
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

    expect(
      screen.getByRole("navigation", { name: /pagination/i }),
    ).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: /Go to page 2/i }));

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
});
