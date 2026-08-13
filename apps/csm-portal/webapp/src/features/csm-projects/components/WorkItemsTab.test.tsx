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
import { describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import WorkItemsTab from "@features/csm-projects/components/WorkItemsTab";

const mockCsmIssuesView = vi.fn();

vi.mock("@features/csm-cases/components/CsmIssuesView", () => ({
  default: (props: Record<string, unknown>) => {
    mockCsmIssuesView(props);
    return <div>IssuesView: {props.entityNoun as string}</div>;
  },
}));

vi.mock("@features/csm-projects/components/ConversationsTab", () => ({
  default: ({ projectId }: { projectId: string }) => <div>Conversations for {projectId}</div>,
}));

describe("WorkItemsTab", () => {
  it("defaults to the Cases sub-tab, locked to this project's case-type cases", () => {
    render(<WorkItemsTab projectId="proj-1" />);

    expect(screen.getByText("IssuesView: cases")).toBeInTheDocument();
    expect(mockCsmIssuesView).toHaveBeenCalledWith(
      expect.objectContaining({
        entityNoun: "cases",
        lockedFilters: { projects: ["proj-1"], caseTypes: ["case"] },
        hideProjectFilter: true,
        hideTypeFilter: true,
      }),
    );
  });

  it("switches to the Service requests sub-tab, routed to the operations detail page", () => {
    render(<WorkItemsTab projectId="proj-1" />);

    fireEvent.click(screen.getByText("Service requests"));

    expect(screen.getByText("IssuesView: service requests")).toBeInTheDocument();
    expect(mockCsmIssuesView).toHaveBeenCalledWith(
      expect.objectContaining({
        lockedFilters: { projects: ["proj-1"], caseTypes: ["service_request"] },
        detailBasePath: "/operations/service-requests",
      }),
    );
  });

  it("switches to the Security reports sub-tab with severity hidden", () => {
    render(<WorkItemsTab projectId="proj-1" />);

    fireEvent.click(screen.getByText("Security reports"));

    expect(screen.getByText("IssuesView: security reports")).toBeInTheDocument();
    expect(mockCsmIssuesView).toHaveBeenCalledWith(
      expect.objectContaining({
        lockedFilters: { projects: ["proj-1"], caseTypes: ["security_report_analysis"] },
        hideSeverityColumn: true,
        detailBasePath: "/security-center/security-reports",
      }),
    );
  });

  it("switches to the Engagements sub-tab with the engagement-type filter shown", () => {
    render(<WorkItemsTab projectId="proj-1" />);

    fireEvent.click(screen.getByText("Engagements"));

    expect(screen.getByText("IssuesView: engagements")).toBeInTheDocument();
    expect(mockCsmIssuesView).toHaveBeenCalledWith(
      expect.objectContaining({
        lockedFilters: { projects: ["proj-1"], caseTypes: ["engagement"] },
        showEngagementTypeFilter: true,
        hideSeverityColumn: true,
        detailBasePath: "/engagements",
      }),
    );
  });

  it("switches to the Conversations sub-tab", () => {
    render(<WorkItemsTab projectId="proj-1" />);

    fireEvent.click(screen.getByText("Chats"));

    expect(screen.getByText("Conversations for proj-1")).toBeInTheDocument();
  });
});
