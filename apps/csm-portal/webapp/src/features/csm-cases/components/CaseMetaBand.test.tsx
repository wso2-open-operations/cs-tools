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
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { describe, expect, it, vi } from "vitest";
import { MemoryRouter } from "react-router";
import "@testing-library/jest-dom/vitest";

// `UserRefLink` (rendered for "Created by"/"Assignee") resolves an unknown id
// through `useResolvedUserId`, which needs the real API client — same
// approach as CaseDetailWidgets.test.tsx.
vi.mock("@api/backend/client", () => ({
  useBackendApi: () => ({ post: vi.fn().mockResolvedValue({ users: [] }) }),
}));

import CaseMetaBand from "@features/csm-cases/components/CaseMetaBand";
import type { CsmCaseDetail } from "@features/csm-cases/types/csmCases";

/** A complete, minimal case-detail fixture; tests override the fix-ETA fields. */
const BASE_CASE: CsmCaseDetail = {
  id: "case-1001",
  caseNumber: "CS-1001",
  wso2CaseId: "ACMESUB-1001",
  subject: "Identity Server token issuance latency spike",
  customer: "Acme Financial",
  accountId: "acc-001",
  projectId: "prj-acme-iam-prod",
  projectName: "IAM Production",
  product: "WSO2 Identity Server",
  severity: "S1",
  state: "work_in_progress",
  workState: "ongoing",
  assignee: "Jane Doe",
  assigneeIsMe: true,
  slaClockType: "first_response",
  minutesToBreach: 120,
  hasSla: false,
  createdAt: "2026-01-01T00:00:00.000Z",
  updatedAt: "2026-01-01T01:00:00.000Z",
  description: "Token issuance latency has spiked.",
  assignmentGroup: "grp.cre_team",
  customerContext: {
    accountName: "Acme Financial",
    tier: "enterprise",
    region: "us-east-1",
    primaryContact: "Jane Doe",
    primaryContactEmail: "jane.doe@example.com",
    accountManager: "John Roe",
    openCases: 1,
  },
  productContext: {
    product: "WSO2 Identity Server",
    version: "7.1.0",
    deployment: "IAM Production",
    environment: "prod",
  },
  watchers: [],
  linkedItems: [],
  tags: [],
  timeLogs: [],
  audit: [],
  attachments: [],
  isWatching: false,
};

function renderBand(overrides: Partial<CsmCaseDetail>): ReturnType<typeof render> {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter>
        <CaseMetaBand
          detail={{ ...BASE_CASE, ...overrides }}
          collapsed={false}
          onToggleCollapsed={() => {}}
        />
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

describe("CaseMetaBand — fix ETA cells", () => {
  it("renders all three fix ETA cells when all three values are set", () => {
    renderBand({
      bestCaseFixEta: "2026-08-01",
      mostLikelyFixEta: "2026-08-05",
      worstCaseFixEta: "2026-08-10",
    });

    expect(screen.getByText("Best case fix ETA")).toBeInTheDocument();
    expect(screen.getByText("Aug 1, 2026")).toBeInTheDocument();
    expect(screen.getByText("Most likely fix ETA")).toBeInTheDocument();
    expect(screen.getByText("Aug 5, 2026")).toBeInTheDocument();
    expect(screen.getByText("Worst case fix ETA")).toBeInTheDocument();
    expect(screen.getByText("Aug 10, 2026")).toBeInTheDocument();
  });

  it("renders no fix ETA cells when all three values are absent", () => {
    renderBand({
      bestCaseFixEta: null,
      mostLikelyFixEta: null,
      worstCaseFixEta: null,
    });

    expect(screen.queryByText("Best case fix ETA")).not.toBeInTheDocument();
    expect(screen.queryByText("Most likely fix ETA")).not.toBeInTheDocument();
    expect(screen.queryByText("Worst case fix ETA")).not.toBeInTheDocument();
  });

  it("renders only the fix ETA cells with a value when only a subset is set", () => {
    renderBand({
      bestCaseFixEta: "2026-08-01",
      mostLikelyFixEta: null,
      worstCaseFixEta: undefined,
    });

    expect(screen.getByText("Best case fix ETA")).toBeInTheDocument();
    expect(screen.getByText("Aug 1, 2026")).toBeInTheDocument();
    expect(screen.queryByText("Most likely fix ETA")).not.toBeInTheDocument();
    expect(screen.queryByText("Worst case fix ETA")).not.toBeInTheDocument();
  });
});

describe("CaseMetaBand — CRE / SRE team", () => {
  it("renders each present team as a clickable chip, not plain text", () => {
    renderBand({
      customerContext: {
        ...BASE_CASE.customerContext,
        creTeam: { id: "team-cre-1", name: "CRE Alpha" },
        sreTeam: { id: "team-sre-1", name: "SRE Bravo" },
      },
    });

    expect(screen.getByText("CRE / SRE team")).toBeInTheDocument();

    const creChip = screen.getByText("CRE Alpha");
    const sreChip = screen.getByText("SRE Bravo");
    expect(creChip).toBeInTheDocument();
    expect(sreChip).toBeInTheDocument();
    // DirectoryEntityChip renders an MUI Chip — a clickable element, unlike
    // the plain <Typography> the old join rendered.
    expect(creChip.closest('[role="button"]')).not.toBeNull();
    expect(sreChip.closest('[role="button"]')).not.toBeNull();
  });

  it("renders only the present team's chip when just one is set", () => {
    renderBand({
      customerContext: {
        ...BASE_CASE.customerContext,
        creTeam: { id: "team-cre-1", name: "CRE Alpha" },
        sreTeam: undefined,
      },
    });

    expect(screen.getByText("CRE Alpha")).toBeInTheDocument();
    expect(screen.queryByText("SRE Bravo")).not.toBeInTheDocument();
  });

  it("omits the CRE / SRE team cell entirely when neither team is set", () => {
    renderBand({
      customerContext: {
        ...BASE_CASE.customerContext,
        creTeam: undefined,
        sreTeam: undefined,
      },
    });

    expect(screen.queryByText("CRE / SRE team")).not.toBeInTheDocument();
  });
});
