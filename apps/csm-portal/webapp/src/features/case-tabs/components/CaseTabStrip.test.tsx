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
import CaseTabStrip, {
  type PinnedTabProps,
} from "@features/case-tabs/components/CaseTabStrip";
import type { CaseTabState } from "@context/case-tabs/caseTabsTypes";

const TAB_1: CaseTabState = {
  id: "t1",
  caseId: "CS1",
  kind: "case",
  path: "/cases/CS1",
  label: "CS0001",
  internalId: "CPASUB-1",
  subject: "First case subject",
  hasDraft: false,
};
const TAB_2: CaseTabState = {
  id: "t2",
  caseId: "CS2",
  kind: "case",
  path: "/cases/CS2",
  label: "CS0002",
  internalId: "CPASUB-2",
  subject: "Second case subject",
  hasDraft: true,
};

const PINNED: PinnedTabProps = { label: "Dashboard", active: false, onClick: vi.fn() };

function noopHandlers() {
  return {
    onActivate: vi.fn(),
    onRequestClose: vi.fn(),
    onCloseAll: vi.fn(),
    onCloseOthers: vi.fn(),
  };
}

describe("CaseTabStrip", () => {
  it("renders nothing when there are no open case tabs, even with a pinned tab given", () => {
    const { container } = render(
      <CaseTabStrip tabs={[]} activeTabId={null} pinnedTab={PINNED} {...noopHandlers()} />,
    );
    expect(container).toBeEmptyDOMElement();
    expect(screen.queryByText("Dashboard")).not.toBeInTheDocument();
  });

  it("shows the pinned tab once at least one case tab is open", () => {
    render(
      <CaseTabStrip tabs={[TAB_1]} activeTabId="t1" pinnedTab={PINNED} {...noopHandlers()} />,
    );
    expect(screen.getByText("Dashboard")).toBeInTheDocument();
    expect(screen.getByText("CS0001")).toBeInTheDocument();
  });

  it("renders one chip per open tab, highlighting the active one", () => {
    render(
      <CaseTabStrip tabs={[TAB_1, TAB_2]} activeTabId="t2" {...noopHandlers()} />,
    );
    const tab1 = screen.getByText("CS0001");
    const tab2 = screen.getByText("CS0002");
    expect(tab1).toBeInTheDocument();
    expect(tab2.closest('[role="tab"]')).toHaveAttribute("aria-selected", "true");
    expect(tab1.closest('[role="tab"]')).toHaveAttribute("aria-selected", "false");
  });

  it("shows a Loading… placeholder (not the raw caseId/UUID) when no label has resolved yet", () => {
    render(
      <CaseTabStrip tabs={[{ ...TAB_1, label: undefined }]} activeTabId="t1" {...noopHandlers()} />,
    );
    expect(screen.getByText("Loading…")).toBeInTheDocument();
    expect(screen.queryByText("CS1")).not.toBeInTheDocument();
  });

  it("shows the internal id + subject (not the short label) in the tab's tooltip", async () => {
    render(<CaseTabStrip tabs={[TAB_1]} activeTabId="t1" {...noopHandlers()} />);
    // Chip text itself stays the short label.
    expect(screen.getByRole("tab")).toHaveTextContent("CS0001");
    // Tooltip content (internalId + subject) only mounts in the DOM once
    // hovered — oxygen-ui/MUI's Tooltip doesn't use a native `title`
    // attribute.
    fireEvent.mouseOver(screen.getByRole("tab"));
    expect(await screen.findByText("CPASUB-1 · First case subject")).toBeInTheDocument();
  });

  it("calls onActivate when a tab chip is clicked", () => {
    const handlers = noopHandlers();
    render(<CaseTabStrip tabs={[TAB_1, TAB_2]} activeTabId="t1" {...handlers} />);
    fireEvent.click(screen.getByText("CS0002"));
    expect(handlers.onActivate).toHaveBeenCalledWith("t2");
  });

  it("calls onRequestClose (not onActivate) when a tab's close button is clicked", () => {
    const handlers = noopHandlers();
    render(<CaseTabStrip tabs={[TAB_1]} activeTabId="t1" {...handlers} />);
    // oxygen-ui/MUI's delete affordance is a bare `aria-hidden` svg icon with
    // its own onClick, not a separately-labelled control (same limitation as
    // this codebase's other Chip-with-onDelete usage, e.g. `PinnedTabs`) —
    // so the delete click has to target that icon directly, by test id.
    fireEvent.click(screen.getByTestId("CancelIcon"));
    expect(handlers.onRequestClose).toHaveBeenCalledWith("t1");
    expect(handlers.onActivate).not.toHaveBeenCalled();
  });

  // Regression test: the chip's own `aria-label` used to be `Close ${label}`
  // — since `aria-label` sets the ACCESSIBLE NAME of the whole
  // `role="tab"` chip (not just its delete icon), a screen reader announced
  // the entire tab as "Close CS0001" instead of "CS0001" (or, once the
  // Tooltip's own fallback labeling is accounted for, "CPASUB-1 · First
  // case subject" — either way, never prefixed with "Close").
  it("the tab chip's accessible name is its own label/tooltip text, not 'Close <label>'", () => {
    render(<CaseTabStrip tabs={[TAB_1]} activeTabId="t1" {...noopHandlers()} />);
    const tab = screen.getByRole("tab");
    expect(tab).not.toHaveAccessibleName(/^close /i);
    expect(screen.queryByRole("tab", { name: /^close /i })).not.toBeInTheDocument();
  });

  describe("right-click context menu", () => {
    it("right-clicking a tab chip offers Close other tabs and Close all tabs", () => {
      const handlers = noopHandlers();
      render(<CaseTabStrip tabs={[TAB_1, TAB_2]} activeTabId="t1" {...handlers} />);
      fireEvent.contextMenu(screen.getByText("CS0001"));
      expect(screen.getByText("Close other tabs")).toBeInTheDocument();
      expect(screen.getByText("Close all tabs")).toBeInTheDocument();

      fireEvent.click(screen.getByText("Close other tabs"));
      expect(handlers.onCloseOthers).toHaveBeenCalledWith("t1");
      expect(handlers.onCloseAll).not.toHaveBeenCalled();
    });

    it("Close all tabs from a tab's context menu closes every tab", () => {
      const handlers = noopHandlers();
      render(<CaseTabStrip tabs={[TAB_1, TAB_2]} activeTabId="t1" {...handlers} />);
      fireEvent.contextMenu(screen.getByText("CS0002"));
      fireEvent.click(screen.getByText("Close all tabs"));
      expect(handlers.onCloseAll).toHaveBeenCalledTimes(1);
    });

    it("right-clicking empty strip space offers only Close all tabs, not Close other tabs", () => {
      const handlers = noopHandlers();
      render(<CaseTabStrip tabs={[TAB_1, TAB_2]} activeTabId="t1" {...handlers} />);
      fireEvent.contextMenu(screen.getByRole("tablist"));
      expect(screen.getByText("Close all tabs")).toBeInTheDocument();
      expect(screen.queryByText("Close other tabs")).not.toBeInTheDocument();

      fireEvent.click(screen.getByText("Close all tabs"));
      expect(handlers.onCloseAll).toHaveBeenCalledTimes(1);
    });

    it("anchors the menu at the right-click cursor position, not the triggering element", () => {
      // Regression test: the menu used to be `anchorEl`-positioned (top-left
      // corner of whatever element was right-clicked), which for the
      // strip's own empty-space right-click meant the strip's full-width
      // container — the menu always opened at the strip's left edge
      // regardless of where within it the user actually clicked. jsdom gives
      // no real pixel layout (and MUI's positioning math applies its own
      // fixed offsets on top of the anchor point), so rather than assert an
      // absolute pixel value, this asserts that the menu's *rendered
      // position moves by the same delta* as the right-click coordinates
      // — proving it tracks the cursor, not a fixed element-derived point.
      const menuPaperStyle = (): CSSStyleDeclaration =>
        window.getComputedStyle(screen.getByRole("menu").closest(".MuiPopover-paper")!);

      const handlers = noopHandlers();
      render(<CaseTabStrip tabs={[TAB_1, TAB_2]} activeTabId="t1" {...handlers} />);

      fireEvent.contextMenu(screen.getByRole("tablist"), { clientX: 100, clientY: 50 });
      const firstStyle = menuPaperStyle();
      const firstTop = parseFloat(firstStyle.top);
      const firstLeft = parseFloat(firstStyle.left);

      fireEvent.keyDown(screen.getByRole("menu"), { key: "Escape" });

      fireEvent.contextMenu(screen.getByRole("tablist"), { clientX: 300, clientY: 200 });
      const secondStyle = menuPaperStyle();
      const secondTop = parseFloat(secondStyle.top);
      const secondLeft = parseFloat(secondStyle.left);

      expect(secondLeft - firstLeft).toBe(200);
      expect(secondTop - firstTop).toBe(150);
    });

    it("the pinned tab is never a right-click target", () => {
      const handlers = noopHandlers();
      render(
        <CaseTabStrip tabs={[TAB_1]} activeTabId="t1" pinnedTab={PINNED} {...handlers} />,
      );
      fireEvent.contextMenu(screen.getByText("Dashboard"));
      // Bubbles to the strip's own onContextMenu (the "empty space" case),
      // since the pinned chip has none of its own — offers only Close all.
      expect(screen.getByText("Close all tabs")).toBeInTheDocument();
      expect(screen.queryByText("Close other tabs")).not.toBeInTheDocument();
    });
  });

  // Regression tests: `role="tablist"`/`role="tab"` promise standard
  // tab-widget keyboard behavior (roving `tabIndex`, arrow-key movement,
  // `aria-controls` linking each tab to its panel) — an earlier version of
  // this strip used the roles without any of that: every chip was its own
  // `Tab`-key stop, no arrow-key navigation existed, and bulk-close was
  // reachable only by right-click (unreachable by keyboard at all).
  describe("keyboard support", () => {
    it("only the active tab (or the pinned tab, while active) is a natural Tab-key stop — every other chip is tabIndex -1", () => {
      render(
        <CaseTabStrip tabs={[TAB_1, TAB_2]} activeTabId="t2" pinnedTab={PINNED} {...noopHandlers()} />,
      );
      const tabs = screen.getAllByRole("tab");
      // [pinned (inactive), CS0001 (inactive), CS0002 (active)]
      expect(tabs[0]).toHaveAttribute("tabindex", "-1");
      expect(tabs[1]).toHaveAttribute("tabindex", "-1");
      expect(tabs[2]).toHaveAttribute("tabindex", "0");
    });

    it("the pinned tab is the roving tab stop while it's the active/live view", () => {
      render(
        <CaseTabStrip
          tabs={[TAB_1]}
          activeTabId={null}
          pinnedTab={{ ...PINNED, active: true }}
          {...noopHandlers()}
        />,
      );
      const tabs = screen.getAllByRole("tab");
      expect(tabs[0]).toHaveAttribute("tabindex", "0");
      expect(tabs[1]).toHaveAttribute("tabindex", "-1");
    });

    it("ArrowRight moves focus and activation to the next tab, wrapping from the last back to the pinned tab", () => {
      const handlers = noopHandlers();
      const { rerender } = render(
        <CaseTabStrip tabs={[TAB_1, TAB_2]} activeTabId="t1" pinnedTab={PINNED} {...handlers} />,
      );
      fireEvent.keyDown(screen.getByRole("tablist"), { key: "ArrowRight" });
      expect(handlers.onActivate).toHaveBeenCalledWith("t2");

      // Wrapping: ArrowRight from the LAST tab goes back to the pinned tab.
      // `onActivate`'s mock doesn't actually change `activeTabId` (this is
      // a presentational-component test), so re-render with "t2" active to
      // simulate the activation having taken effect, matching how
      // `CaseTabStripBar` would re-render this component after a real
      // `onActivate` call updates the controller's state.
      rerender(
        <CaseTabStrip tabs={[TAB_1, TAB_2]} activeTabId="t2" pinnedTab={PINNED} {...handlers} />,
      );
      fireEvent.keyDown(screen.getByRole("tablist"), { key: "ArrowRight" });
      expect(PINNED.onClick).toHaveBeenCalled();
    });

    it("ArrowLeft moves focus and activation to the previous tab, wrapping from the pinned tab to the last one", () => {
      const handlers = noopHandlers();
      const { rerender } = render(
        <CaseTabStrip tabs={[TAB_1, TAB_2]} activeTabId="t1" pinnedTab={PINNED} {...handlers} />,
      );
      // Currently on t1 (index 1 in [pinned, t1, t2]) — ArrowLeft goes to
      // the pinned tab (index 0).
      fireEvent.keyDown(screen.getByRole("tablist"), { key: "ArrowLeft" });
      expect(PINNED.onClick).toHaveBeenCalled();

      // From the pinned tab (now "active"), ArrowLeft wraps to the LAST tab.
      rerender(
        <CaseTabStrip
          tabs={[TAB_1, TAB_2]}
          activeTabId={null}
          pinnedTab={{ ...PINNED, active: true }}
          {...handlers}
        />,
      );
      fireEvent.keyDown(screen.getByRole("tablist"), { key: "ArrowLeft" });
      expect(handlers.onActivate).toHaveBeenCalledWith("t2");
    });

    it("Home/End jump to the first/last tab", () => {
      const handlers = noopHandlers();
      render(
        <CaseTabStrip tabs={[TAB_1, TAB_2]} activeTabId="t2" pinnedTab={PINNED} {...handlers} />,
      );
      const tablist = screen.getByRole("tablist");
      fireEvent.keyDown(tablist, { key: "End" });
      expect(handlers.onActivate).toHaveBeenLastCalledWith("t2");
      fireEvent.keyDown(tablist, { key: "Home" });
      expect(PINNED.onClick).toHaveBeenCalled();
    });

    it("moves real DOM focus to the newly-activated tab, not just calling the handler", () => {
      render(<CaseTabStrip tabs={[TAB_1, TAB_2]} activeTabId="t1" {...noopHandlers()} />);
      const tabs = screen.getAllByRole("tab");
      tabs[0].focus();
      fireEvent.keyDown(tabs[0], { key: "ArrowRight" });
      expect(tabs[1]).toHaveFocus();
    });

    it("each case tab's chip carries id/aria-controls pointing at its own panel", () => {
      render(<CaseTabStrip tabs={[TAB_1, TAB_2]} activeTabId="t1" {...noopHandlers()} />);
      const tabs = screen.getAllByRole("tab");
      expect(tabs[0]).toHaveAttribute("id", "case-tab-t1");
      expect(tabs[0]).toHaveAttribute("aria-controls", "case-tab-panel-t1");
      expect(tabs[1]).toHaveAttribute("id", "case-tab-t2");
      expect(tabs[1]).toHaveAttribute("aria-controls", "case-tab-panel-t2");
    });

    it("a visible kebab button opens the same Close all/Close other tabs menu right-click does — the only keyboard-reachable path to bulk-close", () => {
      const handlers = noopHandlers();
      render(<CaseTabStrip tabs={[TAB_1, TAB_2]} activeTabId="t1" {...handlers} />);
      const kebab = screen.getByRole("button", { name: "More tab actions" });
      // A real keyboard user reaches this by Tab (it's a natural stop,
      // unlike the tab chips) then Enter/Space — `fireEvent.click` is the
      // faithful jsdom stand-in for that activation, same as this test
      // file's other button interactions.
      fireEvent.click(kebab);
      expect(screen.getByText("Close other tabs")).toBeInTheDocument();
      expect(screen.getByText("Close all tabs")).toBeInTheDocument();

      fireEvent.click(screen.getByText("Close other tabs"));
      expect(handlers.onCloseOthers).toHaveBeenCalledWith("t1");
    });

    it("the kebab menu offers only Close all tabs (not Close other tabs) when no tab is active — the pinned tab is the live view", () => {
      render(
        <CaseTabStrip
          tabs={[TAB_1, TAB_2]}
          activeTabId={null}
          pinnedTab={{ ...PINNED, active: true }}
          {...noopHandlers()}
        />,
      );
      fireEvent.click(screen.getByRole("button", { name: "More tab actions" }));
      expect(screen.getByText("Close all tabs")).toBeInTheDocument();
      expect(screen.queryByText("Close other tabs")).not.toBeInTheDocument();
    });
  });
});
