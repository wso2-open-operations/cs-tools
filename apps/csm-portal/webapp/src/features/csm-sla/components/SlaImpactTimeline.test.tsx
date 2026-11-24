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

import { act, fireEvent, render, screen, within } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import { createTheme, ThemeProvider } from "@wso2/oxygen-ui";
import { resolveTheme, THEME_OPTIONS } from "@config/themeConfig";
import { resetFeatureStatesForTests } from "@config/featureFlags";
import { SlaImpactTimeline } from "@features/csm-sla/components/SlaImpactTimeline";
import type { CaseAuditEntry } from "@features/csm-cases/types/csmCases";

const ENTRIES: CaseAuditEntry[] = [
  {
    id: "a1",
    kind: "field_change",
    actor: "Jane Doe",
    createdAt: "2026-07-01T10:00:00Z",
    changes: [
      { field: "state", fieldLabel: "State", previousValue: "Work In Progress", newValue: "Awaiting Info" },
    ],
  },
  {
    id: "a2",
    kind: "field_change",
    actor: "Jane Doe",
    createdAt: "2026-07-02T10:00:00Z",
    changes: [{ field: "workaround_provided_on", fieldLabel: "Workaround Provided", newValue: "2026-07-02T10:00:00Z" }],
  },
];

function renderTimeline(overrides: Partial<Parameters<typeof SlaImpactTimeline>[0]> = {}) {
  return render(
    <SlaImpactTimeline entries={ENTRIES} isLoading={false} isError={false} onRetry={vi.fn()} {...overrides} />,
  );
}

function setOverrides(value: unknown): void {
  window.config = { CSM_PORTAL_FEATURE_OVERRIDES: value } as unknown as typeof window.config;
  resetFeatureStatesForTests();
}

afterEach(() => {
  setOverrides(undefined);
});

describe("SlaImpactTimeline", () => {
  it.each(THEME_OPTIONS.flatMap(({ key }) => ["light", "dark"].map((mode) => ({ key, mode: mode as "light" | "dark" }))))("keeps selection visible in $key / $mode", ({ key, mode }) => {
    const base = resolveTheme(key);
    const schemes = (base as unknown as { colorSchemes?: Partial<Record<"light" | "dark", { palette: typeof base.palette }>> }).colorSchemes;
    const palette = schemes?.[mode]?.palette ?? base.palette;
    const theme = createTheme({ palette: { ...palette, mode } });
    render(<ThemeProvider theme={theme}>
      <SlaImpactTimeline entries={ENTRIES} openedAt="2026-07-01T09:00:00Z" isLoading={false} isError={false} onRetry={vi.fn()} />
    </ThemeProvider>);
    const timeline = screen.getByRole("group", { name: "Ownership timeline" });
    const wso2 = within(timeline).getByRole("button", { name: /With Choreo Labs/ });
    fireEvent.click(wso2);
    expect(wso2).toHaveAttribute("aria-pressed", "true");
    expect(wso2).toHaveStyle({ color: theme.palette.info.contrastText, height: "24px", borderRadius: "999px" });
    expect(screen.getByText("Selected period")).toBeInTheDocument();
    expect(document.querySelector(".lucide-chevron-down")).not.toBeNull();
    const customer = within(timeline).getByRole("button", { name: /With customer/ });
    fireEvent.click(customer);
    expect(customer).toHaveStyle({ color: theme.palette.warning.contrastText, height: "24px", borderRadius: "999px" });
    expect(wso2).toHaveAttribute("aria-pressed", "false");
    expect(wso2).toHaveStyle({ height: "16px", borderRadius: "999px 0 0 999px" });
    expect(timeline).toHaveStyle({ height: "28px", overflow: "visible" });
    expect(customer).toHaveAccessibleDescription(/Selected period/);
  });

  it("renders one focusable marker per event, named with title, values, actor and time", () => {
    renderTimeline();
    const group = screen.getByRole("group", { name: "SLA events, oldest to newest" });
    const state = within(group).getByRole("button", { name: /Waiting on customer/ });
    expect(state).toHaveAccessibleName(/Jane Doe/);
    expect(state).toHaveAccessibleName(/2026/);
    expect(within(group).getByRole("button", { name: /Workaround provided/ })).toBeInTheDocument();
    state.focus();
    expect(state).toHaveFocus();
  });

  it("shows ownership summaries without a closed duration", () => {
    renderTimeline();
    for (const label of ["With Choreo Labs", "With customer"]) {
      expect(screen.getByText(label)).toBeInTheDocument();
    }
    expect(screen.queryByText("Closed")).not.toBeInTheDocument();
  });

  it("opens the audit history on marker click instead of relying on hover", () => {
    renderTimeline();
    const group = screen.getByRole("group", { name: "SLA events, oldest to newest" });
    fireEvent.click(within(group).getByRole("button", { name: /Waiting on customer/ }));
    const dialog = screen.getByRole("dialog", { name: "SLA event history" });
    expect(within(dialog).getAllByText("Jane Doe")[0]).toBeInTheDocument();
    expect(within(dialog).getByText(/State: Work in progress/)).toBeInTheDocument();
  });

  it("shows duration and exact times when an ownership period is clicked", () => {
    renderTimeline({ openedAt: "2026-07-01T09:00:00Z" });
    fireEvent.click(within(screen.getByRole("group", { name: "Ownership timeline" })).getByRole("button", { name: /With Choreo Labs, 1h elapsed/ }));
    expect(screen.getByText("With Choreo Labs · 1h elapsed")).toBeInTheDocument();
  });

  it("limits the main history to the latest three changes", () => {
    renderTimeline({ entries: Array.from({ length: 5 }, (_, i) => ({ ...ENTRIES[1], id: `a${i}`, createdAt: `2026-07-0${i + 1}T10:00:00Z` })) });
    expect(screen.getByText("Latest 3 of 5 · newest first")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "View all 5 events" }));
    expect(within(screen.getByRole("dialog")).getAllByText("Workaround provided")).toHaveLength(5);
  });

  it("opens the full history when a grouped marker is clicked", () => {
    const close = (i: number) => ({ ...ENTRIES[1], id: `g${i}`, createdAt: `2026-07-02T10:00:0${i}Z` });
    renderTimeline({ entries: [ENTRIES[0], close(1), close(2), close(3)] });
    const group = screen.getByRole("group", { name: "SLA events, oldest to newest" });
    fireEvent.click(within(group).getByRole("button", { name: /Workaround provided.*Workaround provided/ }));
    const dialog = screen.getByRole("dialog", { name: "SLA event history" });
    expect(within(dialog).getByText("4 events · newest first")).toBeInTheDocument();
  });

  it("picks up an event recorded after the timeline mounted", () => {
    vi.useFakeTimers();
    try {
      const mountedAt = new Date("2026-07-05T10:00:00Z");
      vi.setSystemTime(mountedAt);
      const later: CaseAuditEntry = { ...ENTRIES[1], id: "later", createdAt: "2026-07-05T10:00:10Z" };
      renderTimeline({ entries: [later], openedAt: "2026-07-01T09:00:00Z" });
      expect(screen.getByText(/No SLA-affecting changes/)).toBeInTheDocument();
      act(() => { vi.advanceTimersByTime(30_000); });
      expect(screen.getByText("Latest 1 of 1 · newest first")).toBeInTheDocument();
    } finally {
      vi.useRealTimers();
    }
  });

  it("hides the view-all button when every event is already shown", () => {
    renderTimeline({ entries: ENTRIES });
    expect(screen.getByText("Latest 2 of 2 · newest first")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /View all/ })).not.toBeInTheDocument();
  });

  it("shows breached SLA statuses without inventing times or placing them on the graph", () => {
    renderTimeline({ slas: [
      { id: "response", definition: "Response SLA", hasBreached: true },
      { id: "resolution", definition: "Resolution SLA", hasBreached: true },
    ] as never });
    const statuses = screen.getByLabelText("Breached SLAs");
    expect(within(statuses).getByText("Response SLA · Breached")).toBeInTheDocument();
    expect(within(statuses).getByText("Resolution SLA · Breached")).toBeInTheDocument();
    expect(screen.queryByRole("group", { name: "SLA breach markers" })).not.toBeInTheDocument();
  });

  it("shows a loading state", () => {
    renderTimeline({ isLoading: true, entries: undefined });
    expect(screen.getByText("SLA impact timeline")).toBeInTheDocument();
    expect(screen.queryByRole("group")).not.toBeInTheDocument();
  });

  it("shows an empty state when nothing SLA-relevant was recorded", () => {
    renderTimeline({ entries: [] });
    expect(screen.getByText(/No SLA-affecting changes/)).toBeInTheDocument();
  });

  it("shows an error state with a working retry", () => {
    const onRetry = vi.fn();
    renderTimeline({ isError: true, onRetry });
    expect(screen.getByText(/Could not load the SLA impact timeline/)).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Retry" }));
    expect(onRetry).toHaveBeenCalledTimes(1);
  });

  it("is shown by default and when the override says enabled", () => {
    const { unmount } = renderTimeline();
    expect(screen.getByText("SLA impact timeline")).toBeInTheDocument();
    unmount();
    setOverrides({ "support.case-sla-timeline": "enabled" });
    renderTimeline();
    expect(screen.getByText("SLA impact timeline")).toBeInTheDocument();
  });

  it("renders nothing when the feature is overridden to hidden", () => {
    setOverrides({ "support.case-sla-timeline": "hidden" });
    const { container } = renderTimeline();
    expect(container).toBeEmptyDOMElement();
  });

  it("also renders nothing for wip, and ignores unrelated overrides", () => {
    setOverrides({ "support.case-sla-timeline": "wip" });
    const { container, unmount } = renderTimeline();
    expect(container).toBeEmptyDOMElement();
    unmount();
    setOverrides({ operations: "hidden" });
    renderTimeline();
    expect(screen.getByText("SLA impact timeline")).toBeInTheDocument();
  });
});
