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
import ProblemActionBar from "@features/csm-operations/components/ProblemActionBar";
import type { BeProblemDetail, BeProblemState } from "@api/backend/types";

function problem(state: BeProblemState): BeProblemDetail {
  return { id: "prb-1", number: "PRB0040000", state };
}

describe("ProblemActionBar", () => {
  it.each([
    ["NEW", "assess", "Move to Assess"],
    ["ASSESS", "confirm", "Move to Root Cause Analysis"],
    ["ROOT_CAUSE_ANALYSIS", "fix", "Move to Fix In Progress"],
    ["FIX_IN_PROGRESS", "resolve", "Move to Resolved"],
    ["RESOLVED", "close", "Move to Closed"],
  ] as const)(
    "renders the single legal forward transition from %s and dispatches transition=%s on click",
    (state, transition, label) => {
      const onAction = vi.fn();
      render(<ProblemActionBar problem={problem(state)} isPending={false} onAction={onAction} />);
      const btn = screen.getByRole("button", { name: label });
      fireEvent.click(btn);
      expect(onAction).toHaveBeenCalledWith(transition);
    },
  );

  it("renders nothing for a Closed (terminal) problem", () => {
    const { container } = render(
      <ProblemActionBar problem={problem("CLOSED")} isPending={false} onAction={vi.fn()} />,
    );
    expect(container).toBeEmptyDOMElement();
  });

  it("renders nothing when the problem has no state", () => {
    const { container } = render(
      <ProblemActionBar problem={{ id: "prb-1" }} isPending={false} onAction={vi.fn()} />,
    );
    expect(container).toBeEmptyDOMElement();
  });

  it("disables the button while a transition is pending", () => {
    render(<ProblemActionBar problem={problem("NEW")} isPending onAction={vi.fn()} />);
    expect(screen.getByRole("button", { name: "Move to Assess" })).toBeDisabled();
  });
});
