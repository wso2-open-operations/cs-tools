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

import { Box, Chip, IconButton, Menu, MenuItem, Tooltip } from "@wso2/oxygen-ui";
import { MoreVertical } from "@wso2/oxygen-ui-icons-react";
import {
  useRef,
  useState,
  type JSX,
  type KeyboardEvent as ReactKeyboardEvent,
  type MouseEvent as ReactMouseEvent,
} from "react";
import type { CaseTabState } from "@context/case-tabs/caseTabsTypes";
import { tabDisplayLabel } from "@features/case-tabs/utils/tabDisplayLabel";
import { tabElementId, tabPanelElementId } from "@features/case-tabs/utils/tabElementIds";

export interface PinnedTabProps {
  label: string;
  active: boolean;
  onClick: () => void;
}

export interface CaseTabStripProps {
  tabs: CaseTabState[];
  activeTabId: string | null;
  onActivate: (id: string) => void;
  onRequestClose: (id: string) => void;
  /** Right-click "Close all tabs" (on a chip, on empty strip space, or via
   * the strip's own kebab menu — see this component's own doc comment). */
  onCloseAll: () => void;
  /** Right-click "Close other tabs" — every open tab except `keepId`. */
  onCloseOthers: (keepId: string) => void;
  /** The permanent, non-closable "wherever the user currently is" tab at
   * position 0 — see `useCurrentLocationTab`. Optional purely so this
   * component's own tests can exercise the plain case-tab strip in
   * isolation; `CaseTabStripBar` always supplies one. Never rendered when
   * `tabs` is empty — the whole strip hides in that case (see this
   * component's own doc comment). */
  pinnedTab?: PinnedTabProps;
}

/** Tooltip content: internal/project-scoped id + subject — a fuller
 * identity than the chip's own short number-only label. Falls back to the
 * chip label alone while the record's own data (and so its tooltip fields)
 * hasn't resolved yet. */
function tabTooltip(tab: CaseTabState): string {
  if (!tab.internalId && !tab.subject) return tabDisplayLabel(tab);
  return [tab.internalId, tab.subject].filter(Boolean).join(" · ");
}

/** Sentinel key for the pinned tab in the roving-tabIndex/arrow-key order —
 * never a real `CaseTabState.id` (those are all `case-tab-<...>`, see
 * `CaseTabsContext`'s `nextTabId`), so it can't collide. */
const PINNED_KEY = "__pinned__";

type ContextMenuTarget = { kind: "tab"; tabId: string } | { kind: "empty" };

type MenuAnchorPosition = { top: number; left: number };

const ARROW_KEYS = new Set(["ArrowLeft", "ArrowRight", "Home", "End"]);

/**
 * Browser-tab-like strip for in-app open tabs, rendered by `CaseTabStripBar`
 * above the routed page content. Presentational: all open/close/activate
 * decisions (capacity, the unsaved-draft confirm) are made by the caller —
 * this component only renders the given `tabs` (plus the pinned tab, if
 * given) and reports clicks. See `useCaseTabCloseConfirm` for the
 * close-confirm dialog this strip's `onRequestClose` is typically wired to.
 *
 * Renders nothing at all when `tabs` is empty — including the pinned tab,
 * which would otherwise sit alone taking up a full strip's worth of space
 * for no case tabs open. The pinned tab only appears once the first case tab
 * does.
 *
 * Full keyboard support, matching the standard ARIA `tablist`/`tab`/
 * `tabpanel` authoring pattern those roles promise (an earlier version of
 * this strip used the roles without the behavior they imply — every chip
 * was its own tab stop, no arrow-key movement, and bulk-close was reachable
 * only via right-click):
 *  - Roving `tabIndex`: only the currently active tab (or the pinned tab,
 *    while it's the live view) is a natural `Tab`-key stop; every other chip
 *    is `tabIndex={-1}` — reachable by arrow key, not by repeatedly
 *    Tab-ing through the whole strip.
 *  - Left/Right arrow keys move focus AND selection together (this strip's
 *    own "automatic activation" choice — matching a browser tab strip's own
 *    behavior, which is what this whole feature is modeled on) between
 *    tabs, wrapping at either end; Home/End jump to the first/last tab.
 *  - Each case tab's chip carries `id`/`aria-controls` pointing at its own
 *    rendered panel (`CaseTabIsolatedRouter`, which sets a matching
 *    `role="tabpanel"` `id`/`aria-labelledby` back at it).
 *  - The kebab button (`aria-label="More tab actions"`) opens the exact same
 *    "Close all tabs"/"Close other tabs" menu right-clicking a chip or the
 *    strip's own empty space does — the only way to reach either action
 *    without a mouse, since neither has a dedicated visible button of its
 *    own otherwise. "Close other tabs" from the kebab keeps whichever tab
 *    is CURRENTLY active (or offers only "Close all tabs" when the pinned
 *    tab is the live view, since there's no "other tabs relative to it" to
 *    keep one of).
 */
export default function CaseTabStrip({
  tabs,
  activeTabId,
  onActivate,
  onRequestClose,
  onCloseAll,
  onCloseOthers,
  pinnedTab,
}: CaseTabStripProps): JSX.Element | null {
  const [menuAnchorPosition, setMenuAnchorPosition] = useState<MenuAnchorPosition | null>(null);
  const [menuTarget, setMenuTarget] = useState<ContextMenuTarget | null>(null);
  const chipRefs = useRef<Map<string, HTMLElement>>(new Map());
  const kebabButtonRef = useRef<HTMLButtonElement>(null);

  if (tabs.length === 0) return null;

  const closeContextMenu = (): void => {
    setMenuAnchorPosition(null);
    setMenuTarget(null);
  };

  const openMenuAt = (position: MenuAnchorPosition, target: ContextMenuTarget): void => {
    setMenuAnchorPosition(position);
    setMenuTarget(target);
  };

  const openContextMenu = (e: ReactMouseEvent<HTMLElement>, target: ContextMenuTarget): void => {
    e.preventDefault();
    // Anchor at the cursor, not the triggering element — an
    // element-anchored `<Menu>` opens at that element's top-left corner
    // (MUI's `anchorEl` default), which for the strip's own empty-space
    // right-click means the strip's full-width container: the menu would
    // always render at the strip's left edge regardless of where within it
    // was actually clicked.
    openMenuAt({ top: e.clientY, left: e.clientX }, target);
  };

  const openKebabMenu = (): void => {
    const rect = kebabButtonRef.current?.getBoundingClientRect();
    const position = rect ? { top: rect.bottom, left: rect.left } : { top: 0, left: 0 };
    // "Close other tabs" relative to whichever tab is CURRENTLY active —
    // there's nothing to keep relative to when the pinned tab is the live
    // view instead (`activeTabId` is `null` then — see `CaseTabStripBar`),
    // so that offers only "Close all tabs".
    openMenuAt(position, activeTabId ? { kind: "tab", tabId: activeTabId } : { kind: "empty" });
  };

  // Roving-tabIndex order: the pinned tab (if given) first, then every open
  // case tab in strip order — arrow keys/Home/End move through this same
  // sequence, wrapping at either end.
  const order: string[] = pinnedTab ? [PINNED_KEY, ...tabs.map((t) => t.id)] : tabs.map((t) => t.id);
  const activeKey = pinnedTab?.active ? PINNED_KEY : (activeTabId ?? tabs[0]?.id ?? PINNED_KEY);

  const activateKey = (key: string): void => {
    if (key === PINNED_KEY) pinnedTab?.onClick();
    else onActivate(key);
    chipRefs.current.get(key)?.focus();
  };

  const handleTablistKeyDown = (e: ReactKeyboardEvent<HTMLElement>): void => {
    if (!ARROW_KEYS.has(e.key)) return;
    e.preventDefault();
    const currentIndex = Math.max(order.indexOf(activeKey), 0);
    let nextIndex = currentIndex;
    if (e.key === "ArrowLeft") nextIndex = currentIndex === 0 ? order.length - 1 : currentIndex - 1;
    else if (e.key === "ArrowRight")
      nextIndex = currentIndex === order.length - 1 ? 0 : currentIndex + 1;
    else if (e.key === "Home") nextIndex = 0;
    else if (e.key === "End") nextIndex = order.length - 1;
    activateKey(order[nextIndex]);
  };

  const setChipRef = (key: string) => (el: HTMLElement | null): void => {
    if (el) chipRefs.current.set(key, el);
    else chipRefs.current.delete(key);
  };

  return (
    <Box
      sx={{
        display: "flex",
        alignItems: "center",
        gap: 0.5,
        px: 3,
        py: 1,
        borderBottom: 1,
        borderColor: "divider",
        flexShrink: 0,
      }}
    >
      <Box
        role="tablist"
        aria-label="Open cases"
        onKeyDown={handleTablistKeyDown}
        onContextMenu={(e: ReactMouseEvent<HTMLElement>) => {
          // Only when the strip's own background was right-clicked, not a
          // chip inside it — each chip has its own onContextMenu, which stops
          // this one from also firing (see its stopPropagation below).
          openContextMenu(e, { kind: "empty" });
        }}
        sx={{
          display: "flex",
          alignItems: "center",
          gap: 0.75,
          overflowX: "auto",
          minWidth: 0,
          flex: 1,
          "&::-webkit-scrollbar": { height: 6 },
          "&::-webkit-scrollbar-thumb": { bgcolor: "action.disabled", borderRadius: 3 },
        }}
      >
        {pinnedTab && (
          <Tooltip title={pinnedTab.label}>
            {/* No `onDelete` and no `onContextMenu` — this tab is permanent,
                not part of the closable case-tab set (see
                `useCurrentLocationTab`'s doc comment), so it's excluded from
                both context-menu actions and never itself a right-click
                target. Also no `id`/`aria-controls` (unlike a case tab
                below): unlike those, it has no discrete "panel" element
                anywhere for that to point at — its content is just
                whatever route is currently live. */}
            <Chip
              ref={setChipRef(PINNED_KEY)}
              size="small"
              role="tab"
              aria-selected={pinnedTab.active}
              tabIndex={activeKey === PINNED_KEY ? 0 : -1}
              label={pinnedTab.label}
              variant={pinnedTab.active ? "filled" : "outlined"}
              onClick={pinnedTab.onClick}
              sx={{
                flexShrink: 0,
                maxWidth: 220,
                cursor: "pointer",
                fontStyle: "italic",
                ...(pinnedTab.active ? { bgcolor: "action.selected", fontWeight: 600 } : {}),
              }}
            />
          </Tooltip>
        )}
        {tabs.map((tab) => {
          const active = tab.id === activeTabId;
          const label = tabDisplayLabel(tab);
          return (
            <Tooltip key={tab.id} title={tabTooltip(tab)}>
              <Chip
                ref={setChipRef(tab.id)}
                id={tabElementId(tab.id)}
                aria-controls={tabPanelElementId(tab.id)}
                size="small"
                role="tab"
                aria-selected={active}
                tabIndex={activeKey === tab.id ? 0 : -1}
                label={label}
                variant={active ? "filled" : "outlined"}
                onClick={() => onActivate(tab.id)}
                onContextMenu={(e: ReactMouseEvent<HTMLElement>) => {
                  e.stopPropagation();
                  openContextMenu(e, { kind: "tab", tabId: tab.id });
                }}
                onDelete={(e) => {
                  // Chip's onDelete already receives a synthetic event whose
                  // propagation stopping is handled by oxygen-ui internally;
                  // stopPropagation here too so a delete click never also
                  // triggers the Chip's own onClick (which would activate the
                  // tab that's about to close).
                  e.stopPropagation();
                  onRequestClose(tab.id);
                }}
                // Deliberately NO `aria-label` here — `aria-label` sets the
                // ACCESSIBLE NAME of this whole `role="tab"` chip, not just
                // its delete affordance; a `Close ${label}` value made a
                // screen reader announce the entire tab as "Close CS0001"
                // instead of "CS0001". Falls back to the chip's own visible
                // text (the label) as its accessible name instead, same as
                // any other unlabelled Chip. The delete icon itself has no
                // separate accessible name of its own — same limitation as
                // this codebase's other Chip-with-onDelete usage (see this
                // file's own test for how it's exercised: by test id, not by
                // an accessible name, until oxygen-ui/MUI's `Chip` exposes
                // one for `deleteIcon` directly).
                sx={{
                  flexShrink: 0,
                  maxWidth: 220,
                  cursor: "pointer",
                  ...(active ? { bgcolor: "action.selected", fontWeight: 600 } : {}),
                }}
              />
            </Tooltip>
          );
        })}
      </Box>

      <Tooltip title="More tab actions">
        <IconButton
          ref={kebabButtonRef}
          size="small"
          aria-label="More tab actions"
          onClick={openKebabMenu}
          sx={{ flexShrink: 0 }}
        >
          <MoreVertical size={16} />
        </IconButton>
      </Tooltip>

      <Menu
        open={Boolean(menuAnchorPosition)}
        onClose={closeContextMenu}
        anchorReference="anchorPosition"
        anchorPosition={menuAnchorPosition ?? undefined}
      >
        {menuTarget?.kind === "tab" && (
          <MenuItem
            onClick={() => {
              if (menuTarget.kind === "tab") onCloseOthers(menuTarget.tabId);
              closeContextMenu();
            }}
          >
            Close other tabs
          </MenuItem>
        )}
        <MenuItem
          onClick={() => {
            onCloseAll();
            closeContextMenu();
          }}
        >
          Close all tabs
        </MenuItem>
      </Menu>
    </Box>
  );
}
