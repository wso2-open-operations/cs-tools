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

import { Box, Skeleton, Typography } from "@wso2/oxygen-ui";
import { Clock, Plane, Server, ShieldAlert, Users } from "@wso2/oxygen-ui-icons-react";
import type { JSX, ReactNode } from "react";
import type { BeDashboardWidget } from "@api/backend/types";
import { useDashboard } from "@features/csm-dashboard/api/useDashboard";
import { groupWidgetsBySection, type WidgetGroup } from "@features/csm-dashboard/utils/dashboardWidgetGridLayout";
import WallboardPanel from "@features/csm-dashboard/components/WallboardPanel";
import WallboardCreSection from "@features/csm-dashboard/components/WallboardCreSection";
import WallboardSreSection from "@features/csm-dashboard/components/WallboardSreSection";
import WallboardStatGrid from "@features/csm-dashboard/components/WallboardStatGrid";
import WallboardSecondaryStat from "@features/csm-dashboard/components/WallboardSecondaryStat";
import {
  CS_OVERVIEW_REFETCH_INTERVAL_MS,
  resolveDisplayNameAlias,
  type WallboardSection,
} from "@features/csm-dashboard/utils/wallboardMetricStyle";

export interface WallboardDashboardProps {
  dashboardId: string;
  selectedTeamCreGroupId?: string | string[];
  selectedTeamSreGroupId?: string | string[];
  selectedTeamLabel?: string;
}

const SECTION_TITLE: Record<WallboardSection, string> = {
  cre: "Customer Reliability Engineering (CRE)",
  sre: "Site Reliability Engineering (SRE)",
  security: "Security Report",
  fde: "Forward Deployed Engineering (FDE)",
};

const SECTION_ICON: Record<WallboardSection, typeof Users> = {
  cre: Users,
  sre: Server,
  security: ShieldAlert,
  fde: Plane,
};

/** Classifies a backend-configured `section` name into one of the four
 * fixed CS Overview families by keyword match, rather than requiring the
 * backend to send an exact literal string — the same tolerant-matching
 * approach `groupWidgetsBySection` already leaves the "section" field free
 * text for. `undefined` for a section that doesn't match any of the four
 * (rendered as a plain fallback grid, never silently dropped). */
function familyFor(sectionName: string | undefined): WallboardSection | undefined {
  if (!sectionName) return undefined;
  const name = sectionName.toLowerCase();
  if (/\bcre\b|customer reliability/.test(name)) return "cre";
  if (/\bsre\b|site reliability/.test(name)) return "sre";
  if (/security/.test(name)) return "security";
  if (/\bfde\b|forward deployed/.test(name)) return "fde";
  return undefined;
}

/** The dark, full-viewport wrapper every one of this component's three
 * render states (loading / error / loaded) shares — factored out so
 * "`bgcolor: '#0f1420'`, full-viewport, 16px padding" is declared once
 * rather than repeated three times with the risk of one copy drifting
 * from the other two. `sx` merges in on top of (and can override) the
 * defaults below — the loaded state uses this to swap `minHeight` for a
 * fixed `height` plus its own flex-column layout. */
function WallboardPageFrame({
  children,
  sx,
}: {
  children: ReactNode;
  sx?: Record<string, unknown>;
}): JSX.Element {
  return <Box sx={{ bgcolor: "#0f1420", minHeight: "100dvh", p: 2, ...sx }}>{children}</Box>;
}

function LoadingSkeleton(): JSX.Element {
  return (
    <WallboardPageFrame>
      <Box sx={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 2 }}>
        {[0, 1, 2, 3].map((i) => (
          <Skeleton key={i} variant="rounded" height={260} sx={{ borderRadius: "16px", bgcolor: "rgba(255,255,255,0.06)" }} />
        ))}
      </Box>
    </WallboardPageFrame>
  );
}

function renderFallbackGrid(
  group: WidgetGroup,
  selectedTeamCreGroupId: string | string[] | undefined,
  selectedTeamSreGroupId: string | string[] | undefined,
  selectedTeamLabel: string | undefined,
): JSX.Element {
  return (
    <Box sx={{ display: "grid", gridTemplateColumns: "repeat(3, 1fr)", gap: 0.75 }}>
      {group.widgets.map((widget) => (
        <WallboardSecondaryStat
          key={widget.widgetId}
          widgetId={widget.widgetId}
          displayName={widget.displayName}
          resourceType={widget.resourceType}
          filters={widget.query}
          selectedTeamCreGroupId={selectedTeamCreGroupId}
          selectedTeamSreGroupId={selectedTeamSreGroupId}
          selectedTeamLabel={selectedTeamLabel}
        />
      ))}
    </Box>
  );
}

function renderSectionBody(
  family: WallboardSection,
  widgets: BeDashboardWidget[],
  selectedTeamCreGroupId: string | string[] | undefined,
  selectedTeamSreGroupId: string | string[] | undefined,
  selectedTeamLabel: string | undefined,
): JSX.Element {
  switch (family) {
    case "cre":
      return (
        <WallboardCreSection
          widgets={widgets}
          selectedTeamCreGroupId={selectedTeamCreGroupId}
          selectedTeamSreGroupId={selectedTeamSreGroupId}
          selectedTeamLabel={selectedTeamLabel}
        />
      );
    case "sre":
      return (
        <WallboardSreSection
          widgets={widgets}
          selectedTeamCreGroupId={selectedTeamCreGroupId}
          selectedTeamSreGroupId={selectedTeamSreGroupId}
          selectedTeamLabel={selectedTeamLabel}
        />
      );
    case "security":
      return (
        <WallboardStatGrid
          widgets={widgets}
          section="security"
          columns={2}
          selectedTeamCreGroupId={selectedTeamCreGroupId}
          selectedTeamSreGroupId={selectedTeamSreGroupId}
          selectedTeamLabel={selectedTeamLabel}
        />
      );
    case "fde":
      return (
        <WallboardStatGrid
          widgets={widgets}
          section="fde"
          columns={3}
          selectedTeamCreGroupId={selectedTeamCreGroupId}
          selectedTeamSreGroupId={selectedTeamSreGroupId}
          selectedTeamLabel={selectedTeamLabel}
        />
      );
  }
}

/**
 * The CS Overview dashboard's content — styled to match `digiops-cs`'s
 * `Wallboard.tsx` (see `CS_Dashboard.png`): a 2x2 grid of panels (CRE / SRE
 * / Security Report / FDE), each with its own accent color and internal
 * layout. Data comes from the exact same config-driven `GET
 * /dashboards/{id}` + per-widget `useWidgetData` path every other CSM
 * Portal dashboard already uses — only the rendering is different.
 */
export default function WallboardDashboard({
  dashboardId,
  selectedTeamCreGroupId,
  selectedTeamSreGroupId,
  selectedTeamLabel,
}: WallboardDashboardProps): JSX.Element {
  const { data, isLoading, isError, dataUpdatedAt } = useDashboard(dashboardId, CS_OVERVIEW_REFETCH_INTERVAL_MS);

  // `dataUpdatedAt` (React Query's own cache timestamp, not component
  // state) — not "track when `data` last changed": React Query's default
  // structural sharing keeps the SAME `data` reference across a refetch
  // that returns identical values, so a `data !== prevData` check would
  // silently skip the timestamp on a real refetch that happened to change
  // nothing. `dataUpdatedAt` updates on every successful fetch regardless,
  // and — living in the shared query cache rather than this component's
  // own state — survives switching away to another dashboard and back
  // (which fully unmounts this component) without going blank in between.
  const lastUpdated = dataUpdatedAt ? new Date(dataUpdatedAt).toLocaleTimeString() : undefined;

  if (isError) {
    return (
      <WallboardPageFrame sx={{ display: "flex", alignItems: "center", justifyContent: "center" }}>
        <Typography variant="body2" sx={{ color: "rgba(255,255,255,0.7)" }}>
          Could not load the dashboard.
        </Typography>
      </WallboardPageFrame>
    );
  }

  if (isLoading || !data) {
    return <LoadingSkeleton />;
  }

  // Alias resolution runs once here, centrally, before grouping — every
  // downstream component (WallboardCreSection, WallboardSreSection,
  // WallboardStatGrid, the emphasis lookup) then only ever sees the
  // already-canonical displayName, never the raw backend one.
  const aliasedWidgets = data.widgets
    .filter((w) => w.shape === "count")
    .map((w) => ({ ...w, displayName: resolveDisplayNameAlias(w.displayName) }));
  const groups = groupWidgetsBySection(aliasedWidgets);

  return (
    <WallboardPageFrame sx={{ minHeight: undefined, height: "100dvh", display: "flex", flexDirection: "column" }}>
      <Box sx={{ display: "flex", justifyContent: "flex-end", mb: 1.5, flexShrink: 0 }}>
        <Box
          sx={{
            display: "flex",
            alignItems: "center",
            gap: 0.75,
            fontSize: "0.65rem",
            fontWeight: 600,
            letterSpacing: "0.05em",
            textTransform: "uppercase",
            color: "#cbd5e1",
            bgcolor: "#1f2937",
            px: 1.5,
            py: 0.5,
            borderRadius: "999px",
            border: "1px solid #374151",
          }}
        >
          <Clock size={13} />
          <span>Last updated: {lastUpdated ?? "—"}</span>
          <Box
            sx={{
              width: 6,
              height: 6,
              borderRadius: "50%",
              bgcolor: "#34d399",
              "@keyframes wallboard-live-pulse": {
                "0%, 100%": { opacity: 1 },
                "50%": { opacity: 0.4 },
              },
              animation: "wallboard-live-pulse 1.6s ease-in-out infinite",
            }}
          />
        </Box>
      </Box>
      <Box
        sx={{
          display: "grid",
          gridTemplateColumns: "1fr 1fr",
          // Matches the original's own layout exactly: CRE/SRE (row 1) grow
          // to fill whatever vertical space is left after Security
          // Report/FDE (row 2) take their own natural, compact height —
          // not an even 50/50 split, and not every panel filling the
          // viewport (which would just stretch Security/FDE's own sparse
          // 2x2 grid until it looked emptier, not fill the page more
          // sensibly).
          gridTemplateRows: "1fr auto",
          gap: 2,
          flex: 1,
          minHeight: 0,
        }}
      >
        {(["cre", "sre", "security", "fde"] as const).map((family) => {
          // `section` is free-text (see `groupWidgetsBySection`'s own doc
          // comment — it preserves every distinct string as its own
          // group), so two differently-named sections can both map to the
          // same family here (e.g. "CRE Operations" and "CRE
          // Escalations", both matched by `familyFor`'s "cre" pattern).
          // Merge every matching group's widgets rather than taking only
          // the first — a lone `.find()` would silently drop the rest.
          const matchingGroups = groups.filter((g) => familyFor(g.section) === family);
          if (matchingGroups.length === 0) return null;
          const widgets = matchingGroups.flatMap((g) => g.widgets);
          return (
            <WallboardPanel key={family} section={family} title={SECTION_TITLE[family]} icon={SECTION_ICON[family]}>
              {renderSectionBody(
                family,
                widgets,
                selectedTeamCreGroupId,
                selectedTeamSreGroupId,
                selectedTeamLabel,
              )}
            </WallboardPanel>
          );
        })}
        {groups
          .filter((g) => familyFor(g.section) === undefined)
          .map((group, i) => (
            <Box key={group.section ?? `untitled-${i}`} sx={{ gridColumn: "1 / -1" }}>
              {group.section && (
                <Typography sx={{ color: "#cbd5e1", fontSize: "0.8rem", fontWeight: 700, mb: 1 }}>
                  {group.section}
                </Typography>
              )}
              {renderFallbackGrid(group, selectedTeamCreGroupId, selectedTeamSreGroupId, selectedTeamLabel)}
            </Box>
          ))}
      </Box>
    </WallboardPageFrame>
  );
}
