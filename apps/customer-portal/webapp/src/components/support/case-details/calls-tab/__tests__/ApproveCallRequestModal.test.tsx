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
import { render, screen, fireEvent } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { usePatchCallRequest } from "@api/usePatchCallRequest";
import ApproveCallRequestModal from "@case-details-calls/ApproveCallRequestModal";
import type { CallRequest } from "@models/responses";

vi.mock("@api/usePatchCallRequest");

const mockMutate = vi.fn();

function createTestQueryClient() {
  return new QueryClient({ defaultOptions: { queries: { retry: false } } });
}

function renderWithProviders(ui: ReactElement) {
  const queryClient = createTestQueryClient();
  return render(
    <QueryClientProvider client={queryClient}>{ui}</QueryClientProvider>,
  );
}

const mockCall: CallRequest = {
  id: "call-1",
  case: { id: "case-1", label: "CS0438719" },
  reason: "Test notes",
  preferredTimes: ["2024-10-29 14:00:00"],
  durationMin: 60,
  scheduleTime: "2024-11-05 14:00:00",
  createdOn: "2024-10-29 10:00:00",
  updatedOn: "2024-10-29 10:00:00",
  state: { id: "3", label: "Pending on Customer" },
};

const mockCallNoPreferredTimes: CallRequest = {
  ...mockCall,
  preferredTimes: [],
  scheduleTime: "",
};

const defaultProps = {
  open: true,
  call: mockCall,
  projectId: "project-1",
  caseId: "case-1",
  onClose: vi.fn(),
  onSuccess: vi.fn(),
  onError: vi.fn(),
  approveStateKey: 2,
};

describe("ApproveCallRequestModal", () => {
  beforeEach(() => {
    mockMutate.mockClear();
    vi.mocked(usePatchCallRequest).mockReturnValue({
      mutate: mockMutate,
      isPending: false,
    } as unknown as ReturnType<typeof usePatchCallRequest>);
  });

  it("should render the modal with title and subtitle", () => {
    renderWithProviders(<ApproveCallRequestModal {...defaultProps} />);
    expect(screen.getByText("Approve Call Request")).toBeInTheDocument();
    expect(
      screen.getByText(/Enter preferred time for this call request/i),
    ).toBeInTheDocument();
  });

  it("should render the preferred time input prefilled from API wall clock", () => {
    renderWithProviders(<ApproveCallRequestModal {...defaultProps} />);
    expect(screen.getByDisplayValue("2024-10-29T14:00")).toBeInTheDocument();
    expect(screen.getAllByLabelText(/Preferred Time/i).length).toBeGreaterThan(0);
  });

  it("should have Approve button disabled when time is not entered", () => {
    renderWithProviders(
      <ApproveCallRequestModal
        {...defaultProps}
        call={mockCallNoPreferredTimes}
      />,
    );
    expect(screen.getByRole("button", { name: /^Approve$/i })).toBeDisabled();
  });

  it("should enable Approve button when a preferred time is entered", () => {
    renderWithProviders(
      <ApproveCallRequestModal
        {...defaultProps}
        call={mockCallNoPreferredTimes}
      />,
    );
    const input = document.querySelector<HTMLInputElement>(
      "#approve-preferred-time-0",
    );
    if (!input) throw new Error("Preferred time input not found");
    fireEvent.change(input, { target: { value: "2099-12-31T14:00" } });
    expect(screen.getByRole("button", { name: /^Approve$/i })).not.toBeDisabled();
  });

  it("should not render the modal when open is false", () => {
    renderWithProviders(<ApproveCallRequestModal {...defaultProps} open={false} />);
    expect(screen.queryByText("Approve Call Request")).not.toBeInTheDocument();
  });

  it("should call onClose when Cancel is clicked", () => {
    const onClose = vi.fn();
    renderWithProviders(<ApproveCallRequestModal {...defaultProps} onClose={onClose} />);
    fireEvent.click(screen.getByRole("button", { name: /^Cancel$/i }));
    expect(onClose).toHaveBeenCalled();
  });
});
