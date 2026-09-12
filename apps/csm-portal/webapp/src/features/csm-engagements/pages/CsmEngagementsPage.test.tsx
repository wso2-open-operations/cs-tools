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
// its own tests -- this file only checks the props `CsmEngagementsPage`
// supplies to it, in particular the `title` override behavior.
const issuesViewSpy = vi.fn();
vi.mock("@features/csm-cases/components/CsmIssuesView", () => ({
  default: (props: Record<string, unknown>) => {
    issuesViewSpy(props);
    return <div>IssuesView</div>;
  },
}));

import CsmEngagementsPage from "@features/csm-engagements/pages/CsmEngagementsPage";

// digiops-cs#2914: a dashboard widget's click-through carries its own
// displayName as the `wt` query param (see
// `WIDGET_RESOURCE_CONFIG.engagement.buildHref`), which this page must
// render as its heading instead of the hardcoded default -- but only when
// present, so a normal left-nav visit (no `wt` param) is completely
// unaffected.
describe("CsmEngagementsPage — title override from a dashboard widget click-through", () => {
  it('defaults to "Engagements" when no wt param is present (normal nav visit)', () => {
    issuesViewSpy.mockClear();
    render(
      <MemoryRouter initialEntries={["/engagements"]}>
        <CsmEngagementsPage />
      </MemoryRouter>,
    );

    expect(screen.getByText("IssuesView")).toBeInTheDocument();
    expect(issuesViewSpy).toHaveBeenCalledTimes(1);
    const props = issuesViewSpy.mock.calls[0][0] as Record<string, unknown>;
    expect(props.title).toBe("Engagements");
    // Every other prop this page locks stays intact -- the title override
    // is purely additive.
    expect(props.lockedFilters).toEqual({ caseTypes: ["engagement"] });
  });

  it("uses the wt param as the title when present (widget click-through)", () => {
    issuesViewSpy.mockClear();
    render(
      <MemoryRouter initialEntries={["/engagements?wt=Total+Outstanding&states=open"]}>
        <CsmEngagementsPage />
      </MemoryRouter>,
    );

    const props = issuesViewSpy.mock.calls[0][0] as Record<string, unknown>;
    expect(props.title).toBe("Total Outstanding");
  });
});
