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

import { render, screen, fireEvent } from "@testing-library/react";
import { createMemoryRouter, RouterProvider } from "react-router";
import { describe, expect, it } from "vitest";
import { useGlobalFilters } from "./filters";

// A minimal harness so a click can drive setFilter and the resulting URL can
// be read back off the router, mirroring IssuesPage.test.tsx's router setup.
function Harness() {
  const { setFilter } = useGlobalFilters();
  return (
    <button type="button" onClick={() => setFilter("repo", "org/alpha")}>
      set repo
    </button>
  );
}

function renderHarness(initialEntries: string[]) {
  const router = createMemoryRouter([{ path: "/issues", element: <Harness /> }], { initialEntries });
  render(<RouterProvider router={router} />);
  return router;
}

describe("useGlobalFilters", () => {
  it("setFilter deletes any existing page search param", () => {
    const router = renderHarness(["/issues?page=3&priority=P1"]);

    fireEvent.click(screen.getByText("set repo"));

    const search = router.state.location.search;
    expect(search).not.toContain("page=");
    expect(search).toContain("repo=org%2Falpha");
    expect(search).toContain("priority=P1");
  });
});
