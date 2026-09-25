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
import { describe, expect, it, vi } from "vitest";
import AnnouncementDetailsPanel from "@features/announcements/components/AnnouncementDetailsPanel";

vi.mock("@features/support/components/case-details/header/CaseDetailsActionRow", () => ({
  default: () => <div data-testid="case-details-actions" />,
}));

// AnnouncementActivityPanel pulls in useGetCaseCommentsInfinite -> useLogger,
// which requires a LoggerProvider this test doesn't set up -- irrelevant to
// what's under test here (the description's own sanitize/render behavior).
vi.mock("@features/announcements/components/AnnouncementActivityPanel", () => ({
  default: () => <div data-testid="announcement-activity" />,
}));

vi.mock("@utils/useDarkMode", () => ({
  useDarkMode: () => false,
}));

describe("AnnouncementDetailsPanel", () => {
  it("renders announcement title and description heading", () => {
    render(
      <AnnouncementDetailsPanel
        data={{
          title: "Maintenance window",
          number: "ANN-100",
          description: "<p>Scheduled downtime</p>",
          status: { id: "1", label: "Open" },
          createdOn: "2024-01-15T10:00:00Z",
        } as never}
        isLoading={false}
        isError={false}
        caseId="case-1"
        projectId="proj-1"
        onBack={() => {}}
      />,
    );
    expect(screen.getByText("Maintenance window")).toBeInTheDocument();
    expect(screen.getByText("Description")).toBeInTheDocument();
    expect(screen.getByText("Back")).toBeInTheDocument();
  });

  it("renders a table in the description instead of stripping it", () => {
    // Regression test: the description used to be sanitized with the
    // case/change-request policy (DESCRIPTION_PURIFY_CONFIG), which forbids
    // table tags and their contents -- silently dropping things like an EOL
    // announcement's product-version table. See AnnouncementDetailsPanel's
    // own sanitize call for the fix.
    const { container } = render(
      <AnnouncementDetailsPanel
        data={{
          title: "EOL notice",
          number: "ANN-101",
          description:
            "<p>Affected versions:</p><table><tbody><tr><td>API Manager</td><td>4.2.0</td></tr></tbody></table>",
          status: { id: "1", label: "Open" },
          createdOn: "2024-01-15T10:00:00Z",
        } as never}
        isLoading={false}
        isError={false}
        caseId="case-1"
        projectId="proj-1"
        onBack={() => {}}
      />,
    );
    // Assert the table structure itself survived, not just its text -- a
    // sanitizer config that unwraps <table>/<tr>/<td> but keeps their text
    // content would still pass a text-only assertion here.
    const table = container.querySelector("table");
    expect(table).not.toBeNull();
    expect(table).toHaveTextContent("API Manager");
    expect(table).toHaveTextContent("4.2.0");
  });

  it("shows a Security chip when the case's announcementType is SECURITY", () => {
    render(
      <AnnouncementDetailsPanel
        data={{
          title: "Critical vulnerability notice",
          number: "ANN-102",
          description: "<p>Details</p>",
          status: { id: "1", label: "Open" },
          createdOn: "2024-01-15T10:00:00Z",
          announcementType: "SECURITY",
        } as never}
        isLoading={false}
        isError={false}
        caseId="case-1"
        projectId="proj-1"
        onBack={() => {}}
      />,
    );
    expect(screen.getByText("Security")).toBeInTheDocument();
  });

  it("shows no Security chip when the case's announcementType is GENERAL or absent", () => {
    render(
      <AnnouncementDetailsPanel
        data={{
          title: "Maintenance window",
          number: "ANN-100",
          description: "<p>Details</p>",
          status: { id: "1", label: "Open" },
          createdOn: "2024-01-15T10:00:00Z",
        } as never}
        isLoading={false}
        isError={false}
        caseId="case-1"
        projectId="proj-1"
        onBack={() => {}}
      />,
    );
    expect(screen.queryByText("Security")).not.toBeInTheDocument();
  });

  it("shows a Security chip via the Security Announcement tag when announcementType predates the column (historical data)", () => {
    render(
      <AnnouncementDetailsPanel
        data={{
          title: "Critical vulnerability notice",
          number: "ANN-090",
          description: "<p>Details</p>",
          status: { id: "1", label: "Open" },
          createdOn: "2023-06-01T10:00:00Z",
          tags: [{ id: "tag-1", label: "Security Announcement" }],
        } as never}
        isLoading={false}
        isError={false}
        caseId="case-1"
        projectId="proj-1"
        onBack={() => {}}
      />,
    );
    expect(screen.getByText("Security")).toBeInTheDocument();
  });

  it("renders back button while loading", () => {
    render(
      <AnnouncementDetailsPanel
        data={undefined}
        isLoading
        isError={false}
        caseId="case-1"
        projectId="proj-1"
        onBack={() => {}}
      />,
    );
    expect(screen.getByText("Back")).toBeInTheDocument();
  });
});
