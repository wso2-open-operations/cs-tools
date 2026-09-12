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

import { act, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it } from "vitest";
import { useState, type JSX } from "react";
import "@testing-library/jest-dom/vitest";
import {
  CaseTabsProvider,
  useCaseTabsController,
} from "@context/case-tabs/CaseTabsContext";
import { CaseTabsBehaviorProvider } from "@context/case-tabs/CaseTabsBehaviorContext";
import { MAX_OPEN_CASE_TABS } from "@context/case-tabs/caseTabsTypes";

const STORAGE_KEY = "csm.caseTabs.v1";
const ENABLED_STORAGE_KEY = "csm.caseTabs.enabled";

function Probe(): JSX.Element {
  const { tabs, activeTabId, openTab, closeTab } = useCaseTabsController();
  return (
    <div>
      <div data-testid="tab-ids">{tabs.map((t) => t.id).join(",")}</div>
      <div data-testid="active">{activeTabId ?? ""}</div>
      <button onClick={() => openTab("CS1", "case", "/cases/CS1")}>open-cs1</button>
      <button onClick={() => openTab("CS2", "case", "/cases/CS2")}>open-cs2</button>
      <button onClick={() => closeTab(tabs[0]?.id ?? "")}>close-first</button>
    </div>
  );
}

function renderProbe(): ReturnType<typeof render> {
  return render(
    <CaseTabsBehaviorProvider>
      <CaseTabsProvider>
        <Probe />
      </CaseTabsProvider>
    </CaseTabsBehaviorProvider>,
  );
}

describe("CaseTabsProvider", () => {
  beforeEach(() => {
    sessionStorage.clear();
    // These tests exercise the tab MECHANISM itself, not the default
    // behavior mode (that's `CaseTabsBehaviorContext`'s own test file, and
    // the "default (mode 'on') behavior" describe block in
    // `CaseTabsIntegration.test.tsx`) — `enabled: false` (an explicit
    // opt-out) would make every `openTab` call here a no-op, so opt into it
    // being on explicitly. Cap-mode is left at its own default
    // (`evict-newest`).
    localStorage.setItem(ENABLED_STORAGE_KEY, "1");
  });

  it("starts with no tabs when sessionStorage is empty", () => {
    renderProbe();
    expect(screen.getByTestId("tab-ids")).toHaveTextContent("");
    expect(screen.getByTestId("active")).toHaveTextContent("");
  });

  it("opens tabs and tracks the active one", async () => {
    renderProbe();
    await act(async () => screen.getByText("open-cs1").click());
    expect(screen.getByTestId("tab-ids").textContent).toMatch(/^case-tab-/);
    await act(async () => screen.getByText("open-cs2").click());
    const ids = screen.getByTestId("tab-ids").textContent?.split(",") ?? [];
    expect(ids).toHaveLength(2);
    expect(screen.getByTestId("active").textContent).toBe(ids[1]);
  });

  it("persists the open tab set to sessionStorage, not localStorage", async () => {
    renderProbe();
    await act(async () => screen.getByText("open-cs1").click());
    const raw = sessionStorage.getItem(STORAGE_KEY);
    expect(raw).toBeTruthy();
    const persisted = JSON.parse(raw as string);
    expect(persisted.tabs).toHaveLength(1);
    expect(persisted.tabs[0].caseId).toBe("CS1");
    expect(localStorage.getItem(STORAGE_KEY)).toBeNull();
  });

  it("restores tabs from a prior session's sessionStorage on mount, reassigning fresh ids", () => {
    // Persisted shape is deliberately just `caseId` + `kind` (see
    // `CaseTabsPersistedState`'s doc comment) — no `id`/`path` to read back,
    // and the active tab is identified by `activeCaseId`, not the prior
    // session's internal id (which no longer exists once rehydrate assigns
    // every tab a fresh one).
    sessionStorage.setItem(
      STORAGE_KEY,
      JSON.stringify({
        tabs: [{ caseId: "CS9", kind: "case" }],
        activeCaseId: "CS9",
      }),
    );
    renderProbe();
    const ids = screen.getByTestId("tab-ids").textContent?.split(",") ?? [];
    expect(ids).toHaveLength(1);
    expect(ids[0]).toMatch(/^case-tab-/);
    expect(screen.getByTestId("active")).toHaveTextContent(ids[0]);
  });

  it("resolves the active tab by caseId, and reopens an incident/change-request tab as the right kind", () => {
    sessionStorage.setItem(
      STORAGE_KEY,
      JSON.stringify({
        tabs: [
          { caseId: "CS9", kind: "case" },
          { caseId: "INC1", kind: "incident" },
        ],
        activeCaseId: "INC1",
      }),
    );
    function KindProbe(): JSX.Element {
      const { tabs, activeTabId } = useCaseTabsController();
      const active = tabs.find((t) => t.id === activeTabId);
      return (
        <div>
          <div data-testid="active-case-id">{active?.caseId}</div>
          <div data-testid="active-kind">{active?.kind}</div>
          <div data-testid="active-path">{active?.path}</div>
        </div>
      );
    }
    render(
      <CaseTabsBehaviorProvider>
        <CaseTabsProvider>
          <KindProbe />
        </CaseTabsProvider>
      </CaseTabsBehaviorProvider>,
    );
    expect(screen.getByTestId("active-case-id")).toHaveTextContent("INC1");
    expect(screen.getByTestId("active-kind")).toHaveTextContent("incident");
    expect(screen.getByTestId("active-path")).toHaveTextContent(
      "/operations/incidents/INC1",
    );
  });

  // There is no longer a cap-behavior mode that refuses a new tab — both
  // `CaseTabsCapMode` values evict an existing tab to make room instead (see
  // that type's own doc comment). This test exercises the DEFAULT mode,
  // `evict-newest`: opening one more distinct case past the cap always
  // succeeds, closing the most-recently-opened tab to make room.
  it("opening a new tab past the cap evicts the newest tab (default mode) instead of refusing", async () => {
    function CapProbe(): JSX.Element {
      const { tabs, openTab } = useCaseTabsController();
      const [result, setResult] = useState<string>("");
      return (
        <div>
          <div data-testid="count">{tabs.length}</div>
          <div data-testid="case-ids">{tabs.map((t) => t.caseId).join(",")}</div>
          <div data-testid="overflow-outcome">{result}</div>
          <button
            onClick={() => {
              for (let i = 0; i < MAX_OPEN_CASE_TABS; i++) {
                openTab(`CS${i}`, "case", `/cases/CS${i}`);
              }
            }}
          >
            fill
          </button>
          <button
            onClick={() => {
              const ok = openTab("CS-overflow", "case", "/cases/CS-overflow");
              setResult(ok ? "opened" : "blocked");
            }}
          >
            overflow
          </button>
        </div>
      );
    }
    render(
      <CaseTabsBehaviorProvider>
        <CaseTabsProvider>
          <CapProbe />
        </CaseTabsProvider>
      </CaseTabsBehaviorProvider>,
    );
    await act(async () => screen.getByText("fill").click());
    expect(screen.getByTestId("count")).toHaveTextContent(String(MAX_OPEN_CASE_TABS));
    await act(async () => screen.getByText("overflow").click());
    expect(screen.getByTestId("overflow-outcome")).toHaveTextContent("opened");
    // Still exactly at the cap — the most-recently-opened tab (CS9, the last
    // of the `fill` loop) was evicted to make room, not the new one refused.
    expect(screen.getByTestId("count")).toHaveTextContent(String(MAX_OPEN_CASE_TABS));
    const caseIds = screen.getByTestId("case-ids").textContent?.split(",") ?? [];
    expect(caseIds).toContain("CS-overflow");
    expect(caseIds).not.toContain("CS9");
    expect(caseIds).toContain("CS0");
  });
});
