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
import { beforeEach, describe, expect, it } from "vitest";
import type { JSX } from "react";
import "@testing-library/jest-dom/vitest";
import {
  CaseTabsBehaviorProvider,
  useCaseTabsBehavior,
} from "@context/case-tabs/CaseTabsBehaviorContext";

const ENABLED_STORAGE_KEY = "csm.caseTabs.enabled";
const CAP_MODE_STORAGE_KEY = "csm.caseTabs.capMode";
const LEGACY_STORAGE_KEY = "csm.caseTabs.behavior";

function Probe(): JSX.Element {
  const { enabled, setEnabled, capMode, setCapMode } = useCaseTabsBehavior();
  return (
    <div>
      <div data-testid="enabled">{String(enabled)}</div>
      <div data-testid="cap-mode">{capMode}</div>
      <button onClick={() => setEnabled(true)}>enable</button>
      <button onClick={() => setEnabled(false)}>disable</button>
      <button onClick={() => setCapMode("evict-oldest")}>set-evict-oldest</button>
      <button onClick={() => setCapMode("evict-newest")}>set-evict-newest</button>
    </div>
  );
}

function renderProbe(): ReturnType<typeof render> {
  return render(
    <CaseTabsBehaviorProvider>
      <Probe />
    </CaseTabsBehaviorProvider>,
  );
}

describe("CaseTabsBehaviorContext", () => {
  beforeEach(() => {
    localStorage.clear();
  });

  // Flipped from the feature's original "beta, off by default" launch
  // decision, per explicit user instruction — tabs now ship ON, with an
  // explicit opt-out (via `PreferencesDialog`) rather than an opt-in.
  it("defaults to enabled (tabs on) on a fresh session with nothing in localStorage", () => {
    renderProbe();
    expect(screen.getByTestId("enabled")).toHaveTextContent("true");
    expect(screen.getByTestId("cap-mode")).toHaveTextContent("evict-newest");
  });

  it("also defaults to enabled outside a provider (the no-op default context value)", () => {
    render(<Probe />);
    expect(screen.getByTestId("enabled")).toHaveTextContent("true");
  });

  it("persists a toggle change to localStorage and reflects it immediately", () => {
    renderProbe();
    fireEvent.click(screen.getByText("enable"));
    expect(screen.getByTestId("enabled")).toHaveTextContent("true");
    expect(localStorage.getItem(ENABLED_STORAGE_KEY)).toBe("1");
    fireEvent.click(screen.getByText("disable"));
    expect(screen.getByTestId("enabled")).toHaveTextContent("false");
    expect(localStorage.getItem(ENABLED_STORAGE_KEY)).toBe("0");
  });

  it("persists a cap-mode change to localStorage and reflects it immediately", () => {
    renderProbe();
    fireEvent.click(screen.getByText("set-evict-oldest"));
    expect(screen.getByTestId("cap-mode")).toHaveTextContent("evict-oldest");
    expect(localStorage.getItem(CAP_MODE_STORAGE_KEY)).toBe("evict-oldest");
  });

  it("restores previously-saved preferences on mount", () => {
    localStorage.setItem(ENABLED_STORAGE_KEY, "1");
    localStorage.setItem(CAP_MODE_STORAGE_KEY, "evict-newest");
    renderProbe();
    expect(screen.getByTestId("enabled")).toHaveTextContent("true");
    expect(screen.getByTestId("cap-mode")).toHaveTextContent("evict-newest");
  });

  it("falls back to the default cap mode for a garbage/unrecognized stored value", () => {
    localStorage.setItem(CAP_MODE_STORAGE_KEY, "not-a-real-mode");
    renderProbe();
    expect(screen.getByTestId("cap-mode")).toHaveTextContent("evict-newest");
  });

  it("falls back to the default cap mode for the removed legacy 'block' value", () => {
    // "block" was a valid `CaseTabsCapMode` value before it was removed
    // entirely (there is no longer a "refuse the new tab" mode — see that
    // type's own doc comment) — a stored "block" from before that change
    // must not crash or silently stick around; it's just another
    // unrecognized value now, same as `not-a-real-mode` above.
    localStorage.setItem(CAP_MODE_STORAGE_KEY, "block");
    renderProbe();
    expect(screen.getByTestId("cap-mode")).toHaveTextContent("evict-newest");
  });

  describe("migration from the earlier single 4-value setting", () => {
    it("derives enabled=true and the cap mode from a legacy non-off value", () => {
      localStorage.setItem(LEGACY_STORAGE_KEY, "evict-oldest");
      renderProbe();
      expect(screen.getByTestId("enabled")).toHaveTextContent("true");
      expect(screen.getByTestId("cap-mode")).toHaveTextContent("evict-oldest");
    });

    it("derives enabled=false from a legacy 'off' value", () => {
      localStorage.setItem(LEGACY_STORAGE_KEY, "off");
      renderProbe();
      expect(screen.getByTestId("enabled")).toHaveTextContent("false");
    });

    it("prefers the new keys over the legacy one once both are present", () => {
      localStorage.setItem(LEGACY_STORAGE_KEY, "off");
      localStorage.setItem(ENABLED_STORAGE_KEY, "1");
      renderProbe();
      expect(screen.getByTestId("enabled")).toHaveTextContent("true");
    });
  });
});
