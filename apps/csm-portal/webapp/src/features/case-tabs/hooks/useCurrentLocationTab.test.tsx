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
import { describe, expect, it } from "vitest";
import type { JSX } from "react";
import "@testing-library/jest-dom/vitest";
import { MemoryRouter, Route, Routes } from "react-router";
import { useCurrentLocationTab } from "@features/case-tabs/hooks/useCurrentLocationTab";

function Probe(): JSX.Element {
  const tab = useCurrentLocationTab();
  return <div data-testid="path">{tab.path}</div>;
}

function renderAt(path: string) {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <Routes>
        <Route path="*" element={<Probe />} />
      </Routes>
    </MemoryRouter>,
  );
}

describe("useCurrentLocationTab", () => {
  // Regression test: the stored "last non-case location" only captured
  // pathname + search, dropping the `#hash` — a user who opened a case tab
  // from an anchor-linked page (e.g. `/help#topic`) lost the anchor when
  // clicking back to the pinned tab, landing on `/help` instead.
  it("preserves the hash fragment in the pinned tab's path", () => {
    renderAt("/help#topic");
    expect(screen.getByTestId("path")).toHaveTextContent("/help#topic");
  });

  it("preserves pathname, search, AND hash together", () => {
    renderAt("/help?q=foo#topic");
    expect(screen.getByTestId("path")).toHaveTextContent("/help?q=foo#topic");
  });

  it("has no hash for the dashboard fallback when starting on a case route", () => {
    render(
      <MemoryRouter initialEntries={["/cases/CS0001"]}>
        <Routes>
          <Route path="*" element={<Probe />} />
        </Routes>
      </MemoryRouter>,
    );
    expect(screen.getByTestId("path")).toHaveTextContent("/dashboard");
    expect(screen.getByTestId("path").textContent).not.toContain("#");
  });
});
