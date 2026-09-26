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

import type { ReactNode } from "react";
import { Box, MenuItem } from "@mui/material";
import { Outlet, useMatch } from "react-router";
import { useOverview } from "@api/hooks";
import { FetchProgressBar, FetchProgressProvider } from "@components/FetchProgressBar";
import { FilterSelect } from "@components/FilterSelect";
import { IssueSearchBox } from "@components/IssueSearchBox";
import { SyncButton } from "@components/SyncButton";
import { UserProfile } from "@components/UserProfile";
import { useFetchProgressActive } from "@lib/fetchProgress";
import { priorityOptionsFrom, useGlobalFilters } from "@lib/filters";

/**
 * The header's global Project/Priority/ABT Team selects, plus the filtered
 * `useOverview` call that backs their option lists. Rendered only off
 * `/issues`, whose own filter dropdowns already own those URL keys and fetch
 * overview unfiltered — mounting this here too would fire a second,
 * redundant (and differently-scoped) overview request on that page.
 */
function GlobalFilterSelects() {
  const { repo, priority, abtTeam, setFilter } = useGlobalFilters();
  const { data: overview } = useOverview({ repo, priority, abtTeam });

  const repoOptions = overview?.projects ?? [];
  const priorityOptions = priorityOptionsFrom(overview?.priorities);
  const abtTeamOptions = overview?.abtTeams ?? [];

  return (
    <Box sx={{ display: "flex", alignItems: "center", gap: "10px" }}>
      <FilterSelect value={repo ?? "all"} onChange={(v) => setFilter("repo", v === "all" ? "" : v)}>
        <MenuItem value="all">All Projects</MenuItem>
        {repoOptions.map((r) => (
          <MenuItem key={r.repoId} value={r.repo}>
            {r.name}
          </MenuItem>
        ))}
      </FilterSelect>

      <FilterSelect value={priority ?? "all"} onChange={(v) => setFilter("priority", v === "all" ? "" : v)}>
        <MenuItem value="all">All Priorities</MenuItem>
        {priorityOptions.map((p) => (
          <MenuItem key={p.value} value={p.value}>
            {p.label}
          </MenuItem>
        ))}
      </FilterSelect>

      <FilterSelect value={abtTeam ?? "all"} onChange={(v) => setFilter("abtTeam", v === "all" ? "" : v)}>
        <MenuItem value="all">All ABT Teams</MenuItem>
        {abtTeamOptions.map((team) => (
          <MenuItem key={team} value={team}>
            {team}
          </MenuItem>
        ))}
      </FilterSelect>
    </Box>
  );
}

/** The WSO2 pulse mark shown in the top nav bar. */
function Logo() {
  return <Box component="img" src="/wso2-pulse.svg" alt="WSO2" sx={{ height: 34, width: 34, display: "block" }} />;
}

/** The signed-in app frame: top nav with global repo/priority/ABT-team filters, routed content below. */
export default function AppShell({ children }: { children?: ReactNode }) {
  return (
    <FetchProgressProvider>
      <AppShellContent>{children}</AppShellContent>
    </FetchProgressProvider>
  );
}

function AppShellContent({ children }: { children?: ReactNode }) {
  const progressActive = useFetchProgressActive();
  const onIssuesPage = useMatch("/issues");

  return (
    // No bgcolor here — Oxygen UI's MuiCssBaseline override paints the
    // Acrylic radial-gradient body backdrop (see theme/global.css); an
    // opaque background on this wrapper would hide it completely.
    <Box sx={{ minHeight: "100vh", color: "var(--sla-fg)" }}>
      <FetchProgressBar active={progressActive} />
      <Box
        component="header"
        sx={{
          position: "sticky",
          top: 0,
          zIndex: 20,
          borderBottom: "1px solid var(--sla-border)",
          bgcolor: "var(--sla-card)",
          backdropFilter: "var(--sla-blur) saturate(150%)",
          WebkitBackdropFilter: "var(--sla-blur) saturate(150%)",
        }}
      >
        <Box
          sx={{
            mx: "auto",
            maxWidth: 1480,
            display: "flex",
            flexWrap: "wrap",
            alignItems: "center",
            gap: "20px",
            px: "28px",
            py: "14px",
          }}
        >
          <Box sx={{ mr: "auto", display: "flex", alignItems: "center", gap: 1.5 }}>
            <Logo />
            <Box>
              <Box sx={{ fontSize: 16, fontWeight: 600, letterSpacing: "-0.01em" }}>Git Internals Dashboard</Box>
              <Box sx={{ fontSize: 12, color: "var(--sla-fg3)" }}>
                For GitHub issues Opened by CS Team
              </Box>
            </Box>
          </Box>

          {onIssuesPage ? <IssueSearchBox /> : <GlobalFilterSelects />}

          <Box sx={{ display: "flex", alignItems: "center", gap: "10px", pl: 1 }}>
            <SyncButton />
            <UserProfile />
          </Box>
        </Box>
      </Box>

      <Box component="main" sx={{ mx: "auto", maxWidth: 1480, px: "28px", pb: "60px", pt: 3 }}>
        {children ?? <Outlet />}
      </Box>
    </Box>
  );
}
