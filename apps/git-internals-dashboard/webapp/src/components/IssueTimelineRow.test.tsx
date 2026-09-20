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
import { render } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { IssueTimelineRow } from "./IssueTimelineRow";
import type { IssueRow } from "@api/types";

function issue(id: number): IssueRow {
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
  };
}

function renderRow(id: number) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={queryClient}>
      <IssueTimelineRow issue={issue(id)} titleLoading isCsStatus={() => false} />
    </QueryClientProvider>,
  );
}

describe("IssueTimelineRow title skeleton", () => {
  it("sizes the loading skeleton to a real, varying width instead of a fixed tiny box", () => {
    const { container } = renderRow(101);
    const skeleton = container.querySelector(".MuiSkeleton-root") as HTMLElement;
    expect(skeleton).toBeTruthy();

    const widthPct = parseFloat(window.getComputedStyle(skeleton).width);
    expect(widthPct).toBeGreaterThanOrEqual(30);
    expect(widthPct).toBeLessThanOrEqual(85);
  });

  it("is stable for the same issue across renders but varies across different issues", () => {
    const { container: a1 } = renderRow(202);
    const { container: a2 } = renderRow(202);
    const { container: b } = renderRow(303);

    const widthOf = (c: HTMLElement) => window.getComputedStyle(c.querySelector(".MuiSkeleton-root") as HTMLElement).width;

    expect(widthOf(a1)).toBe(widthOf(a2));
    expect(widthOf(a1)).not.toBe(widthOf(b));
  });
});
