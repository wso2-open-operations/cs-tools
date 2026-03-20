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

import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, it, expect, vi } from "vitest";
import UpdateHistoryTab from "@components/project-details/deployments/UpdateHistoryTab";
import type { ProductUpdate } from "@models/responses";

describe("UpdateHistoryTab", () => {
  const mockUpdates: ProductUpdate[] = [
    {
      updateLevel: 18,
      date: "2026-02-17",
      details: "Performance improvements and bug fixes",
    },
    {
      updateLevel: 15,
      date: "2026-02-03",
      details: "Security patch for CVE-2026-0001",
    },
  ];

  it("renders update history timeline", () => {
    const mockSave = vi.fn();
    render(
      <UpdateHistoryTab
        updates={mockUpdates}
        isLoading={false}
        onSaveUpdates={mockSave}
      />,
    );

    expect(screen.getByText(/U18/i)).toBeInTheDocument();
    expect(screen.getByText(/U15/i)).toBeInTheDocument();
    expect(
      screen.getByText(/Performance improvements and bug fixes/i),
    ).toBeInTheDocument();
  });

  it("displays current update level", () => {
    const mockSave = vi.fn();
    render(
      <UpdateHistoryTab
        updates={mockUpdates}
        isLoading={false}
        onSaveUpdates={mockSave}
      />,
    );

    expect(screen.getByText(/Current Update Level:/i)).toBeInTheDocument();
    expect(screen.getByText(/U18/i)).toBeInTheDocument();
  });

  it("shows add update form", () => {
    const mockSave = vi.fn();
    render(
      <UpdateHistoryTab
        updates={mockUpdates}
        isLoading={false}
        onSaveUpdates={mockSave}
      />,
    );

    expect(screen.getByLabelText(/Update Level \*/i)).toBeInTheDocument();
    expect(screen.getByLabelText(/Applied On \*/i)).toBeInTheDocument();
    expect(screen.getByLabelText(/Description/i)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /Add Update/i })).toBeInTheDocument();
  });

  it("calls onSaveUpdates when adding a new update", async () => {
    const user = userEvent.setup();
    const mockSave = vi.fn().mockResolvedValue(undefined);
    render(
      <UpdateHistoryTab
        updates={mockUpdates}
        isLoading={false}
        onSaveUpdates={mockSave}
      />,
    );

    const updateLevelInput = screen.getByLabelText(/Update Level \*/i);
    const dateInput = screen.getByLabelText(/Applied On \*/i);
    const addButton = screen.getByRole("button", { name: /Add Update/i });

    await user.type(updateLevelInput, "20");
    await user.type(dateInput, "2026-03-20");
    await user.click(addButton);

    await waitFor(() => {
      expect(mockSave).toHaveBeenCalledWith([
        ...mockUpdates,
        {
          updateLevel: 20,
          date: "2026-03-20",
          details: undefined,
        },
      ]);
    });
  });

  it("shows loading skeleton when isLoading is true", () => {
    const mockSave = vi.fn();
    render(
      <UpdateHistoryTab
        updates={[]}
        isLoading={true}
        onSaveUpdates={mockSave}
      />,
    );

    const skeletons = screen.getAllByTestId("skeleton");
    expect(skeletons.length).toBeGreaterThan(0);
  });

  it("displays 'No update history available' when updates array is empty", () => {
    const mockSave = vi.fn();
    render(
      <UpdateHistoryTab
        updates={[]}
        isLoading={false}
        onSaveUpdates={mockSave}
      />,
    );

    expect(
      screen.getByText(/No update history available/i),
    ).toBeInTheDocument();
  });
});
