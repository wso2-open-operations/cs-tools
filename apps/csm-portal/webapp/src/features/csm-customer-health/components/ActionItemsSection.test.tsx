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

import "@testing-library/jest-dom/vitest";
import { fireEvent, render, screen, within } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import ActionItemsSection from "./ActionItemsSection";
import type { ProjectDetail, ProjectHealthStatus, RiskActionItem } from "../api/customerHealthTypes";

vi.mock("@context/success-banner/SuccessBannerContext", () => ({
  useSuccessBanner: () => ({ showSuccess: vi.fn() }),
}));
vi.mock("@context/error-banner/ErrorBannerContext", () => ({
  useErrorBanner: () => ({ showError: vi.fn() }),
}));

const item: RiskActionItem = {
  id: 11,
  riskId: 9,
  projectSysId: "p1",
  accountSysId: "a1",
  title: "Book a renewal call",
  description: null,
  priority: "high",
  status: "open",
  assignedToEmail: null,
  dueDate: null,
  resolutionComment: null,
  resolvedByEmail: null,
  resolvedOn: null,
  createdByEmail: "csm@example.test",
  createdOn: "2026-10-01T09:00:00Z",
  updatedOn: "2026-10-01T09:00:00Z",
  commentCount: 0,
};

// One array for every render: the section copies the items' comment counts
// into state whenever the list changes, so a new array each time never settles.
const items = [item];
const noComments: never[] = [];
const mutation = () => ({ mutateAsync: vi.fn(), isPending: false });
vi.mock("../api/useActionItems", () => ({
  useAccountActionItems: () => ({ data: items, isLoading: false, refetch: vi.fn() }),
  useCreateActionItem: () => mutation(),
  useUpdateActionItemStatus: () => mutation(),
  useActionItemComments: () => ({ data: noComments, isLoading: false }),
  useCreateActionItemComment: () => mutation(),
}));

const projects = [{ sysId: "p1", name: "Acme Cloud" } as ProjectDetail];
const healthStatusData: ProjectHealthStatus[] = [
  {
    projectSysId: "p1",
    healthStatus: { id: 1, projectSysId: "p1", accountSysId: "a1", status: "at_risk", reviewedByEmail: null, reviewedOn: null },
    openRisk: {
      id: 9,
      projectSysId: "p1",
      accountSysId: "a1",
      status: "open",
      openedComment: "Renewal stalled",
      openedByEmail: "csm@example.test",
      openedOn: "2026-10-01T09:00:00Z",
      closedComment: null,
      actionItems: [item],
    },
  },
];

function renderSection() {
  render(<ActionItemsSection accountId="a1" projects={projects} healthStatusData={healthStatusData} />);
}

function moveItemTo(status: "Resolved" | "Cancelled") {
  fireEvent.mouseDown(screen.getByRole("combobox"));
  fireEvent.click(screen.getByRole("option", { name: status }));
}

describe("ActionItemsSection: required fields", () => {
  it("marks the Due Date required with one asterisk, as every required field has", () => {
    renderSection();
    fireEvent.click(screen.getByRole("button", { name: "Add Action Item" }));
    const dialog = screen.getByRole("dialog", { name: "Add Action Item" });
    // The label's own text is the name alone; the asterisk is MUI's, once.
    const label = within(dialog).getByText("Due Date");
    expect(label.querySelectorAll(".MuiFormLabel-asterisk")).toHaveLength(1);
    expect(within(dialog).getByRole("textbox", { name: "Title" })).toBeRequired();
  });

  it("asks for a resolution comment, marked required, to resolve an item", () => {
    renderSection();
    moveItemTo("Resolved");
    const dialog = screen.getByRole("dialog", { name: "Resolve Action Item" });
    const box = within(dialog).getByRole("textbox", { name: "Resolution comment" });
    expect(box).toBeRequired();
    expect(dialog.querySelector(`label[for="${box.id}"] .MuiFormLabel-asterisk`)).toBeInTheDocument();
    const confirm = within(dialog).getByRole("button", { name: "Confirm" });
    expect(confirm).toBeDisabled();
    fireEvent.change(box, { target: { value: "Call booked." } });
    expect(confirm).toBeEnabled();
  });

  it("asks for a cancellation reason, marked required, to cancel an item", () => {
    renderSection();
    moveItemTo("Cancelled");
    const dialog = screen.getByRole("dialog", { name: "Cancel Action Item" });
    const box = within(dialog).getByRole("textbox", { name: "Cancellation reason" });
    expect(box).toBeRequired();
    expect(within(dialog).getByRole("button", { name: "Confirm" })).toBeDisabled();
  });
});
