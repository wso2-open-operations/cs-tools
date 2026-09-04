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

import { Box, Link, Sidebar, Tooltip, Typography } from "@wso2/oxygen-ui";
import { useCallback, useEffect, useMemo, type JSX } from "react";
import { Link as NavigateLink, useLocation } from "react-router";
import {
  type CsmNavSection,
  navNodeById,
  navNodeHref,
  navNodeMatchForPath,
  navSectionForPath,
} from "@config/csmNavItems";
import {
  featureState,
  visibleNavChildren,
  visibleNavSections,
} from "@config/featureFlags";
import { useNavTransition } from "@hooks/useNavTransition";
import { usePermissions } from "@hooks/usePermissions";
import { isNavNodeAuthorized } from "@layouts/navFilter";

/** Tooltip for a disabled WIP item. Includes the label so the collapsed rail
 *  (which hides the label) still says which feature it is. */
const wipTooltip = (label: string): string =>
  `${label} — this section is still under construction`;

const COMPANY_NAME = "WSO2 LLC";
const TERMS_OF_SERVICE_URL = "https://wso2.com/terms-of-use/";
const PRIVACY_POLICY_URL = "https://wso2.com/privacy-policy/";

// Persists the last resolved section across a full page reload, and is the
// single home for it — no in-component ref/state mirror, so nothing is read
// out of a ref during render (react-hooks/refs). sessionStorage (not
// localStorage) because this is transient nav context for the current tab,
// not a durable preference like SIDEBAR_COLLAPSED_KEY.
const LAST_SECTION_KEY = "csm.sidebar.lastSection";

function getLastSectionId(): string {
  try {
    const stored = sessionStorage.getItem(LAST_SECTION_KEY);
    if (!stored) return "dashboard";
    // Normalise on read, not just on write. The fallback must be a *section*
    // id, and an earlier build persisted a submenu child's dotted id verbatim
    // (`operations.incidents`) -- sessionStorage survives a reload, so a tab
    // that was open across that deploy can still hand one back. Reading
    // defensively keeps the very first render correct no matter what wrote it.
    const dot = stored.indexOf(".");
    return dot === -1 ? stored : stored.slice(0, dot);
  } catch {
    return "dashboard";
  }
}

function setLastSectionId(id: string): void {
  try {
    sessionStorage.setItem(LAST_SECTION_KEY, id);
  } catch {
    /* ignore */
  }
}

interface CsmSideBarProps {
  collapsed: boolean;
  expandedMenus?: Record<string, boolean>;
  onSelect?: (id: string) => void;
  onToggleExpand?: (id: string) => void;
}

/**
 * A section whose children get their own submenu entries in the rail (as
 * opposed to a section like Customers/Settings, whose children live in an
 * in-page tab/route strip instead). A query-param tab's `tab` field is the
 * structural marker for this — see `CsmNavNode.tab`'s doc comment — so this
 * stays correct if another section adopts the same pattern later, with no
 * hardcoded id list to keep in sync.
 */
function isSubmenuSection(section: CsmNavSection): boolean {
  return Boolean(section.children?.length) && (section.children ?? []).every((child) => Boolean(child.tab));
}

function pickActiveId(pathname: string): string {
  if (pathname === "/" || pathname === "") return "dashboard";
  // A submenu section's own child (e.g. `operations.incidents`) gets its own
  // rail entry, so highlight *that* rather than the section — this is what
  // lets the nested item light up and the parent auto-expand. Every other
  // route (including a tab-less section's own child route, e.g.
  // `/customers/accounts`, which has no rail entry of its own) still
  // highlights the owning *section*. Routes with no owning section (e.g.
  // `/people/:id`, linked from all over the app) fall back to whichever
  // section was last active instead of hard-jumping to Dashboard.
  const match = navNodeMatchForPath(pathname);
  if (match?.node.tab !== undefined) return match.node.id;
  return navSectionForPath(pathname)?.id ?? getLastSectionId();
}

export default function CsmSideBar({
  collapsed,
  expandedMenus,
  onSelect,
  onToggleExpand,
}: CsmSideBarProps): JSX.Element {
  const location = useLocation();
  const navigate = useNavTransition();
  const { roles: userRoles } = usePermissions();
  const activeItem = pickActiveId(location.pathname);
  useEffect(() => {
    // The persisted id is the fallback used for routes with no owning section
    // (see `pickActiveId`'s doc comment) -- it must stay a *section* id.
    // `activeItem` can be a submenu child's own dotted id (e.g.
    // `operations.incidents`); persisting that verbatim meant navigating to
    // an unrelated, section-less route later read the stale child id back as
    // the fallback and lit up/expanded the wrong (previous) section.
    const owningSectionId = activeItem.includes(".")
      ? activeItem.slice(0, activeItem.indexOf("."))
      : activeItem;
    setLastSectionId(owningSectionId);
  }, [activeItem]);

  // A submenu child (e.g. `operations.incidents`, rendered without its own
  // navigating `Link` — see below) reaches here through Oxygen's own
  // `onSelect`, dotted ids are how it's told apart from a flat top-level
  // item's id (never dotted) whose Link already handled the navigation
  // itself. A WIP child stays visible (so a deployment's config change is
  // still legible in the rail) but inert, same intent as a WIP top-level
  // section, just enforced here instead of structurally — Oxygen's own
  // collapsed-rail popover reads a nested item's id/icon/label straight off
  // its props, so it has to stay a plain `Sidebar.Item` rather than being
  // wrapped in a disabling `Box`/`Tooltip` the way a top-level WIP section is.
  const handleSelect = useCallback(
    (id: string): void => {
      if (id.includes(".") && featureState(id) !== "wip") {
        const node = navNodeById(id);
        if (node) navigate(navNodeHref(node));
      }
      onSelect?.(id);
    },
    [navigate, onSelect],
  );

  // A submenu section auto-expands the very first time one of its own
  // children becomes the active item — e.g. a fresh load landing directly on
  // a child route, before `onToggleExpand` has ever fired for that section —
  // since `expandedMenus` alone can't do this (it starts empty every
  // session). Only applied while the section has no explicit entry of its
  // own (`undefined`): once the user has toggled it at all (`toggleMenu`
  // always writes a real `true`/`false`), that choice wins even while one of
  // its children is still the active page. Forcing `true` unconditionally
  // here previously made collapsing a section impossible without first
  // navigating off its active child page — clicking the chevron flipped
  // `expandedMenus` correctly, but this memo immediately overwrote it back
  // to `true` on the very next render since `activeItem` hadn't changed.
  const effectiveExpandedMenus = useMemo(() => {
    const dot = activeItem.indexOf(".");
    if (dot === -1) return expandedMenus;
    const sectionId = activeItem.slice(0, dot);
    if (expandedMenus?.[sectionId] !== undefined) return expandedMenus;
    return { ...expandedMenus, [sectionId]: true };
  }, [expandedMenus, activeItem]);

  return (
    <Sidebar
      collapsed={collapsed}
      // Wider than Oxygen's own 250px default: a submenu child's label sits
      // under extra left padding (depth-based indent) plus its icon, and at
      // 250px the longest one ("Problem management") ran out of room and
      // clipped.
      width={280}
      activeItem={activeItem}
      expandedMenus={effectiveExpandedMenus}
      onSelect={handleSelect}
      onToggleExpand={onToggleExpand}
    >
      <Sidebar.Nav>
        <Sidebar.Category>
          {/* `hidden` sections are filtered out entirely; `wip` ones stay
              rendered but disabled below. */}
          {visibleNavSections()
            .filter((item) => isNavNodeAuthorized(item, userRoles))
            .map((item) => {
            const itemContent = (
              <Sidebar.Item id={item.id}>
                <Sidebar.ItemIcon>
                  <item.icon size={20} />
                </Sidebar.ItemIcon>
                {/* Plain string: Oxygen derives the collapsed-rail tooltip via
                    String(ItemLabel.children), so a wrapper element would render
                    as "[object Object]". */}
                <Sidebar.ItemLabel>{item.label}</Sidebar.ItemLabel>
              </Sidebar.Item>
            );

            // WIP sections stay visible but disabled: no navigating Link, dimmed
            // and non-clickable (pointer events blocked on the inner box so no
            // click reaches Oxygen's select handler). The outer element is a
            // focusable div (tabIndex 0, aria-disabled) so keyboard users can
            // reach it and reveal the "work in progress" tooltip, which fires on
            // both hover and focus. Their routes render the coming-soon page
            // (see App.tsx's WipRouteGuard).
            if (featureState(item.id) === "wip") {
              return (
                <Tooltip
                  key={item.id}
                  title={wipTooltip(item.label)}
                  placement="right"
                >
                  <Box
                    aria-disabled
                    tabIndex={0}
                    sx={{ display: "block", cursor: "not-allowed" }}
                  >
                    <Box sx={{ opacity: 0.45, pointerEvents: "none" }}>
                      {itemContent}
                    </Box>
                  </Box>
                </Tooltip>
              );
            }

            // A submenu section (Operations, Security Center) renders its
            // children as nested `Sidebar.Item`s instead of navigating
            // directly: Oxygen shows a chevron and calls `onToggleExpand`
            // for any item with nested items rather than `onSelect`, so this
            // parent is deliberately NOT wrapped in a `Link` — only its
            // children (below) navigate. A section whose config has hidden
            // every one of its children falls through to the plain flat item
            // instead of rendering an entry with nothing to expand.
            const children = isSubmenuSection(item)
              ? visibleNavChildren(item).filter((child) => isNavNodeAuthorized(child, userRoles))
              : [];
            if (children.length > 0) {
              return (
                <Sidebar.Item id={item.id} key={item.id}>
                  <Sidebar.ItemIcon>
                    <item.icon size={20} />
                  </Sidebar.ItemIcon>
                  <Sidebar.ItemLabel>{item.label}</Sidebar.ItemLabel>
                  {children.map((child) => {
                    const childWip = featureState(child.id) === "wip";
                    return (
                      <Sidebar.Item id={child.id} key={child.id}>
                        {child.icon && (
                          <Sidebar.ItemIcon>
                            <child.icon size={18} />
                          </Sidebar.ItemIcon>
                        )}
                        <Sidebar.ItemLabel>{child.label}</Sidebar.ItemLabel>
                        {childWip && (
                          <Sidebar.ItemBadge color="warning">WIP</Sidebar.ItemBadge>
                        )}
                      </Sidebar.Item>
                    );
                  })}
                </Sidebar.Item>
              );
            }

            return (
              <Link
                key={item.id}
                component={NavigateLink}
                to={item.href}
                color="inherit"
                underline="none"
              >
                {itemContent}
              </Link>
            );
          })}
        </Sidebar.Category>
      </Sidebar.Nav>

      {/* Legal footer lives at the bottom of the left rail so the main content
          area keeps its full height for meaningful work. Hidden when the rail
          is collapsed — the legal text won't fit the narrow rail. */}
      {!collapsed && (
        <Sidebar.Footer showDivider>
          <Box
            sx={{
              px: 2,
              py: 1.5,
              display: "flex",
              flexDirection: "column",
              gap: 0.5,
            }}
          >
            <Typography variant="caption" color="text.secondary">
              © {new Date().getFullYear()} {COMPANY_NAME}. All rights reserved.
            </Typography>
            <Box sx={{ display: "flex", flexWrap: "wrap", columnGap: 1.5, rowGap: 0.25 }}>
              <Link
                href={TERMS_OF_SERVICE_URL}
                target="_blank"
                rel="noopener noreferrer"
                variant="caption"
                color="text.secondary"
                underline="hover"
              >
                Terms & Conditions
              </Link>
              <Link
                href={PRIVACY_POLICY_URL}
                target="_blank"
                rel="noopener noreferrer"
                variant="caption"
                color="text.secondary"
                underline="hover"
              >
                Privacy Policy
              </Link>
            </Box>
          </Box>
        </Sidebar.Footer>
      )}
    </Sidebar>
  );
}
