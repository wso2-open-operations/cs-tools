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
import { describe, it, expect } from "vitest";
import TimeTrackingCard from "@time-tracking/TimeTrackingCard";
import type { TimeCard } from "@/types/timeTracking";

describe("TimeTrackingCard", () => {
  const mockCard: TimeCard = {
    id: "1",
    totalTime: 60,
    createdOn: "2025-12-10 03:47:10",
    hasBillable: true,
    state: { id: "approved", label: "Approved" },
    approvedBy: { id: "a1", label: "Dileepa Peiris (Intern)" },
    project: { id: "p1", label: "Customer 3 Project 1" },
    case: {
      number: "CS0437343",
      id: "c1",
      label: "test - image upload in Desc",
    },
  };

  it("should render card with all information", () => {
    render(<TimeTrackingCard card={mockCard} />);

    expect(screen.getByText("test - image upload in Desc")).toBeInTheDocument();
    expect(screen.getByText("Approved")).toBeInTheDocument();
    expect(screen.getByText("Billable")).toBeInTheDocument();
    expect(screen.getByText("CS0437343")).toBeInTheDocument();
    expect(screen.getByText("1 hr")).toBeInTheDocument(); // 60 minutes = 1 hour
    expect(
      screen.getByText(/Approved by: Dileepa Peiris \(Intern\)/),
    ).toBeInTheDocument();
  });

  it("should show fallback '--' for missing values", () => {
    const incompleteCard: TimeCard = {
      ...mockCard,
      state: null,
      approvedBy: null,
      case: { number: "", id: "c1", label: "" },
      totalTime: 0,
    };

    render(<TimeTrackingCard card={incompleteCard} />);

    expect(screen.getByText(/Approved by: --/)).toBeInTheDocument();
    expect(screen.getAllByText("--").length).toBeGreaterThan(0);
  });

  it("should not render Billable chip when hasBillable is false", () => {
    const nonBillableCard: TimeCard = {
      ...mockCard,
      hasBillable: false,
    };

    render(<TimeTrackingCard card={nonBillableCard} />);

    expect(screen.queryByText("Billable")).not.toBeInTheDocument();
    expect(screen.getByText("test - image upload in Desc")).toBeInTheDocument();
  });
});
