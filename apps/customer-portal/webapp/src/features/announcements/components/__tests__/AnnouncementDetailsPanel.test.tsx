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
