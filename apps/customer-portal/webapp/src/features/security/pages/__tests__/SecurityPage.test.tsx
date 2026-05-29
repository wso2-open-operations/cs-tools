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
import SecurityPage from "@features/security/pages/SecurityPage";
import { SecurityStatKey } from "@features/security/types/security";
import { CaseStatus } from "@features/support/constants/supportConstants";

const mockSetSearchParams = vi.fn();
const mockNavigate = vi.fn();

vi.mock("react-router", () => ({
  useParams: () => ({ projectId: "p-1" }),
  useSearchParams: () => [new URLSearchParams(""), mockSetSearchParams],
}));

vi.mock("@hooks/useModifierAwareNavigate", () => ({
  useModifierAwareNavigate: () => mockNavigate,
}));

vi.mock("@api/useGetProjectDetails", () => ({
  default: () => ({ data: { type: { label: "Enterprise" } } }),
}));
vi.mock("@api/useGetProjectFeatures", () => ({
  default: () => ({ data: {}, isLoading: false }),
}));
vi.mock("@api/useGetProjectFilters", () => ({
  default: () => ({
    data: {
      caseStates: [
        { id: "1", label: CaseStatus.OPEN },
        { id: "2", label: CaseStatus.CLOSED },
      ],
    },
  }),
}));

vi.mock("@utils/permission", () => ({
  getProjectPermissions: () => ({
    hasSecurityReportAnalysis: true,
    hasComponentAnalysis: true,
  }),
}));

vi.mock("@features/security/components/SecurityStats", () => ({
  default: ({ onStatClick }: { onStatClick: (k: SecurityStatKey) => void }) => (
    <button onClick={() => onStatClick(SecurityStatKey.resolvedSecurityReports)}>
      trigger-stat
    </button>
  ),
}));
vi.mock("@components/tab-bar/TabBar", () => ({
  default: () => <div>tab-bar</div>,
}));
vi.mock("@features/security/components/ProductVulnerabilitiesTable", () => ({
  default: () => <div>product-table</div>,
}));
vi.mock("@features/security/components/SecurityReportAnalysis", () => ({
  default: () => <div>report-analysis</div>,
}));

describe("SecurityPage", () => {
  it("shows stats and default tab content", () => {
    render(<SecurityPage />);
    expect(screen.getByText("trigger-stat")).toBeInTheDocument();
    expect(screen.getByText("tab-bar")).toBeInTheDocument();
    expect(screen.getByText("report-analysis")).toBeInTheDocument();
  });

  it("applies stat filter mode and supports back action", () => {
    render(<SecurityPage />);
    fireEvent.click(screen.getByText("trigger-stat"));
    expect(screen.getByRole("button", { name: "Back" })).toBeInTheDocument();
    expect(screen.queryByText("tab-bar")).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Back" }));
    expect(screen.getByText("tab-bar")).toBeInTheDocument();
  });
});
