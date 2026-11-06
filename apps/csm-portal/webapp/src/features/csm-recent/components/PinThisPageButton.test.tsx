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
import { MemoryRouter } from "react-router";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { RecentView } from "@features/csm-recent/hooks/useRecentViews";
import { STANDARD_CHANGES_PATH } from "@features/csm-operations/utils/standardChanges";

const recents: { current: RecentView[] } = { current: [] };
const record = vi.fn();
const toggleRecentViewPin = vi.fn();

vi.mock("@asgardeo/react", () => ({ useAsgardeo: () => ({ isSignedIn: true }) }));
vi.mock("@features/csm-recent/hooks/useRecentViews", () => ({
  useRecentViews: () => recents.current,
  useRecordRecentView: () => record,
  toggleRecentViewPin: (...args: unknown[]) => toggleRecentViewPin(...args),
}));

import PinThisPageButton from "@features/csm-recent/components/PinThisPageButton";

function renderAt(url: string): void {
  render(
    <MemoryRouter initialEntries={[url]}>
      <PinThisPageButton />
    </MemoryRouter>,
  );
}

/** Standard changes as "Pin this page" pins it: without the change request list's `cr*` filters. */
const pinnedStandardChanges: RecentView = {
  kind: "page",
  id: STANDARD_CHANGES_PATH,
  title: "Standard changes",
  href: STANDARD_CHANGES_PATH,
  visitedAt: "2026-10-29T00:00:00Z",
  pinned: true,
};

describe("PinThisPageButton", () => {
  beforeEach(() => {
    recents.current = [];
    record.mockReset();
    toggleRecentViewPin.mockReset();
  });

  it("pins an unpinned page under the entry it derives", () => {
    renderAt(`${STANDARD_CHANGES_PATH}&crQ=router&crStates=closed`);
    fireEvent.click(screen.getByRole("button", { name: "Pin this page to top nav bar" }));
    expect(record).toHaveBeenCalledWith(
      expect.objectContaining({ title: "Standard changes", href: STANDARD_CHANGES_PATH }),
    );
    expect(toggleRecentViewPin).toHaveBeenCalledWith("page", STANDARD_CHANGES_PATH);
  });

  it("reads as pinned on Standard changes when its URL carries the list's filters the pin leaves out", () => {
    recents.current = [pinnedStandardChanges];
    renderAt(`${STANDARD_CHANGES_PATH}&crQ=router&crStates=closed`);
    const button = screen.getByRole("button", { name: "Unpin this page" });
    expect(button.getAttribute("aria-pressed")).toBe("true");
    // The click unpins that entry; nothing new is recorded.
    fireEvent.click(button);
    expect(toggleRecentViewPin).toHaveBeenCalledWith("page", STANDARD_CHANGES_PATH);
    expect(record).not.toHaveBeenCalled();
  });

  it("reads as pinned on the exact URL too", () => {
    recents.current = [pinnedStandardChanges];
    renderAt(STANDARD_CHANGES_PATH);
    expect(screen.getByRole("button", { name: "Unpin this page" })).toBeTruthy();
  });

  it("prefers an entry recorded for the exact URL", () => {
    const exact: RecentView = {
      kind: "page",
      id: "/operations/change-requests?crQ=router",
      title: "Change requests: 1 filter",
      href: "/operations/change-requests?crQ=router",
      visitedAt: "2026-10-29T00:01:00Z",
      pinned: false,
    };
    recents.current = [exact];
    renderAt("/operations/change-requests?crQ=router");
    fireEvent.click(screen.getByRole("button", { name: "Pin this page to top nav bar" }));
    expect(toggleRecentViewPin).toHaveBeenCalledWith("page", exact.id);
    expect(record).not.toHaveBeenCalled();
  });

  it("does not take another place's pin for this one", () => {
    recents.current = [pinnedStandardChanges];
    renderAt("/operations/change-requests?view=standard&show=proposals&crQ=router");
    expect(screen.getByRole("button", { name: "Pin this page to top nav bar" })).toBeTruthy();
  });
});
