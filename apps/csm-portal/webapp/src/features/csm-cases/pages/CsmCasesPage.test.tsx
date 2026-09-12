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
import { MemoryRouter } from "react-router";
import { describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";

// `CsmIssuesView` does all the real search/filtering work and is covered by
// its own tests — this file only checks the props `CsmCasesPage` (the
// "Support" section's page) supplies to it, in particular that it no longer
// suppresses the case-type control (digiops-cs#2907: "case-type filter is
// hidden on the Support section").
const issuesViewSpy = vi.fn();
vi.mock("@features/csm-cases/components/CsmIssuesView", () => ({
  default: (props: Record<string, unknown>) => {
    issuesViewSpy(props);
    return <div>IssuesView</div>;
  },
}));

import CsmCasesPage from "@features/csm-cases/pages/CsmCasesPage";

describe("CsmCasesPage — case-type filter visibility", () => {
  it("no longer passes hideTypeFilter, so the case-type control is shown", () => {
    issuesViewSpy.mockClear();
    render(
      <MemoryRouter>
        <CsmCasesPage />
      </MemoryRouter>,
    );

    expect(screen.getByText("IssuesView")).toBeInTheDocument();
    expect(issuesViewSpy).toHaveBeenCalledTimes(1);
    const props = issuesViewSpy.mock.calls[0][0] as Record<string, unknown>;
    expect(props.hideTypeFilter).toBeUndefined();
    // `lockedFilters` is still passed (drives severity-filter-visibility /
    // column-default hints inside CsmIssuesView) but, with the control
    // visible, no longer pins the *query* -- see
    // CsmIssuesView.typeFilterLock.test.tsx for that behavior.
    expect(props.lockedFilters).toEqual({ caseTypes: ["case"] });
  });
});

// digiops-cs#2914: a dashboard widget's click-through carries its own
// displayName as the `wt` query param (see
// `WIDGET_RESOURCE_CONFIG.case.buildHref`), which this page must render as
// its heading instead of the hardcoded default -- but only when present, so
// a normal left-nav visit (no `wt` param) is completely unaffected.
describe("CsmCasesPage — title override from a dashboard widget click-through", () => {
  it('defaults to "Cases" when no wt param is present (normal nav visit)', () => {
    issuesViewSpy.mockClear();
    render(
      <MemoryRouter initialEntries={["/cases"]}>
        <CsmCasesPage />
      </MemoryRouter>,
    );

    const props = issuesViewSpy.mock.calls[0][0] as Record<string, unknown>;
    expect(props.title).toBe("Cases");
  });

  it("uses the wt param as the title when present (widget click-through)", () => {
    issuesViewSpy.mockClear();
    render(
      <MemoryRouter initialEntries={["/cases?wt=Total+Outstanding&states=open"]}>
        <CsmCasesPage />
      </MemoryRouter>,
    );

    const props = issuesViewSpy.mock.calls[0][0] as Record<string, unknown>;
    expect(props.title).toBe("Total Outstanding");
  });
});
