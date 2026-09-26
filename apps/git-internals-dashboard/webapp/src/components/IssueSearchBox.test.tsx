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

import { act, fireEvent, render, screen } from "@testing-library/react";
import { createMemoryRouter, RouterProvider } from "react-router";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { IssueSearchBox } from "./IssueSearchBox";

function renderSearchBox(initialEntries: string[] = ["/issues"]) {
  const router = createMemoryRouter([{ path: "/issues", element: <IssueSearchBox /> }], { initialEntries });
  render(<RouterProvider router={router} />);
  return router;
}

describe("IssueSearchBox", () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.runOnlyPendingTimers();
    vi.useRealTimers();
  });

  it("debounces typed input into the q URL param after 300ms", async () => {
    const router = renderSearchBox();

    act(() => {
      fireEvent.change(screen.getByPlaceholderText("Search by issue #…"), { target: { value: "42" } });
    });
    expect(router.state.location.search).not.toContain("q=42");

    await act(async () => {
      vi.advanceTimersByTime(300);
    });

    expect(router.state.location.search).toContain("q=42");
  });

  it("keeps an external URL change made mid-debounce instead of a stale snapshot clobbering it", async () => {
    const router = renderSearchBox();

    act(() => {
      fireEvent.change(screen.getByPlaceholderText("Search by issue #…"), { target: { value: "42" } });
    });

    // Before the 300ms debounce fires, an external change lands on the same
    // URL (e.g. a filter dropdown's own immediate, non-debounced update).
    act(() => {
      void router.navigate("/issues?bucket=violated");
    });

    await act(async () => {
      vi.advanceTimersByTime(300);
    });

    const search = router.state.location.search;
    expect(search).toContain("q=42");
    expect(search).toContain("bucket=violated");
  });

  it("clears q when the input is emptied", async () => {
    const router = renderSearchBox(["/issues?q=42"]);

    act(() => {
      fireEvent.change(screen.getByPlaceholderText("Search by issue #…"), { target: { value: "" } });
    });

    await act(async () => {
      vi.advanceTimersByTime(300);
    });

    expect(router.state.location.search).not.toContain("q=");
  });
});
