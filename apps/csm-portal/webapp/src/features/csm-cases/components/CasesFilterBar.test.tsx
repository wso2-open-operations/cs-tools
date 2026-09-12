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
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import type { ReactNode } from "react";
import CasesFilterBar, {
  type CasesFilters,
} from "@features/csm-cases/components/CasesFilterBar";
import { DEFAULT_CASES_FILTERS } from "@features/csm-cases/utils/casesFiltersUrl";
import { useSearchTags } from "@features/csm-cases/api/useSearchTags";

const postMock = vi.fn();

vi.mock("@api/backend/client", () => ({
  useBackendApi: () => ({ post: postMock, get: vi.fn() }),
}));

vi.mock("@config/apiConfig", () => ({
  apiConfig: { backendUrl: "https://example.test" },
}));

// Mocked directly (same approach as AddTagDialog.test.tsx) so tests don't
// have to drive the real 300ms debounce in `AsyncTagMultiSelect` (rendered
// only for an Advanced-mode `tag` row now — Tags is Advanced-only, see the
// mode-toggle tests below).
vi.mock("@features/csm-cases/api/useSearchTags", () => ({
  useSearchTags: vi.fn(),
}));
const mockedUseSearchTags = vi.mocked(useSearchTags);
function mockTagSearchResult(
  overrides: Partial<ReturnType<typeof useSearchTags>>,
): void {
  mockedUseSearchTags.mockReturnValue({
    data: [],
    isFetching: false,
    isError: false,
    ...overrides,
  } as unknown as ReturnType<typeof useSearchTags>);
}
// A default mock return value, set once here at file scope, so any test that
// does land in Advanced mode with a `tag` row visible doesn't crash for want
// of a mock.
beforeEach(() => {
  mockTagSearchResult({});
});

function renderBar(
  filters: CasesFilters,
  onChange = vi.fn(),
  extraProps: Partial<Parameters<typeof CasesFilterBar>[0]> = {},
): {
  onChange: ReturnType<typeof vi.fn>;
  /** Re-renders the SAME `CasesFilterBar` instance with new `filters` --
   * unlike a fresh `renderBar` call, this does NOT remount the component, so
   * its `mode` state (initialized once at mount, see `CasesFilterBar.tsx`'s
   * own doc comment on that `useState`) persists across the update, exactly
   * like a real `onChange` from a bar control or an applied saved view. */
  rerenderWith: (nextFilters: CasesFilters) => void;
} {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  const bar = (f: CasesFilters): ReactNode => (
    <QueryClientProvider client={queryClient}>
      <CasesFilterBar
        filters={f}
        onChange={onChange}
        onReset={() => {}}
        isFiltersOpen
        onFiltersToggle={() => {}}
        availableAssigneeUsers={[]}
        availableProjects={[]}
        {...extraProps}
      />
    </QueryClientProvider>
  );
  const { rerender } = render(bar(filters) as ReactNode);
  return { onChange, rerenderWith: (nextFilters) => rerender(bar(nextFilters) as ReactNode) };
}

describe("CasesFilterBar — active-filter chips for URL-only fields", () => {
  beforeEach(() => {
    postMock.mockReset();
    postMock.mockResolvedValue({ teams: [] });
  });

  it("renders no chips when nothing is active", () => {
    renderBar({ ...DEFAULT_CASES_FILTERS });
    expect(screen.queryByText(/SLA/)).not.toBeInTheDocument();
    expect(screen.queryByText(/Escalat/)).not.toBeInTheDocument();
    expect(screen.queryByText(/CS team:/)).not.toBeInTheDocument();
    expect(screen.queryByText(/^Tag:/)).not.toBeInTheDocument();
  });

  // `slaElapsedPctGte`/`hasEscalation`/`createdOnGte` are all in
  // `isSimpleRepresentable`'s gating list, so a filters object with any of
  // them set mounts the bar directly into Advanced mode (see the mode
  // `useState` initializer in `CasesFilterBar.tsx`). In Advanced mode every
  // one of these fields is its own row in the unified builder
  // (`filtersToAdvancedRows`), so per `buildActiveFilterChips`'s doc
  // comment this function renders NO chips for them any more — the row is
  // the only visible/removable UI. This used to be a chip-based test (round
  // 5 predates the fix); see git history for the old assertions if the row
  // behavior below ever needs to be cross-checked against the previous
  // chip-based one.
  it("renders no chips for URL-only fields once they force Advanced mode -- the row is the only UI", () => {
    renderBar({
      ...DEFAULT_CASES_FILTERS,
      slaElapsedPctGte: 80,
      hasEscalation: true,
      createdOnGte: "2026-07-27",
    });

    // No chip renders for any of these three any more.
    expect(screen.queryByText("SLA ≥ 80%")).not.toBeInTheDocument();
    expect(screen.queryByText("Escalated")).not.toBeInTheDocument();
    expect(screen.queryByText(/Created after/)).not.toBeInTheDocument();

    // Each is its own row in the unified Advanced builder instead, in
    // catalogue order (`taskSLABusinessElapsedPercent` gte, then
    // `escalation` isNotEmpty, then `createdOn` gte — see
    // `advancedFilters.ts`'s `ADVANCED_FILTER_FIELDS`).
    const fieldSelects = screen.getAllByRole("combobox", { name: "Field" });
    expect(fieldSelects.map((el) => el.textContent)).toEqual([
      "SLA business-elapsed %",
      "Escalation",
      "Created on",
    ]);
  });

  it("removing the SLA row (its own 'Remove filter row' control) clears only slaElapsedPctGte", () => {
    const { onChange } = renderBar({
      ...DEFAULT_CASES_FILTERS,
      slaElapsedPctGte: 80,
      hasEscalation: true,
      createdOnGte: "2026-07-27",
    });

    // Catalogue order puts the SLA row first (see the test above).
    const removeButtons = screen.getAllByRole("button", { name: "Remove filter row" });
    fireEvent.click(removeButtons[0]);

    expect(onChange).toHaveBeenCalledWith(
      expect.objectContaining({
        slaElapsedPctGte: null,
        hasEscalation: true,
        createdOnGte: "2026-07-27",
      }),
    );
  });

  it("both SLA bounds render as their own rows and clear independently", () => {
    const { onChange } = renderBar({
      ...DEFAULT_CASES_FILTERS,
      slaElapsedPctGte: 80,
      slaElapsedPctLte: 100,
    });

    // No chips for either bound.
    expect(screen.queryByText("SLA ≥ 80%")).not.toBeInTheDocument();
    expect(screen.queryByText("SLA ≤ 100%")).not.toBeInTheDocument();

    // Two rows: gte (catalogue order puts it before lte).
    const removeButtons = screen.getAllByRole("button", { name: "Remove filter row" });
    expect(removeButtons).toHaveLength(2);
    fireEvent.click(removeButtons[1]);

    expect(onChange).toHaveBeenCalledWith(
      expect.objectContaining({ slaElapsedPctGte: 80, slaElapsedPctLte: null }),
    );
  });
});

describe("CasesFilterBar — Simple mode keeps its existing chip behavior unchanged", () => {
  beforeEach(() => {
    postMock.mockReset();
    postMock.mockResolvedValue({ teams: [] });
  });

  // `workStates` is deliberately NOT in `isSimpleRepresentable`'s gating
  // list (see that function's own doc comment), so a filters object with
  // only `states`/`workStates` set still mounts into Simple mode, where
  // there is no unified row list -- the chip stays the only way to see/
  // clear a value that arrived via a dashboard click-through or a saved
  // view while looking at the Simple grid.
  it("still chips workStates in Simple mode, with the row list nowhere in sight", () => {
    const { onChange } = renderBar({
      ...DEFAULT_CASES_FILTERS,
      states: ["work_in_progress"],
      workStates: ["ongoing"],
    });

    expect(screen.queryByText("Advanced filters")).not.toBeInTheDocument();
    const chip = screen.getByText("Work state: Ongoing");
    expect(chip).toBeInTheDocument();

    fireEvent.click(chip.closest(".MuiChip-root")!.querySelector("svg")!);
    expect(onChange).toHaveBeenCalledWith(expect.objectContaining({ workStates: [] }));
  });

  // Originally this test asserted that a date-range bound arriving while
  // already in Simple mode (e.g. an applied saved view's `onChange`, which
  // updates `filters` without remounting `CasesFilterBar`) stayed on a chip,
  // because `mode` state -- set once at mount -- doesn't re-derive on its
  // own. CodeRabbit correctly flagged that as a real bug: it left an active
  // filter with no visible control at all once Advanced-mode chips were
  // suppressed for it (see `activeFilterChips`'s `effectiveMode` gating).
  // `effectiveMode` now recomputes every render, so this same sequence must
  // switch into Advanced mode and show the row instead.
  it("switches into Advanced mode when a date-range bound arrives mid-session in Simple mode, rather than leaving it on a stranded chip", () => {
    const { rerenderWith } = renderBar({ ...DEFAULT_CASES_FILTERS });
    expect(screen.queryByText("Advanced filters")).not.toBeInTheDocument();

    rerenderWith({ ...DEFAULT_CASES_FILTERS, createdOnGte: "2026-07-27" });

    // `effectiveMode` flips Simple -> Advanced now that the filters are no
    // longer Simple-representable, even though `mode` state itself never
    // changed -- the row list is what's visible, not a chip.
    expect(screen.getByText("Advanced filters")).toBeInTheDocument();
    const expected = new Date(2026, 6, 27).toLocaleDateString();
    expect(screen.queryByText(`Created after ${expected}`)).not.toBeInTheDocument();
  });
});


describe("CasesFilterBar — chips stay visible while the panel is collapsed", () => {
  beforeEach(() => {
    postMock.mockReset();
    postMock.mockResolvedValue({ teams: [] });
  });

  // CodeRabbit finding on PR #1630: `activeFilterChips` renders outside the
  // `isFiltersOpen` gate by design (chips are the only summary a collapsed
  // panel has), but gating the memo on `effectiveMode === "simple"` alone
  // silently emptied it whenever the panel was collapsed while in Advanced
  // mode -- neither the Simple grid nor the Advanced builder renders when
  // collapsed, so an Advanced-only value (e.g. from an applied saved view)
  // was left with no visible summary at all.
  it("still shows a chip for an Advanced-only value when the panel is collapsed", () => {
    renderBar(
      { ...DEFAULT_CASES_FILTERS, escalationLevels: ["2"] },
      undefined,
      { isFiltersOpen: false },
    );
    expect(screen.getByText("Escalation level: 2")).toBeInTheDocument();
  });
});

describe("CasesFilterBar — removed bar controls fall back to chips", () => {
  beforeEach(() => {
    postMock.mockReset();
  });


  /**
   * `tags`/`excludeTags` are Advanced-mode-only now (see the mode toggle in
   * `CasesFilterBar.tsx`): any non-empty value forces Advanced mode on
   * mount, where the Tag row itself (in the unified builder) is the visible/
   * removable UI — deliberately still NOT chipped here, to avoid showing the
   * same selection twice.
   */
  it("does not render a chip for tags — Advanced mode's own Tag row is the visible/removable UI", () => {
    renderBar({ ...DEFAULT_CASES_FILTERS, tags: ["micro-gw"] });
    expect(screen.queryByText(/^Tag: /)).not.toBeInTheDocument();
  });

  it("does not render a chip for excludeTags — Advanced mode's own Tag row is the visible/removable UI", () => {
    renderBar({ ...DEFAULT_CASES_FILTERS, excludeTags: ["s_dip"] });
    expect(screen.queryByText(/^Excluding tag:/)).not.toBeInTheDocument();
  });

  it("does not render a chip for csTeams — it has its own 'Team' bar control", () => {
    renderBar({ ...DEFAULT_CASES_FILTERS, csTeams: ["g1"] });
    expect(screen.queryByText(/^CS team:/)).not.toBeInTheDocument();
  });

  // The State field's tri-state include/exclude toggle was removed — Simple
  // mode's "State" control is now a plain include-only multi-select, so
  // `excludeStates` has no bar control of its own any more. Any active
  // `excludeStates` value also forces Advanced mode on mount (see
  // `isSimpleRepresentable`), where the "State"/"is not one of" row is the
  // primary way to see/edit it while the panel is open — same reasoning as
  // `sreTeams`/`workStates` above, a chip is still the only summary while the
  // panel is collapsed (`activeFilterChips`'s own `effectiveMode`/
  // `isFiltersOpen` gating), so it's asserted here with the panel collapsed.
  it("renders a removable chip for excludeStates now that the tri-state 'State' control is gone", () => {
    const { onChange } = renderBar(
      { ...DEFAULT_CASES_FILTERS, states: ["open"], excludeStates: ["closed"] },
      undefined,
      { isFiltersOpen: false },
    );
    const chip = screen.getByText("Excluding state: Closed");
    expect(chip).toBeInTheDocument();

    fireEvent.click(chip.closest(".MuiChip-root")!.querySelector("svg")!);
    expect(onChange).toHaveBeenCalledWith(expect.objectContaining({ excludeStates: [] }));
  });

  it("does not render a chip for onboardingStatuses — it has its own 'Onboarding status' bar control", () => {
    renderBar({ ...DEFAULT_CASES_FILTERS, onboardingStatuses: ["Completed"] });
    expect(screen.queryByText(/^Onboarding:/)).not.toBeInTheDocument();
  });
});

describe("CasesFilterBar — 'CRE Team' control (replaces the removed 'Work state' one)", () => {
  beforeEach(() => {
    postMock.mockReset();
    postMock.mockResolvedValue({
      teams: [
        { id: "abt-1", name: "ABT One", family: "cre-abt", creGroupId: "g-1" },
        { id: "abt-2", name: "ABT Two", family: "cre-abt", creGroupId: "g-2" },
        // No creGroupId configured -- must not appear as a selectable option.
        { id: "abt-3", name: "ABT Three", family: "cre-abt" },
        // Has a creGroupId but a non-`cre-abt` family -- must not appear either.
        { id: "abt-4", name: "ABT Four", family: "cre", creGroupId: "g-4" },
      ],
    });
  });

  it("renders team display names as options, backed by creGroupId (what the filter actually matches on)", async () => {
    renderBar({ ...DEFAULT_CASES_FILTERS });

    fireEvent.mouseDown(screen.getByRole("combobox", { name: "CRE Team" }));
    expect(await screen.findByRole("option", { name: "ABT One" })).toBeInTheDocument();
    expect(screen.getByRole("option", { name: "ABT Two" })).toBeInTheDocument();
    expect(screen.queryByRole("option", { name: "ABT Three" })).not.toBeInTheDocument();
    expect(screen.queryByRole("option", { name: "ABT Four" })).not.toBeInTheDocument();
  });

  it("selecting a team sets csTeams to its creGroupId, not its registry id", async () => {
    const { onChange } = renderBar({ ...DEFAULT_CASES_FILTERS });

    fireEvent.mouseDown(screen.getByRole("combobox", { name: "CRE Team" }));
    fireEvent.click(await screen.findByRole("option", { name: "ABT One" }));

    expect(onChange).toHaveBeenCalledWith(expect.objectContaining({ csTeams: ["g-1"] }));
  });
});

// The tri-state include/exclude toggle (digiops-cs#2907) was removed —
// Simple mode's "State" control is now a plain include-only multi-select,
// same as every other Simple-mode field (Severity, Case type, ...).
// Exclusion (`excludeStates`) is still expressible, just Advanced-mode-only
// now (the "State"/"is not one of" row) — see `filterFieldAdapters.test.ts`.
describe("CasesFilterBar — 'State' control (plain include-only multi-select)", () => {
  it("selecting a state adds it to states, leaving excludeStates untouched", async () => {
    const { onChange } = renderBar({ ...DEFAULT_CASES_FILTERS });

    fireEvent.mouseDown(screen.getByRole("combobox", { name: "State" }));
    fireEvent.click(await screen.findByRole("option", { name: "Closed" }));

    expect(onChange).toHaveBeenCalledWith(
      expect.objectContaining({ states: ["closed"], excludeStates: [] }),
    );
  });

  it("deselecting an already-selected state removes it from states", async () => {
    const { onChange } = renderBar({ ...DEFAULT_CASES_FILTERS, states: ["closed"] });

    fireEvent.mouseDown(screen.getByRole("combobox", { name: "State" }));
    fireEvent.click(await screen.findByRole("option", { name: "Closed" }));

    expect(onChange).toHaveBeenCalledWith(
      expect.objectContaining({ states: [], excludeStates: [] }),
    );
  });

  it("has no exclude affordance any more — only a checkbox per option, no +/- icon", async () => {
    renderBar({ ...DEFAULT_CASES_FILTERS });

    fireEvent.mouseDown(screen.getByRole("combobox", { name: "State" }));
    const option = await screen.findByRole("option", { name: "Closed" });
    expect(option.querySelector('input[type="checkbox"]')).toBeInTheDocument();
  });
});

// Tags moved out of Simple mode entirely (Advanced-only now, see
// `CasesFilterBar.tsx`'s mode toggle) -- the old tri-state cycling control
// (`TagsMultiSelect`) tested here is no longer rendered in the Simple grid
// at all. Its replacement — a plain `tag` `in`/`notIn` row in the unified
// Advanced-mode builder, backed by `AsyncTagMultiSelect` — is covered by
// `filterFieldAdapters.test.ts`'s adapter round-trip tests and
// `advancedFilters.test.ts`'s catalogue tests instead.
describe("CasesFilterBar — Tags is Advanced-only", () => {
  it("does not render a 'Tags' control in the Simple grid", () => {
    renderBar({ ...DEFAULT_CASES_FILTERS });
    expect(screen.queryByRole("combobox", { name: "Tags" })).not.toBeInTheDocument();
  });

  it("any active tags/excludeTags filter forces the bar into Advanced mode on mount", () => {
    renderBar({ ...DEFAULT_CASES_FILTERS, tags: ["micro-gw"] });
    // In Advanced mode, the Simple-only "State" combobox is gone and the
    // "Advanced filters" row builder is showing instead.
    expect(screen.queryByRole("combobox", { name: "State" })).not.toBeInTheDocument();
    expect(screen.getByText("Advanced filters")).toBeInTheDocument();
  });
});

// Regression: "Quick filters" used to be disabled outright whenever the
// active filter had anything Advanced-only in it (e.g. tags), with no way to
// even look at the Quick filters grid without first clearing that filter by
// hand. It's clickable unconditionally now — clicking it while not
// Simple-representable clears the active filter (Quick filters must never
// silently keep something active it can't display) but stashes the previous
// value locally so an accidental click doesn't lose real filter criteria:
// clicking straight back to Advanced restores it, as long as nothing was
// changed in Quick filters in between.
describe("CasesFilterBar — switching to Quick filters from an Advanced-only filter", () => {
  it("is not disabled, and clears the active filter (preserving search) when clicked", () => {
    const { onChange } = renderBar({
      ...DEFAULT_CASES_FILTERS,
      tags: ["micro-gw"],
      search: "printer",
    });

    const quickFiltersButton = screen.getByRole("button", { name: "Quick filters" });
    expect(quickFiltersButton).not.toBeDisabled();

    fireEvent.click(quickFiltersButton);

    expect(onChange).toHaveBeenCalledWith({ ...DEFAULT_CASES_FILTERS, search: "printer" });
  });

  it("restores the stashed Advanced filter when Advanced is clicked right back, without needing another edit", () => {
    const original = { ...DEFAULT_CASES_FILTERS, tags: ["micro-gw"] };
    const { onChange, rerenderWith } = renderBar(original);

    fireEvent.click(screen.getByRole("button", { name: "Quick filters" }));
    const cleared = onChange.mock.calls[0][0] as CasesFilters;
    rerenderWith(cleared);

    fireEvent.click(screen.getByRole("button", { name: "Advanced" }));
    expect(onChange).toHaveBeenLastCalledWith(original);
  });

  it("does not restore the stash once a real edit is made in Quick filters — the fresh edit wins", () => {
    const original = { ...DEFAULT_CASES_FILTERS, tags: ["micro-gw"] };
    const { onChange, rerenderWith } = renderBar(original);

    fireEvent.click(screen.getByRole("button", { name: "Quick filters" }));
    const cleared = onChange.mock.calls[0][0] as CasesFilters;
    rerenderWith(cleared);

    // A real edit in Quick filters — e.g. picking a severity. It's a
    // multi-select, so choosing an option doesn't auto-close the menu;
    // close it explicitly (MUI's Modal makes the rest of the page
    // aria-hidden while open, hiding the mode-toggle buttons from
    // getByRole until it does).
    fireEvent.mouseDown(screen.getByRole("combobox", { name: "Severity" }));
    fireEvent.click(screen.getByRole("option", { name: "S0" }));
    fireEvent.keyDown(screen.getByRole("listbox"), { key: "Escape" });
    const edited = onChange.mock.calls[1][0] as CasesFilters;
    expect(edited.tags).toEqual([]); // still not the stashed tags filter
    rerenderWith(edited);

    fireEvent.click(screen.getByRole("button", { name: "Advanced" }));
    // The stash was dropped by the edit above — Advanced just shows the
    // caller's current (edited) filters, not a resurrected old tags filter.
    expect(onChange).not.toHaveBeenCalledWith(original);
  });

  // CodeRabbit catch on the PR that introduced the stash: applyView and the
  // reset button both replace `filters` via a path that bypasses
  // `handleSimpleFieldChange` (a saved view calls `onChange` directly; reset
  // calls the `onReset` prop) — neither used to clear a pending stash, so
  // clicking Advanced right after either one could resurrect the
  // pre-Quick-filters criteria over what the user just deliberately applied
  // or cleared.
  it("applying a saved view drops the stash — Advanced afterward shows the view, not resurrected tags", () => {
    localStorage.clear();
    localStorage.setItem(
      "csm.savedFilters.v1",
      JSON.stringify([{ name: "First", qs: "states=open" }]),
    );
    const original = { ...DEFAULT_CASES_FILTERS, tags: ["micro-gw"] };
    const { onChange, rerenderWith } = renderBar(original);

    fireEvent.click(screen.getByRole("button", { name: "Quick filters" }));
    rerenderWith(onChange.mock.calls[0][0] as CasesFilters);

    fireEvent.click(screen.getByRole("button", { name: /Saved views/ }));
    fireEvent.click(screen.getByText("First"));
    const viewApplied = onChange.mock.calls[1][0] as CasesFilters;
    expect(viewApplied.states).toEqual(["open"]);
    rerenderWith(viewApplied);

    fireEvent.click(screen.getByRole("button", { name: "Advanced" }));
    expect(onChange).not.toHaveBeenCalledWith(original);
  });

  it("resetting drops the stash — Advanced afterward does not resurrect the cleared tags filter", () => {
    // search stays active (and preserved) through the Quick-filters switch,
    // so "Clear filters" is still the button shown — this is the realistic
    // way to reach a reset with a stash still pending, since any actual
    // edit in the Quick filters grid itself already drops the stash on its
    // own (the case above).
    const original = { ...DEFAULT_CASES_FILTERS, tags: ["micro-gw"], search: "printer" };
    const { onChange, rerenderWith } = renderBar(original);

    fireEvent.click(screen.getByRole("button", { name: "Quick filters" }));
    rerenderWith(onChange.mock.calls[0][0] as CasesFilters);

    fireEvent.click(screen.getByRole("button", { name: /Clear filters/ }));

    fireEvent.click(screen.getByRole("button", { name: "Advanced" }));
    expect(onChange).not.toHaveBeenCalledWith(original);
  });
});

// Regression: reported live — with every control the same width, a project
// name of any real length ellipsized almost immediately, and the control sat
// mid-row rather than having room to grow. Moved to the end of the grid and
// widened.
describe("CasesFilterBar — 'Project' control is last and wider than its siblings", () => {
  it("renders after every other filter control in document order", () => {
    renderBar({ ...DEFAULT_CASES_FILTERS });

    const project = screen.getByRole("combobox", { name: "Project" });
    const onboarding = screen.getByRole("combobox", { name: "Onboarding status" });

    expect(
      project.compareDocumentPosition(onboarding) & Node.DOCUMENT_POSITION_PRECEDING,
    ).toBeTruthy();
  });
});

describe("CasesFilterBar — 'Onboarding status' control", () => {
  beforeEach(() => {
    postMock.mockReset();
  });

  it("offers all 4 fixed projectOnboardingStatus choices", () => {
    renderBar({ ...DEFAULT_CASES_FILTERS });

    fireEvent.mouseDown(screen.getByRole("combobox", { name: "Onboarding status" }));
    expect(screen.getByRole("option", { name: "In progress" })).toBeInTheDocument();
    expect(screen.getByRole("option", { name: "Not started" })).toBeInTheDocument();
    expect(screen.getByRole("option", { name: "Completed" })).toBeInTheDocument();
    expect(screen.getByRole("option", { name: "Not applicable" })).toBeInTheDocument();
  });

  // The control edits the real `onboardingStatuses` (`in`) field directly —
  // there is no separate exclude field/URL param for this control to write
  // to. A dashboard widget's `projectOnboardingStatus notIn` filter is
  // folded into this same field as its complement at the translation
  // boundary (`translateCaseDashboardFilters`), specifically so this bar
  // control's URL param never collides with a second, exclude-flavored one.
  it("selecting a value sets onboardingStatuses to its raw backend token", () => {
    const { onChange } = renderBar({ ...DEFAULT_CASES_FILTERS });

    fireEvent.mouseDown(screen.getByRole("combobox", { name: "Onboarding status" }));
    fireEvent.click(screen.getByRole("option", { name: "In progress" }));

    expect(onChange).toHaveBeenCalledWith(
      expect.objectContaining({ onboardingStatuses: ["In-Progress"] }),
    );
  });

  it("adds to any value already set rather than replacing it", () => {
    const { onChange } = renderBar({
      ...DEFAULT_CASES_FILTERS,
      onboardingStatuses: ["Not-Applicable"],
    });

    fireEvent.mouseDown(screen.getByRole("combobox", { name: "Onboarding status" }));
    fireEvent.click(screen.getByRole("option", { name: "In progress" }));

    expect(onChange).toHaveBeenCalledWith(
      expect.objectContaining({ onboardingStatuses: ["Not-Applicable", "In-Progress"] }),
    );
  });
});

describe("CasesFilterBar — work state has no bar control, only a chip", () => {
  beforeEach(() => {
    postMock.mockReset();
  });

  it("no longer renders a Work state bar control", () => {
    renderBar({ ...DEFAULT_CASES_FILTERS, states: ["work_in_progress"] });
    expect(screen.queryByRole("combobox", { name: "Work state" })).not.toBeInTheDocument();
  });

  it("renders a removable chip when workStates is set (e.g. from a saved view or dashboard click-through)", () => {
    const { onChange } = renderBar({
      ...DEFAULT_CASES_FILTERS,
      states: ["work_in_progress"],
      workStates: ["ongoing"],
    });

    expect(screen.getByText("Work state: Ongoing")).toBeInTheDocument();

    fireEvent.click(
      screen.getByText("Work state: Ongoing").closest(".MuiChip-root")!.querySelector("svg")!,
    );
    expect(onChange).toHaveBeenCalledWith(expect.objectContaining({ workStates: [] }));
  });

  // The underlying invariant (workStates only means something when
  // work_in_progress is the *sole* selected state) still matters even
  // without a bar control to disable -- a stale workStates value from a
  // saved view/URL must not survive the State control widening past
  // work_in_progress alone.
  it("clears workStates when a second state is added alongside work_in_progress", () => {
    const { onChange } = renderBar({
      ...DEFAULT_CASES_FILTERS,
      states: ["work_in_progress"],
      workStates: ["ongoing"],
    });

    fireEvent.mouseDown(screen.getByRole("combobox", { name: "State" }));
    fireEvent.click(screen.getByRole("option", { name: "Open" }));

    expect(onChange).toHaveBeenCalledWith(
      expect.objectContaining({
        states: ["work_in_progress", "open"],
        workStates: [],
      }),
    );
  });
});

describe("CasesFilterBar — saved views reordering", () => {
  beforeEach(() => {
    postMock.mockReset();
    localStorage.clear();
    localStorage.setItem(
      "csm.savedFilters.v1",
      JSON.stringify([
        { name: "First", qs: "states=open" },
        { name: "Second", qs: "states=closed" },
      ]),
    );
  });

  function openSavedViewsMenu(): void {
    fireEvent.click(screen.getByRole("button", { name: /Saved views/ }));
  }

  it("renders move up/down buttons for saved views; the Suggested section is gone", () => {
    renderBar({ ...DEFAULT_CASES_FILTERS });
    openSavedViewsMenu();

    // The built-in Suggested section has been removed entirely.
    expect(screen.queryByText("Suggested")).not.toBeInTheDocument();
    expect(screen.queryByText("S0/S1 active")).not.toBeInTheDocument();

    // Saved (user) views get reorder controls.
    expect(
      screen.getByRole("button", { name: "Move saved view First down" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Move saved view Second up" }),
    ).toBeInTheDocument();
  });

  it("disables (or omits an enabled) up-arrow on the first item", () => {
    renderBar({ ...DEFAULT_CASES_FILTERS });
    openSavedViewsMenu();

    expect(
      screen.getByRole("button", { name: "Move saved view First up" }),
    ).toBeDisabled();
    expect(
      screen.getByRole("button", { name: "Move saved view Second down" }),
    ).toBeDisabled();
  });

  it("clicking move-down on the first saved view reorders the list without applying it", () => {
    const { onChange } = renderBar({ ...DEFAULT_CASES_FILTERS });
    openSavedViewsMenu();

    fireEvent.click(
      screen.getByRole("button", { name: "Move saved view First down" }),
    );

    // Reordering must not also apply/select the view.
    expect(onChange).not.toHaveBeenCalled();

    // The persisted order flips, reflected back through the reactive hook —
    // "Second" now moves up-button-enabled into the first slot.
    expect(
      screen.getByRole("button", { name: "Move saved view Second up" }),
    ).toBeDisabled();
    expect(
      screen.getByRole("button", { name: "Move saved view First up" }),
    ).not.toBeDisabled();
  });
});

describe("CasesFilterBar — case-type control label", () => {
  beforeEach(() => {
    postMock.mockReset();
  });

  it("defaults the case-type control's label to \"Case type\"", () => {
    renderBar({ ...DEFAULT_CASES_FILTERS });
    expect(screen.getByLabelText("Case type")).toBeInTheDocument();
  });

  it("renders a caller-supplied typeFilterLabel instead (e.g. a project's mixed work-items view)", () => {
    renderBar({ ...DEFAULT_CASES_FILTERS }, vi.fn(), {
      typeFilterLabel: "Work item type",
    });
    expect(screen.getByLabelText("Work item type")).toBeInTheDocument();
    expect(screen.queryByLabelText("Case type")).not.toBeInTheDocument();
  });

  it("hides the control entirely when hideTypeFilter is set, regardless of typeFilterLabel", () => {
    renderBar({ ...DEFAULT_CASES_FILTERS }, vi.fn(), {
      hideTypeFilter: true,
      typeFilterLabel: "Work item type",
    });
    expect(screen.queryByLabelText("Work item type")).not.toBeInTheDocument();
    expect(screen.queryByLabelText("Case type")).not.toBeInTheDocument();
  });
});

describe("CasesFilterBar — search box placeholder", () => {
  beforeEach(() => {
    postMock.mockReset();
  });

  it("only claims to match case #, subject and internal ID — not customer/project/assignee", () => {
    renderBar({ ...DEFAULT_CASES_FILTERS });
    const search = screen.getByPlaceholderText(
      "Search by case #, subject or internal ID…",
    );
    expect(search).toBeInTheDocument();
    // Regression guard for the removed, inaccurate claim (`searchQuery` never
    // matched customer/project/assignee on the backend) — checked against
    // this one input's own placeholder, not the whole bar (the Project
    // picker below legitimately has its own, unrelated "Type a project…"
    // placeholder).
    expect(search).not.toHaveAttribute(
      "placeholder",
      "Search by case #, subject, customer, project, assignee…",
    );
  });
});

describe("CasesFilterBar — per-consumer hidden Simple-mode controls", () => {
  beforeEach(() => {
    postMock.mockReset();
    postMock.mockResolvedValue({ teams: [] });
  });

  it("shows Onboarding status and CRE Team by default", () => {
    renderBar({ ...DEFAULT_CASES_FILTERS });
    expect(
      screen.getByRole("combobox", { name: "Onboarding status" }),
    ).toBeInTheDocument();
    expect(screen.getByRole("combobox", { name: "CRE Team" })).toBeInTheDocument();
  });

  it("hides Onboarding status when hideOnboardingStatusFilter is set (e.g. a project-scoped view)", () => {
    renderBar({ ...DEFAULT_CASES_FILTERS }, vi.fn(), {
      hideOnboardingStatusFilter: true,
    });
    expect(
      screen.queryByRole("combobox", { name: "Onboarding status" }),
    ).not.toBeInTheDocument();
    expect(screen.getByRole("combobox", { name: "CRE Team" })).toBeInTheDocument();
  });

  it("hides CRE Team when hideCreTeamFilter is set (e.g. a project-scoped view)", () => {
    renderBar({ ...DEFAULT_CASES_FILTERS }, vi.fn(), {
      hideCreTeamFilter: true,
    });
    expect(
      screen.queryByRole("combobox", { name: "CRE Team" }),
    ).not.toBeInTheDocument();
    expect(
      screen.getByRole("combobox", { name: "Onboarding status" }),
    ).toBeInTheDocument();
  });

  it("hides both when both flags are set, without affecting the Severity control", () => {
    renderBar({ ...DEFAULT_CASES_FILTERS }, vi.fn(), {
      hideOnboardingStatusFilter: true,
      hideCreTeamFilter: true,
      showSeverityFilter: true,
    });
    expect(
      screen.queryByRole("combobox", { name: "Onboarding status" }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("combobox", { name: "CRE Team" }),
    ).not.toBeInTheDocument();
    expect(screen.getByRole("combobox", { name: "Severity" })).toBeInTheDocument();
  });
});
