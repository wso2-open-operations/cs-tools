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
import { TasksWidget } from "@features/csm-cases/components/TasksWidget";
import type { BeListCaseTasksResponse, BeTaskDetail } from "@api/backend/types";
import { useGetCaseTasks } from "@features/csm-cases/api/useGetCaseTasks";
import { useGetTask } from "@features/csm-cases/api/useGetTask";

vi.mock("@features/csm-cases/api/useGetCaseTasks", () => ({
  useGetCaseTasks: vi.fn(),
}));
vi.mock("@features/csm-cases/api/useGetTask", () => ({
  useGetTask: vi.fn(),
}));

const mockedUseGetCaseTasks = vi.mocked(useGetCaseTasks);
const mockedUseGetTask = vi.mocked(useGetTask);

function mockListResult(overrides: Partial<ReturnType<typeof useGetCaseTasks>>): void {
  mockedUseGetCaseTasks.mockReturnValue({
    data: null,
    isLoading: false,
    isError: false,
    isFetching: false,
    refetch: vi.fn(),
    ...overrides,
  } as unknown as ReturnType<typeof useGetCaseTasks>);
}

function mockDetailResult(overrides: Partial<ReturnType<typeof useGetTask>>): void {
  mockedUseGetTask.mockReturnValue({
    data: null,
    isLoading: false,
    isError: false,
    refetch: vi.fn(),
    ...overrides,
  } as unknown as ReturnType<typeof useGetTask>);
}

const TASK_ROW = {
  id: "task-1",
  subject: "Extract logs for CS0440062",
  state: "OPEN" as const,
  dueDate: null,
  assignedTo: { id: "u-1", name: "Jane Doe" },
  updatedOn: "2026-07-15T10:00:00Z",
};

describe("TasksWidget", () => {
  beforeEach(() => {
    mockedUseGetCaseTasks.mockReset();
    mockedUseGetTask.mockReset();
    mockDetailResult({});
  });

  it("renders a loading skeleton while the query is in flight", () => {
    mockListResult({ isLoading: true });
    render(<TasksWidget caseId="case-1" />);
    expect(screen.getByText("Tasks")).toBeInTheDocument();
  });

  it("renders an empty state when there are no tasks", () => {
    const empty: BeListCaseTasksResponse = { tasks: [], total: 0, offset: 0, limit: 20 };
    mockListResult({ data: empty });
    render(<TasksWidget caseId="case-1" />);
    expect(screen.getByText("No tasks on this case.")).toBeInTheDocument();
  });

  it("renders an error state with a retry action that refetches", () => {
    const refetch = vi.fn();
    mockListResult({ isError: true, refetch });
    render(<TasksWidget caseId="case-1" />);
    expect(screen.getByText("Could not load tasks for this case.")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: /retry/i }));
    expect(refetch).toHaveBeenCalled();
  });

  it("renders a row with subject, state, assignee, and a dash for a missing due date", () => {
    const list: BeListCaseTasksResponse = {
      tasks: [TASK_ROW],
      total: 1,
      offset: 0,
      limit: 20,
    };
    mockListResult({ data: list });
    render(<TasksWidget caseId="case-1" />);
    expect(screen.getByText("Extract logs for CS0440062")).toBeInTheDocument();
    expect(screen.getByText("Open")).toBeInTheDocument();
    expect(screen.getByText("Jane Doe")).toBeInTheDocument();
    // dueDate is null on this row — rendered as a plain dash, not a broken cell.
    expect(screen.getAllByText("—").length).toBeGreaterThan(0);
  });

  it("renders a neutral badge for an OTHER/null state instead of assuming only OPEN/CLOSED", () => {
    const list: BeListCaseTasksResponse = {
      tasks: [
        { ...TASK_ROW, id: "task-2", state: "OTHER" },
        { ...TASK_ROW, id: "task-3", state: null, subject: "Task with null state" },
      ],
      total: 2,
      offset: 0,
      limit: 20,
    };
    mockListResult({ data: list });
    render(<TasksWidget caseId="case-1" />);
    expect(screen.getByText("Other")).toBeInTheDocument();
    // A null state falls back to the "—" label rather than crashing or
    // showing "undefined".
    expect(screen.getByText("Task with null state")).toBeInTheDocument();
  });

  it("opens the task detail dialog when a row is clicked", () => {
    const list: BeListCaseTasksResponse = {
      tasks: [TASK_ROW],
      total: 1,
      offset: 0,
      limit: 20,
    };
    mockListResult({ data: list });
    const detail: BeTaskDetail = {
      id: "task-1",
      subject: TASK_ROW.subject,
      state: "OPEN",
      dueDate: null,
      visibleToCustomer: false,
      assignedTo: TASK_ROW.assignedTo,
      requestType: "1",
      requestTypeLabel: "Log Extraction",
      environment: null,
      environmentLabel: null,
      product: null,
      parentCase: { id: "case-1", number: "CS0440062" },
      createdOn: "2026-07-01T00:00:00Z",
      updatedOn: "2026-07-15T10:00:00Z",
    };
    mockDetailResult({ data: detail });

    render(<TasksWidget caseId="case-1" />);
    fireEvent.click(screen.getByRole("button", { name: /view task/i }));

    expect(screen.getByText("Task details")).toBeInTheDocument();
    expect(screen.getByText("Log Extraction")).toBeInTheDocument();
    expect(screen.getByText("CS0440062")).toBeInTheDocument();
  });
});
