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

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { IssueTimelineRow } from "./IssueTimelineRow";
import type { IssueRow } from "@api/types";
import type { IssueRowVariant } from "@lib/grid";

function issue(id: number, overrides: Partial<IssueRow> = {}): IssueRow {
  return {
    id,
    number: id,
    state: "OPEN",
    url: `https://github.com/example/repo/issues/${id}`,
    repo: "example/repo",
    priority: "High(P2)",
    currentStatus: "Open",
    githubCreatedAt: "2026-01-01T00:00:00Z",
    githubUpdatedAt: "2026-01-01T00:00:00Z",
    sla: null,
    title: null,
    abtTeam: null,
    openedBy: null,
    ...overrides,
  };
}

function renderRow(row: IssueRow, variant?: IssueRowVariant) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={queryClient}>
      <IssueTimelineRow issue={row} variant={variant} isCsStatus={() => false} />
    </QueryClientProvider>,
  );
}

describe("IssueTimelineRow title", () => {
  it("renders issue.title as the row's title text when it's set", () => {
    renderRow(issue(101, { title: "Fix the widget" }));
    expect(screen.getByText("Fix the widget")).toBeTruthy();
    expect(screen.getByText("#101")).toBeTruthy();
  });

  it("renders just the #number, with no extra title text, when issue.title is null", () => {
    const { container } = renderRow(issue(102, { title: null }));
    expect(screen.getByText("#102")).toBeTruthy();
    expect(container.querySelector(".MuiSkeleton-root")).toBeNull();
  });
});

describe("IssueTimelineRow Opened by cell", () => {
  it("variant=\"full\" shows the full email address when issue.openedBy is set", () => {
    renderRow(issue(201, { openedBy: "person@wso2.com" }), "full");
    expect(screen.getByText("person@wso2.com")).toBeTruthy();
  });

  it("variant=\"full\" shows — when issue.openedBy is null", () => {
    // A non-null sla keeps the Budget cell from also rendering "—", so the
    // dash below is unambiguously the Opened by cell's.
    renderRow(
      issue(202, {
        openedBy: null,
        sla: { budgetHours: 48, consumedHours: 10, remainingHours: 38, pctConsumed: 0.2, slaState: "OK", slaRunning: true },
      }),
      "full",
    );
    expect(screen.getByText("—")).toBeTruthy();
  });

  it("variant=\"compact\" (the default) renders no Opened by cell at all", () => {
    renderRow(issue(203, { openedBy: "person@wso2.com" }));
    expect(screen.queryByText("person@wso2.com")).toBeNull();
  });
});

describe("IssueTimelineRow Created/Updated cells", () => {
  it('variant="full" renders relative Created and Updated times instead of a single Age cell', () => {
    renderRow(issue(301), "full");
    // Exact text depends on real "now" relative to the fixture's fixed
    // timestamps, so assert there are two relative-time strings, not a
    // specific value.
    const times = screen.getAllByText(/ago|from now|just now/);
    expect(times.length).toBe(2);
  });

  it('variant="compact" (the default) still shows the Age cell via fmtAge, not a relative-time string', () => {
    renderRow(issue(302, { githubCreatedAt: new Date(Date.now() - 3 * 3_600_000).toISOString() }));
    expect(screen.getByText(/^\d+h$/)).toBeTruthy();
    expect(screen.queryByText(/ago|from now|just now/)).toBeNull();
  });
});
