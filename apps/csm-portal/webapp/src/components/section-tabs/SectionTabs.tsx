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

import { Box, Chip, Tab, Tabs } from "@wso2/oxygen-ui";
import { type JSX } from "react";
import { Navigate, useLocation } from "react-router";
import type { SectionTabsState } from "@hooks/useSectionTabs";
import {
  enabledPathTabKeys,
  firstEnabledPathTab,
  firstEnabledTabHref,
} from "@hooks/useSectionTabs";

interface SectionTabsProps extends SectionTabsState {
  /** Accessible name for the strip, e.g. "Operations tabs". */
  ariaLabel: string;
  scrollable?: boolean;
  /**
   * Visual weight. `"primary"` (default) is a section's own tab strip.
   * `"secondary"` renders smaller and indented, for a strip that belongs to
   * one of those tabs rather than to the section itself — e.g. Settings'
   * "User management" tab has its own row of sub-tabs underneath the primary
   * strip.
   */
  variant?: "primary" | "secondary";
}

/**
 * A section's tab strip, rendered from the navigation tree. Also doubles as a
 * nested tab's own strip via `variant="secondary"` — the underlying data
 * (`useRouteTabs`/`useQueryTabs`) is already resolved per nav-node id, so a
 * second level is just a second `<SectionTabs>` fed by a second hook call,
 * not a different component.
 *
 * A tab the deployment marked WIP stays in the strip but is disabled and
 * chipped, so the section still advertises what is coming without offering a
 * dead panel; a hidden tab never reaches this component. A tab that's only
 * visible because the signed-in admin switched it on in their own Feature
 * Preview dialog (`tab.previewedOnly`) stays enabled and clickable, but still
 * gets its own colourful "Preview" chip, so previewing admin never mistakes
 * "I can see this" for "this is live for everyone". Renders nothing when the
 * section has no visible tabs at all, which only happens if every tab is
 * hidden by config.
 */
export default function SectionTabs({
  tabs,
  activeKey,
  select,
  ariaLabel,
  scrollable = false,
  variant = "primary",
}: SectionTabsProps): JSX.Element | null {
  if (tabs.length === 0) return null;
  const isSecondary = variant === "secondary";

  return (
    <Box
      sx={{
        borderBottom: 1,
        borderColor: "divider",
        ...(isSecondary && { pl: 2 }),
      }}
    >
      <Tabs
        aria-label={ariaLabel}
        value={activeKey}
        onChange={(_, key: string) => select(key)}
        variant={scrollable ? "scrollable" : "standard"}
        scrollButtons={scrollable ? "auto" : false}
        sx={
          isSecondary
            ? {
                minHeight: 36,
                "& .MuiTab-root": {
                  minHeight: 36,
                  paddingTop: 0.5,
                  paddingBottom: 0.5,
                  fontSize: "0.8125rem",
                },
              }
            : undefined
        }
      >
        {tabs.map((tab) =>
          tab.state === "wip" ? (
            // The chip carries the "not ready yet" message on its own: a
            // Tooltip would never fire here, because a disabled MUI Tab drops
            // pointer events for its whole subtree.
            <Tab
              key={tab.key}
              value={tab.key}
              disabled
              label={
                <Box
                  component="span"
                  sx={{ display: "flex", alignItems: "center", gap: 1 }}
                >
                  {tab.label}
                  <Chip
                    size="small"
                    label="WIP"
                    color="warning"
                    variant="outlined"
                    sx={{ height: 18, fontSize: 10 }}
                  />
                </Box>
              }
            />
          ) : tab.previewedOnly ? (
            // Unlike the WIP chip, this tab is fully enabled -- only the chip
            // itself marks it as not generally available yet. A solid, filled
            // chip in the theme's own primary (button) colour -- the same
            // colour the app's primary actions use -- so it reads as a
            // distinct, eye-catching "just you" signal rather than another
            // flavour of WIP.
            <Tab
              key={tab.key}
              value={tab.key}
              label={
                <Box
                  component="span"
                  sx={{ display: "flex", alignItems: "center", gap: 1 }}
                >
                  {tab.label}
                  <Chip
                    size="small"
                    label="Preview"
                    color="primary"
                    variant="filled"
                    sx={{ height: 18, fontSize: 10, fontWeight: 600 }}
                  />
                </Box>
              }
            />
          ) : (
            <Tab key={tab.key} value={tab.key} label={tab.label} />
          ),
        )}
      </Tabs>
    </Box>
  );
}

/**
 * Index-route element for a tabbed section: sends the user to its first usable
 * tab rather than to a hardcoded one that this deployment may have restricted.
 * Falls back to the dashboard when every tab is hidden.
 */
export function SectionIndexRedirect({
  sectionId,
}: {
  sectionId: string;
}): JSX.Element {
  return <Navigate to={firstEnabledTabHref(sectionId) ?? "/dashboard"} replace />;
}

/**
 * Index-route element for a `usePathSectionTabs` section (Operations,
 * Security Center — see `App.tsx`'s `operations`/`security-center` routes).
 * Handles both ways someone can land on the section's bare path:
 *
 * - `/operations` with no query — sends them to the first usable tab, same
 *   intent as `SectionIndexRedirect` above, just landing on a real path
 *   segment (`/operations/incidents`) instead of a `?tab=` href.
 * - `/operations?tab=incidents` — the OLD `?tab=` form these two sections used
 *   before they got their own path segments. A link in this shape may already
 *   be shared, bookmarked, or pinned, so it's translated rather than left to
 *   404 or silently drop the requested tab: the legacy value is converted to
 *   its path-segment form (see `pathTabKey` in `useSectionTabs.ts`) and used
 *   if it names a real, currently-usable tab; an unrecognised or restricted
 *   one falls through to the same first-usable-tab default as the bare-path
 *   case. Every other existing search param and the hash both carry through
 *   unchanged onto the new URL.
 */
export function LegacyQueryTabRedirect({
  sectionId,
  basePath,
}: {
  sectionId: string;
  basePath: string;
}): JSX.Element {
  const location = useLocation();
  const params = new URLSearchParams(location.search);
  const legacyTab = params.get("tab");
  const requestedKey = legacyTab ? legacyTab.replace(/_/g, "-") : undefined;
  const enabledKeys = enabledPathTabKeys(sectionId);
  // Only honour the requested key when it names a tab this deployment
  // actually offers — a restricted or unknown legacy `?tab=` value falls
  // through to the same first-usable-tab default the bare-path case uses,
  // instead of redirecting onto a dead tab. Mirrors `resolveActiveKey`'s own
  // "requested tab only if enabled, else the first enabled one" rule.
  const targetKey =
    requestedKey && enabledKeys.includes(requestedKey)
      ? requestedKey
      : (firstEnabledPathTab(sectionId) ?? undefined);

  params.delete("tab");
  const rest = params.toString();

  if (!targetKey) {
    return (
      <Navigate
        to={`/dashboard${rest ? `?${rest}` : ""}${location.hash}`}
        replace
      />
    );
  }

  return (
    <Navigate
      to={`${basePath}/${targetKey}${rest ? `?${rest}` : ""}${location.hash}`}
      replace
    />
  );
}
