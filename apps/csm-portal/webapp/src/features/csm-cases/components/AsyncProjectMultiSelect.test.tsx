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
import { useState, type JSX } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import AsyncProjectMultiSelect from "@features/csm-cases/components/AsyncProjectMultiSelect";
import { useInfiniteProjectSearch } from "@features/csm-cases/api/useProjectSearch";

vi.mock("@features/csm-cases/api/useProjectSearch", () => ({
  useInfiniteProjectSearch: vi.fn(),
}));

const mockedUseInfiniteProjectSearch = vi.mocked(useInfiniteProjectSearch);

afterEach(() => {
  mockedUseInfiniteProjectSearch.mockReset();
});

const P1 = { id: "p1", name: "Customer 3 Project - Managed Cloud Subscription" };
const P2 = { id: "p2", name: "CP Ph2 Test Project - Managed Cloud Subscription" };

/** `mockImplementation`, not `mockReturnValue` — returns a fresh
 * array/object on every call, like the real `useInfiniteProjectSearch`
 * (backed by TanStack Query's own `.flatMap()`) does on every render, even
 * when nothing meaningful actually changed. Needed to catch the class of bug
 * below: a memo that (incorrectly) depends on this hook's own return-value
 * identity recomputes on every render, not only when something it actually
 * cares about changes. */
function mockResults(projects: Array<{ id: string; name: string }>): void {
  mockedUseInfiniteProjectSearch.mockImplementation(() => ({
    projects: projects.map((p) => ({ ...p })),
    isFetching: false,
    isFetchingNextPage: false,
    hasNextPage: false,
    isError: false,
    fetchNextPage: vi.fn(),
  }));
}

/** Mirrors how a real caller wires this up (CasesFilterBar, etc.) — a plain
 * values/onChange prop pair only re-renders with the new selection if
 * something actually holds state and feeds it back, which a bare `vi.fn()`
 * onChange does not. */
function Harness({ initial = [] as string[] }): JSX.Element {
  const [values, setValues] = useState<string[]>(initial);
  return <AsyncProjectMultiSelect values={values} onChange={setValues} />;
}

describe("AsyncProjectMultiSelect", () => {
  it("clears the typed search text once an option is picked, like a plain uncontrolled search box", () => {
    mockResults([P1, P2]);
    const onChange = vi.fn();
    render(<AsyncProjectMultiSelect values={[]} onChange={onChange} />);

    const input = screen.getByRole("combobox") as HTMLInputElement;
    fireEvent.focus(input);
    fireEvent.mouseDown(input);
    fireEvent.change(input, { target: { value: "managed" } });
    expect(input.value).toBe("managed");

    fireEvent.click(screen.getByText(P2.name));

    expect(onChange).toHaveBeenCalledWith(["p2"]);
    expect(input.value).toBe("");
  });

  it("shows the selected-names summary immediately after picking, without closing the dropdown first", () => {
    mockResults([P1, P2]);
    render(<Harness />);

    const input = screen.getByRole("combobox");
    fireEvent.focus(input);
    fireEvent.mouseDown(input);
    fireEvent.click(screen.getByText(P2.name));

    // The dropdown is still open (disableCloseOnSelect) and the summary
    // already reflects the pick, not only after the user clicks away.
    expect(screen.getByRole("listbox")).toBeInTheDocument();
    expect(screen.getAllByText(P2.name)).toHaveLength(2); // the summary + the still-open list row
  });

  it("hides the summary while typing a second search, instead of colliding with it, then shows both once picked", () => {
    // Regression test: found live after the previous commit made renderTags
    // unconditional again — picking a *first* project is fine (the box
    // empties right back out), but typing a *second* query while the first
    // project's summary was already showing collided on the same line,
    // exactly like the original bug this component was first fixed for.
    mockResults([P1, P2]);
    render(<Harness initial={["p1"]} />);

    expect(screen.getByText(P1.name)).toBeInTheDocument();

    const input = screen.getByRole("combobox") as HTMLInputElement;
    fireEvent.focus(input);
    fireEvent.mouseDown(input);
    fireEvent.change(input, { target: { value: "ph2" } });

    // While actively typing, the summary is hidden (nothing to collide with
    // the live query text) — p1's name still appears exactly once, as the
    // open dropdown's own (ticked) list row, not a second time as a summary
    // sharing the line with the live-typed query.
    expect(screen.getAllByText(P1.name)).toHaveLength(1);

    fireEvent.click(screen.getByText(P2.name));

    // Picking clears the box again, so the summary is back — now for both.
    expect(input.value).toBe("");
    expect(screen.getByText(`${P1.name}, ${P2.name}`)).toBeInTheDocument();
  });

  it("does not wipe the typed search text on every re-render, even with a project already selected", () => {
    // Regression test: MUI's Autocomplete resets its own (uncontrolled)
    // input text whenever its `value` prop gets a NEW reference while
    // focused (see useAutocomplete's own value-changed effect). This
    // component's `value` is `selectedOptions`, derived via useMemo from
    // `nameById`, which used to get a brand new Map identity on every
    // render purely because `projects` (this mock) returns a fresh
    // array/objects each call — found live as the typed search term getting
    // deleted while the user was still typing it, with a project already
    // selected. mockResults' own mockImplementation reproduces that
    // realistic non-referentially-stable hook output; selectedOptions must
    // stay referentially stable regardless, so MUI never sees its `value`
    // prop "change" when nothing the user picked actually did.
    mockResults([P1, P2]);
    render(<Harness initial={["p1"]} />);

    const input = screen.getByRole("combobox") as HTMLInputElement;
    fireEvent.focus(input);
    fireEvent.mouseDown(input);
    fireEvent.change(input, { target: { value: "m" } });
    fireEvent.change(input, { target: { value: "ma" } });
    fireEvent.change(input, { target: { value: "man" } });
    fireEvent.change(input, { target: { value: "managed" } });

    expect(input.value).toBe("managed");
  });

  it("offers the top real search match first, not the already-selected project pinned ahead of it", () => {
    // p2 is already selected, but the current search's own top match is p1 —
    // the dropdown's first row (what Enter/the default highlight would act
    // on) must be p1, not p2 forced to the front because it's selected.
    mockResults([P1, P2]);
    render(<AsyncProjectMultiSelect values={["p2"]} onChange={vi.fn()} />);

    fireEvent.mouseDown(screen.getByRole("combobox"));

    const options = screen.getAllByRole("option");
    expect(options[0]).toHaveTextContent(P1.name);
  });
});
