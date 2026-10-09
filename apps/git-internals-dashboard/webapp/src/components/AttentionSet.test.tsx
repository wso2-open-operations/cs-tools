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
import { AttentionSet } from "./AttentionSet";

const issues = [
  ...Array.from({ length: 12 }, (_, i) => ({ id: i + 1, number: i + 1, currentStatus: "Open", sla: { slaState: "VIOLATED" } })),
  ...Array.from({ length: 12 }, (_, i) => ({ id: i + 101, number: i + 101, currentStatus: "WOC", sla: { slaState: "OK" } })),
];

vi.mock("@api/hooks", () => ({ useIssues: () => ({ data: { issues } }) }));
vi.mock("@components/IssueTimelineRow", () => ({
  IssueTimelineRow: ({ issue }: { issue: { number: number } }) => <div data-testid="row">#{issue.number}</div>,
}));

const hero = { violated: { n: 12 }, atRisk: { n: 0 }, cs: { n: 12 } } as never;
const renderSet = () =>
  render(<AttentionSet hero={hero} projects={[]} isCsStatus={(s) => s === "WOC"} />);

describe("AttentionSet", () => {
  it("shows at most 10 rows even when more issues qualify", () => {
    renderSet();
    expect(screen.getAllByTestId("row")).toHaveLength(10);
  });

  it("still fills up to 10 rows after a chip is toggled off", () => {
    renderSet();
    fireEvent.click(screen.getByRole("button", { name: /Violated/ }));
    const rows = screen.getAllByTestId("row");
    expect(rows).toHaveLength(10);
    expect(rows[0]).toHaveTextContent("#101");
  });
});
