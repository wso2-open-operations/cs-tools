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

/** Rounded-top, flat-bottom corner radius — a real tab's own silhouette
 * (a folder-tab shape), not a fully-rounded floating pill. A fully-rounded
 * shape is what reads as "a chip" rather than "a tab" even once every other
 * chip-like trait (visible border, small size) is removed — reported live
 * ("still I can see chips") against the active tab specifically, which had
 * kept the all-around-rounded shape from an earlier version of this fix. */
const TAB_CORNER_RADIUS = "8px 8px 0 0";
/** Taller than a stock `size="small"` Chip's own 24px — closer to a real
 * browser tab's proportions, which read noticeably taller/more substantial
 * than a small metadata chip. */
const TAB_HEIGHT = 32;

/**
 * The active tab's fill — the one place in this component that genuinely has
 * to branch on light vs. dark, because "the active tab is the lightest thing
 * in the strip" needs a *different* color in each, not a different token:
 *  - Light theme: solid white on a grey strip, exactly like a real browser's
 *    — anything translucent barely lifts off an already-light strip and was
 *    reported live as not white enough.
 *  - Dark theme: a solid grey. White here (translucent or not) renders as a
 *    glaring block against a navy strip — reported live — while a wash faint
 *    enough to avoid that stops reading as a distinct tab at all. Grey sits
 *    between the two: clearly lighter than the strip, still dark enough to
 *    keep the ambient near-white label legible on top of it.
 *
 * A literal color rather than a palette token because no token means "lighter
 * than the ambient surface in either mode" — the `background.*` tokens flip
 * their absolute lightness between the two themes (an earlier version used
 * `background.default`, which made the active tab the DARKEST thing in the
 * strip under the dark theme), and `action.selected` is an overlay tuned for
 * a subtle table-row highlight, too faint to pick the open tab out at a
 * glance. Both of those were reported live too.
 *
 * Scoped with `applyStyles("dark", …)` rather than a `palette.mode === "dark"`
 * check. This app drives theming through MUI CssVars, where `palette.mode`
 * stays pinned to the default scheme no matter which one is actually showing —
 * so a mode check here always took the light branch, and the dark theme
 * rendered a white tab with a white (invisible) label on it. `applyStyles`
 * emits a scheme-scoped selector instead, which is the mechanism the rest of
 * this app's per-scheme styling already uses (see `CaseMetaBand`, the a11y
 * theme overrides). Never reach for `palette.mode` in this codebase.
 *
 * Typed structurally rather than against the theme type — these two members
 * are all this needs, and MUI passes the real theme at call time.
 */
type ColorSchemeAwareTheme = {
  palette: { grey: { 700: string }; common: { white: string } };
  applyStyles: (
    scheme: "dark" | "light",
    styles: Record<string, unknown>,
  ) => Record<string, unknown>;
};

function activeTabFillStyles(theme: ColorSchemeAwareTheme): Record<string, unknown> {
  return {
    backgroundColor: theme.palette.common.white,
    ...theme.applyStyles("dark", { backgroundColor: theme.palette.grey[700] }),
  };
}

/**
 * Shared chip styling for the browser-tab look: a rounded-top/flat-bottom
 * "folder tab" shape (not a fully-rounded pill), a taller-than-stock height,
 * and — for the *active* tab only — `activeTabFillStyles` (see its own
 * comment for why that one is per-scheme) plus a theme-accent bottom rule
 * sitting on the strip's own divider line, so it reads as continuous with the
 * page below it. An inactive tab is fully transparent, showing the strip's
 * own tinted toolbar backdrop through untouched.
 *
 * Returns an `sx` *callback*, not a plain object, because that fill has to be
 * resolved per colour scheme off the real theme. Call sites therefore pass it
 * through directly (`sx={tabChipSx(active)}`) or, when they have extra styles
 * of their own to add, in `sx`'s array form — spreading it into an object
 * literal would spread a function and silently drop every style in it.
 *
 * Two early passes here, both corrected against live reference screenshots,
 * are worth not repeating: inactive tabs each kept a visible `outlined`
 * border (they still read as separate chips), and the active tab was a small,
 * fully-rounded floating pill with gaps on every side rather than a
 * flat-bottomed shape merged into the page below.
 */
function tabChipSx(active: boolean): (theme: ColorSchemeAwareTheme) => Record<string, unknown> {
  return (theme) => ({
    flexShrink: 0,
    maxWidth: 220,
    height: TAB_HEIGHT,
    cursor: "pointer",
    borderRadius: TAB_CORNER_RADIUS,
    ...(active
      ? {
          // Per-scheme — see `activeTabFillStyles`. The label keeps the
          // ambient `text.primary` under either theme rather than an inverted
          // one of its own, which both fills are chosen to stay legible
          // against; the bold weight and the accent rule below carry the rest
          // of the emphasis, so the fill doesn't have to do it alone.
          ...activeTabFillStyles(theme),
          fontWeight: 600,
          // A theme-accent rule along the bottom edge — a second, unambiguous
          // "this one is open" signal beyond the fill color alone, and the
          // reason the strip itself has NO bottom padding (see this
          // component's own strip `Box`): tabs sit flush on the strip's
          // bottom edge, so this rule lands directly on the strip's own
          // divider line and reads as one accent line rather than a second
          // line floating above it.
          //
          // An earlier version instead kept the strip's padding and pushed
          // this chip down over it with `position: relative; bottom: -9px`.
          // That is exactly the 9px the tablist's own `overflowY: "hidden"`
          // clips away, so the border silently never painted at all — the two
          // rules were added for unrelated reasons and quietly cancelled each
          // other. Keep this tab inside its container's own box; don't
          // reintroduce a negative offset here.
          borderBottom: "2px solid",
          borderBottomColor: "primary.main",
          // No additional hover treatment: this tab is already the most
          // prominent one, so hovering it shouldn't visibly change further.
          "&:hover": activeTabFillStyles(theme),
        }
      : {
          bgcolor: "transparent",
          // Explicit and deliberately far weaker than the active tab's own
          // light fill above — relying on Chip's own built-in
          // default hover treatment left hovering an inactive tab looking
          // about as prominent as the actual active tab, reported live as
          // "hover and active color seem similar" (unlike a real browser,
          // where a hover preview is clearly weaker than the genuinely
          // active tab).
          "&:hover": { bgcolor: "action.hover" },
        }),
  });
}

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
        pt: 1,
        // No bottom padding, so the tabs sit flush on the strip's own bottom
        // edge and the active tab's accent rule (see `tabChipSx`) lands
        // directly on the divider below instead of floating above it as a
        // separate second line — which is how it read while this was a
        // symmetric `py: 1`.
        pb: 0,
        // A recessed "toolbar" backdrop: a *darkening* overlay, so the strip
        // stays in the same family as the page content below it under either
        // theme. Deliberately not `action.hover`, which lightens under the
        // dark theme — that turned the whole strip into a pale slate band
        // floating above a much darker page, a far heavier separation than a
        // tab strip should draw, and it left the active tab's own light fill
        // with almost nothing to stand out against.
        bgcolor: "rgba(0, 0, 0, 0.18)",
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
          // Adjacent tabs touch — no gap between them — matching a real
          // browser tab strip; each tab's own distinct fill (see
          // `tabChipSx`) is what visually separates one from the next.
          gap: 0,
          overflowX: "auto",
          // Explicit, not left to default: per the CSS overflow spec, a box
          // with `overflow-x` set to anything other than `visible` and no
          // explicit `overflow-y` has its *other* axis computed as `auto`
          // too, not `visible` — so this box was silently scrollable
          // vertically as well, and the `::-webkit-scrollbar` styling below
          // (meant only for the horizontal scrollbar) applied to that
          // unwanted vertical one too, rendering a short vertical grey
          // thumb at the strip's right edge, right next to the kebab button
          // — reported live as "a scroll icon" there. This box's content
          // never needs to scroll vertically; only cutting off that axis
          // explicitly stops the browser from offering to.
          overflowY: "hidden",
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
              variant="filled"
              onClick={pinnedTab.onClick}
              sx={[tabChipSx(pinnedTab.active), { fontStyle: "italic" }]}
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
                variant="filled"
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
                sx={tabChipSx(active)}
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
