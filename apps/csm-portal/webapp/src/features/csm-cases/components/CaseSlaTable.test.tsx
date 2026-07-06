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
import { CaseSlaTable } from "@features/csm-cases/components/CaseSlaTable";
import type { CaseSla, CaseSlaList } from "@features/csm-cases/types/csmCases";
import { useGetCsmCaseSlas } from "@features/csm-cases/api/useGetCsmCaseSlas";

vi.mock("@features/csm-cases/api/useGetCsmCaseSlas", () => ({
  useGetCsmCaseSlas: vi.fn(),
}));

const mockedUseGetCsmCaseSlas = vi.mocked(useGetCsmCaseSlas);

const SLA_ROW: CaseSla = {
  id: "sla-1",
  definition: "S1 - Response",
  target: "1 Business Hour",
  stage: "in_progress",
  stageLabel: "In progress",
  hasBreached: false,
  businessTimeLeftLabel: "30 minutes",
  businessElapsedLabel: "30 minutes",
  businessElapsedPercent: 50,
  startTime: "2026-07-01T10:00:00Z",
  stopTime: null,
};

function mockResult(overrides: Partial<ReturnType<typeof useGetCsmCaseSlas>>): void {
  mockedUseGetCsmCaseSlas.mockReturnValue({
    data: null,
    isLoading: false,
    isError: false,
    isFetching: false,
    dataUpdatedAt: 0,
    refetch: vi.fn(),
    ...overrides,
  } as unknown as ReturnType<typeof useGetCsmCaseSlas>);
}

describe("CaseSlaTable", () => {
  beforeEach(() => {
    mockedUseGetCsmCaseSlas.mockReset();
  });

  it("renders a loading skeleton while the query is in flight", () => {
    mockResult({ isLoading: true });
    render(<CaseSlaTable caseId="case-1" />);
    expect(screen.getByText("SLAs")).toBeInTheDocument();
  });

  it("renders an empty state when there are no SLAs", () => {
    const empty: CaseSlaList = { caseId: "case-1", count: 0, slas: [] };
    mockResult({ data: empty });
    render(<CaseSlaTable caseId="case-1" />);
    expect(screen.getByText("No SLAs on this case.")).toBeInTheDocument();
  });

  it("renders SLA rows with definition, stage, and time labels", () => {
    const list: CaseSlaList = { caseId: "case-1", count: 1, slas: [SLA_ROW] };
    mockResult({ data: list });
    render(<CaseSlaTable caseId="case-1" />);
    expect(screen.getByText("S1 - Response")).toBeInTheDocument();
    expect(screen.getByText("In progress")).toBeInTheDocument();
    expect(screen.getAllByText("30 minutes").length).toBeGreaterThan(0);
    expect(screen.getByText("50%")).toBeInTheDocument();
    // Stop time is null → rendered as an em dash.
    expect(screen.getByText("—")).toBeInTheDocument();
  });

  it("renders an error state with a retry action that refetches", () => {
    const refetch = vi.fn();
    mockResult({ isError: true, refetch });
    render(<CaseSlaTable caseId="case-1" />);
    expect(screen.getByText("Could not load SLAs for this case.")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: /retry/i }));
    expect(refetch).toHaveBeenCalled();
  });

  it("calls refetch when the header refresh button is clicked", () => {
    const refetch = vi.fn();
    const list: CaseSlaList = { caseId: "case-1", count: 1, slas: [SLA_ROW] };
    mockResult({ data: list, refetch });
    render(<CaseSlaTable caseId="case-1" />);
    fireEvent.click(screen.getByRole("button", { name: /refresh slas/i }));
    expect(refetch).toHaveBeenCalled();
  });

  it("shows a truncation notice when fewer rows are rendered than the total count", () => {
    const list: CaseSlaList = { caseId: "case-1", count: 51, slas: [SLA_ROW] };
    mockResult({ data: list });
    render(<CaseSlaTable caseId="case-1" />);
    expect(screen.getByText("Showing first 1 of 51")).toBeInTheDocument();
  });
});
