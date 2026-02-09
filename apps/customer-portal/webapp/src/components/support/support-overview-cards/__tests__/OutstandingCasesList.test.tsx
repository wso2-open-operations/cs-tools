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
import { describe, expect, it } from "vitest";
import OutstandingCasesList from "@components/support/support-overview-cards/OutstandingCasesList";
import type { CaseListItem } from "@models/responses";

const mockCases: CaseListItem[] = [
  {
    id: "1",
    internalId: "int-1",
    number: "CASE-2845",
    createdOn: new Date(Date.now() - 2 * 60 * 60 * 1000).toISOString(),
    title: "API Gateway timeout issues",
    description: "",
    assignedEngineer: "Sarah Chen",
    project: { id: "p1", label: "Project 1" },
    type: null,
    deployment: null,
    severity: { id: "1", label: "High" },
    status: { id: "1", label: "Work In Progress" },
  },
];

describe("OutstandingCasesList", () => {
  it("should render empty state when no cases", () => {
    render(<OutstandingCasesList cases={[]} />);
    expect(screen.getByText("No outstanding cases.")).toBeInTheDocument();
  });

  it("should render case number, title, severity and status", () => {
    render(<OutstandingCasesList cases={mockCases} />);
    expect(screen.getByText("CASE-2845")).toBeInTheDocument();
    expect(screen.getByText("API Gateway timeout issues")).toBeInTheDocument();
    expect(screen.getByText("High")).toBeInTheDocument();
    expect(screen.getByText("Work In Progress")).toBeInTheDocument();
    expect(screen.getByText(/Assigned to Sarah Chen/)).toBeInTheDocument();
    expect(screen.getByText("2 hours ago")).toBeInTheDocument();
  });
});
