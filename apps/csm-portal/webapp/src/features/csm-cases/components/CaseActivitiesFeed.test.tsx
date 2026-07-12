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
import { formatAbsoluteForUser } from "@utils/dateTime";
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
    // The line reads "State: In Progress → Resolved" across sibling text nodes
    // (muted old value, arrow, then new value) — assert on combined row text.
    expect(container.textContent).toContain("In Progress");
    expect(container.textContent).toContain("→");
    expect(container.textContent).toContain("Resolved");
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

  it("renders a comment-style header (author + permalinked time) above the changes", () => {
    const entry: CaseAuditEntry = {
      id: "fc-5",
      kind: "field_change",
      actor: "Jane Doe",
      createdAt: "2026-07-01T10:15:00Z",
      changes: [
        {
          field: "state",
          fieldLabel: "State",
          previousValue: "In Progress",
          newValue: "Resolved",
        },
      ],
    };

    render(
      <CaseActivitiesFeed comments={[]} audit={[entry]} attachments={[]} />,
    );

    expect(screen.getByText("Jane Doe")).toBeInTheDocument();
    expect(screen.getByText("Lifecycle")).toBeInTheDocument();
    // The time is a permalink anchor to the entry, same pattern comments use.
    const permalink = document.querySelector(`a[href="#${entry.id}"]`);
    expect(permalink).not.toBeNull();
  });

  it("renders every backend-provided change line, even one matching the entry's own timestamp", () => {
    const entry: CaseAuditEntry = {
      id: "fc-6",
      kind: "field_change",
      actor: "Jane Doe",
      createdAt: "2026-07-01T10:15:00Z",
      changes: [
        {
          field: "state",
          fieldLabel: "State",
          previousValue: "In Progress",
          newValue: "Resolved",
        },
        {
          field: "resolvedAt",
          fieldLabel: "Resolved On",
          previousValue: undefined,
          // Field-level curation is a backend concern now — the FE renders
          // whatever `changes[]` it receives, with no client-side dropping.
          newValue: "2026-07-01 10:15:22",
        },
      ],
    };

    render(
      <CaseActivitiesFeed comments={[]} audit={[entry]} attachments={[]} />,
    );

    expect(screen.getByText("State:")).toBeInTheDocument();
    expect(screen.getByText("Resolved On:")).toBeInTheDocument();
  });

  it("does not suppress a timestamp change when it differs from the entry's own time", () => {
    const entry: CaseAuditEntry = {
      id: "fc-7",
      kind: "field_change",
      actor: "Jane Doe",
      createdAt: "2026-07-01T10:15:00Z",
      changes: [
        {
          field: "dueDate",
          fieldLabel: "Due Date",
          previousValue: undefined,
          newValue: "2026-08-15 09:00:00",
        },
      ],
    };

    const { container } = render(
      <CaseActivitiesFeed comments={[]} audit={[entry]} attachments={[]} />,
    );

    expect(screen.getByText("Due Date:")).toBeInTheDocument();
    // The timestamp value must be routed through the shared user-timezone
    // formatter, not shown as the raw backend string.
    expect(screen.queryByText("2026-08-15 09:00:00")).not.toBeInTheDocument();
    const expected = formatAbsoluteForUser("2026-08-15 09:00:00");
    expect(expected).not.toBeNull();
    expect(container.textContent).toContain(expected);
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
