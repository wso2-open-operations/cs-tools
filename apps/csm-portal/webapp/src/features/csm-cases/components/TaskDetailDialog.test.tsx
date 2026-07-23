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
import { describe, expect, it, vi, beforeEach } from "vitest";
import "@testing-library/jest-dom/vitest";
import { TaskDetailDialog } from "@features/csm-cases/components/TaskDetailDialog";
import type { BeTaskDetail } from "@api/backend/types";
import { useGetTask } from "@features/csm-cases/api/useGetTask";
import { useUpdateTask } from "@features/csm-cases/api/useUpdateTask";
import { useSearchUsers } from "@features/csm-users/api/useSearchUsers";

vi.mock("@features/csm-cases/api/useGetTask", () => ({
  useGetTask: vi.fn(),
}));
vi.mock("@features/csm-cases/api/useUpdateTask", () => ({
  useUpdateTask: vi.fn(),
}));
vi.mock("@features/csm-users/api/useSearchUsers", () => ({
  useSearchUsers: vi.fn(),
}));

const mockedUseGetTask = vi.mocked(useGetTask);
const mockedUseUpdateTask = vi.mocked(useUpdateTask);
const mockedUseSearchUsers = vi.mocked(useSearchUsers);

const TASK: BeTaskDetail = {
  id: "task-1",
  subject: "Extract logs for CS0440062",
  state: "OPEN",
  dueDate: null,
  visibleToCustomer: false,
  assignedTo: { id: "u-1", name: "Jane Doe" },
  requestType: "1",
  requestTypeLabel: "Log Extraction",
  environment: null,
  environmentLabel: null,
  product: null,
  parentCase: { id: "case-1", number: "CS0440062" },
  createdOn: "2026-07-01T00:00:00Z",
  updatedOn: "2026-07-15T10:00:00Z",
};

describe("TaskDetailDialog — inline state/assignee edits", () => {
  const mutate = vi.fn();

  beforeEach(() => {
    mockedUseGetTask.mockReturnValue({
      data: TASK,
      isLoading: false,
      isError: false,
      refetch: vi.fn(),
      // eslint-disable-next-line @typescript-eslint/no-explicit-any -- partial UseQueryResult stub
    } as any);
    mockedUseUpdateTask.mockReturnValue({
      mutate,
      isPending: false,
      isError: false,
      // eslint-disable-next-line @typescript-eslint/no-explicit-any -- partial UseMutationResult stub
    } as any);
    mockedUseSearchUsers.mockReturnValue({
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
        limit: 6,
        offset: 0,
        hasMore: false,
      },
      isFetching: false,
      isError: false,
      // eslint-disable-next-line @typescript-eslint/no-explicit-any -- partial UseQueryResult stub
    } as any);
    mutate.mockReset();
  });

  it("changes the task state via the inline select", () => {
    render(<TaskDetailDialog taskId="task-1" caseId="case-1" onClose={() => {}} />);
    fireEvent.mouseDown(screen.getByRole("combobox"));
    fireEvent.click(screen.getByRole("option", { name: "Closed" }));
    expect(mutate).toHaveBeenCalledWith({ state: "CLOSED" });
  });

  it("reassigns the task by picking an engineer from the inline search", () => {
    render(<TaskDetailDialog taskId="task-1" caseId="case-1" onClose={() => {}} />);
    fireEvent.click(screen.getByRole("button", { name: /change assignee/i }));
    fireEvent.change(screen.getByPlaceholderText(/search engineers/i), {
      target: { value: "jane" },
    });
    fireEvent.click(screen.getByRole("button", { name: /jane smith/i }));
    expect(mutate).toHaveBeenCalledWith({ assignedToEmail: "jane.smith@example.com" });
  });
});
