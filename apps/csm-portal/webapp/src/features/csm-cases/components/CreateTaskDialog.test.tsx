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

import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import CreateTaskDialog from "@features/csm-cases/components/CreateTaskDialog";
import { useSearchUsers } from "@features/csm-users/api/useSearchUsers";

vi.mock("@features/csm-users/api/useSearchUsers", () => ({
  useSearchUsers: vi.fn(),
}));

const mockUseSearchUsers = vi.mocked(useSearchUsers);

function mockCandidates(): void {
  mockUseSearchUsers.mockReturnValue({
    data: {
      users: [
        {
          id: "u-2",
          userName: "jsmith",
          name: "Jane Smith",
          email: "jane.smith@example.com",
          timezone: null,
          active: true,
        },
      ],
      total: 1,
      limit: 8,
      offset: 0,
      hasMore: false,
    },
    isFetching: false,
    isError: false,
    // eslint-disable-next-line @typescript-eslint/no-explicit-any -- partial UseQueryResult stub
  } as any);
}

describe("CreateTaskDialog — task creation", () => {
  it("disables Create task until a subject is entered", () => {
    mockCandidates();
    render(
      <CreateTaskDialog isSaving={false} onClose={() => {}} onSubmit={() => {}} />,
    );
    expect(screen.getByRole("button", { name: /create task/i })).toBeDisabled();
  });

  it("submits just the subject when no other field is set", () => {
    mockCandidates();
    const onSubmit = vi.fn();
    render(
      <CreateTaskDialog isSaving={false} onClose={() => {}} onSubmit={onSubmit} />,
    );
    fireEvent.change(screen.getByLabelText(/subject/i), {
      target: { value: "Extract logs" },
    });
    fireEvent.click(screen.getByRole("button", { name: /create task/i }));
    expect(onSubmit).toHaveBeenCalledWith({ subject: "Extract logs" });
  });

  it("picks an assignee from the search results and includes assignedToEmail on submit", () => {
    mockCandidates();
    const onSubmit = vi.fn();
    render(
      <CreateTaskDialog isSaving={false} onClose={() => {}} onSubmit={onSubmit} />,
    );
    fireEvent.change(screen.getByLabelText(/subject/i), {
      target: { value: "Extract logs" },
    });
    fireEvent.change(
      screen.getByPlaceholderText(/search engineers by name or email/i),
      { target: { value: "jane" } },
    );
    fireEvent.click(screen.getByRole("button", { name: /jane smith/i }));
    fireEvent.click(screen.getByRole("button", { name: /create task/i }));
    expect(onSubmit).toHaveBeenCalledWith(
      expect.objectContaining({
        subject: "Extract logs",
        assignedToEmail: "jane.smith@example.com",
      }),
    );
  });

  it("includes visibleToCustomer when the switch is toggled on", () => {
    mockCandidates();
    const onSubmit = vi.fn();
    render(
      <CreateTaskDialog isSaving={false} onClose={() => {}} onSubmit={onSubmit} />,
    );
    fireEvent.change(screen.getByLabelText(/subject/i), {
      target: { value: "Extract logs" },
    });
    fireEvent.click(screen.getByLabelText(/visible to customer/i));
    fireEvent.click(screen.getByRole("button", { name: /create task/i }));
    expect(onSubmit).toHaveBeenCalledWith(
      expect.objectContaining({ visibleToCustomer: true }),
    );
  });

  it("calls onClose on Cancel without calling onSubmit", () => {
    mockCandidates();
    const onSubmit = vi.fn();
    const onClose = vi.fn();
    render(
      <CreateTaskDialog isSaving={false} onClose={onClose} onSubmit={onSubmit} />,
    );
    fireEvent.click(screen.getByRole("button", { name: /^cancel$/i }));
    expect(onClose).toHaveBeenCalled();
    expect(onSubmit).not.toHaveBeenCalled();
  });
});
