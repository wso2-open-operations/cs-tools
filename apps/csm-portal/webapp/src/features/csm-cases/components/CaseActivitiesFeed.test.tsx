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
import "@testing-library/jest-dom/vitest";
import CaseActivitiesFeed from "@features/csm-cases/components/CaseActivitiesFeed";
import type { CaseAuditEntry } from "@features/csm-cases/types/csmCases";

describe("CaseActivitiesFeed", () => {
  it("renders a field_change entry with old/new values", () => {
    const entry: CaseAuditEntry = {
      id: "fc-1",
      kind: "field_change",
      actor: "Jane Doe",
      createdAt: "2026-07-01T00:00:00Z",
      changes: [
        {
          field: "state",
          fieldLabel: "State",
          previousValue: "In Progress",
          newValue: "Resolved",
        },
      ],
    };

    const { container } = render(
      <CaseActivitiesFeed comments={[]} audit={[entry]} attachments={[]} />,
    );

    expect(screen.getByText("State:")).toBeInTheDocument();
    // The line reads "State: Resolved was In Progress" across sibling text
    // nodes (new value, then the struck-through old value in its own span) —
    // assert on the row's combined text rather than a single text node.
    expect(container.textContent).toContain("Resolved");
    expect(container.textContent).toContain("was");
    expect(container.textContent).toContain("In Progress");
    expect(screen.getByText(/Jane Doe/)).toBeInTheDocument();
  });

  it("renders multiple field changes from one field_change entry", () => {
    const entry: CaseAuditEntry = {
      id: "fc-2",
      kind: "field_change",
      actor: "Jane Doe",
      createdAt: "2026-07-01T00:00:00Z",
      changes: [
        {
          field: "state",
          fieldLabel: "State",
          previousValue: "New",
          newValue: "In Progress",
        },
        {
          field: "assignedEngineer",
          fieldLabel: "Assignee",
          previousValue: undefined,
          newValue: "John Smith",
        },
      ],
    };

    render(
      <CaseActivitiesFeed comments={[]} audit={[entry]} attachments={[]} />,
    );

    expect(screen.getByText("State:")).toBeInTheDocument();
    expect(screen.getByText("Assignee:")).toBeInTheDocument();
    expect(screen.getByText("John Smith")).toBeInTheDocument();
    // No previous value on the "set" change — no strike-through text for it.
    expect(screen.queryByText("cleared")).not.toBeInTheDocument();
  });

  it("shows a cleared marker when newValue is empty", () => {
    const entry: CaseAuditEntry = {
      id: "fc-3",
      kind: "field_change",
      actor: "Jane Doe",
      createdAt: "2026-07-01T00:00:00Z",
      changes: [
        {
          field: "assignedEngineer",
          fieldLabel: "Assignee",
          previousValue: "John Smith",
          newValue: undefined,
        },
      ],
    };

    render(
      <CaseActivitiesFeed comments={[]} audit={[entry]} attachments={[]} />,
    );

    expect(screen.getByText("cleared")).toBeInTheDocument();
    expect(screen.getByText("John Smith")).toBeInTheDocument();
  });

  it("falls back to description when changes is absent", () => {
    const entry: CaseAuditEntry = {
      id: "fc-4",
      kind: "state_change",
      actor: "System",
      description: "Case moved to In Progress",
      createdAt: "2026-07-01T00:00:00Z",
    };

    render(
      <CaseActivitiesFeed comments={[]} audit={[entry]} attachments={[]} />,
    );

    expect(screen.getByText("Case moved to In Progress")).toBeInTheDocument();
  });
});
