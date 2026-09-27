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

// Ported from apps/support-portal-lite/webapp's own
// features/spl/cases/pages/SplCasesPage.tsx — an overview of six case-state
// counts that drills into a per-state table. Six separate query hooks with
// per-state offset/rowsPerPage state (rather than one query re-fetched on
// filter change) is the source's own pattern, kept as-is: it's what avoids
// a pagination change on one state's table re-triggering every other
// state's count card.
//
// No SplShell wrapper here (unlike the source): SplRouteGuard (App.tsx)
// already gates the whole /spl/* route tree and mounts
// SplPermissionProvider above this page. No PageHeader either (this app has
// no equivalent helper) — a plain Typography stands in.
import { useState } from "react";
import { Box, Grid, Paper, Typography } from "@wso2/oxygen-ui";
import { useTheme, useColorScheme } from "@mui/material/styles";
import { useGetSplCases } from "../api/useSplCases";
import Search from "../components/Search";
import CaseStateCard from "../components/CaseStateCard";
import CaseStateView from "../components/CaseStateView";
import type { CaseDetailsWithCount } from "../api/splCaseTypes";

const STATES = ["Open", "Work In Progress", "Awaiting Info", "Solution Proposed", "Waiting on WSO2", "Reopened"];
// Per state, a {light, dark} pair rather than one static hex — the count's
// own uniform grey card background (see CaseStateCard) sits at a different
// brightness in each mode, and a single color can't have good contrast
// against both: a pale pink or a slate grey close to the card's own tone
// (the previous flat COLORS values) read as nearly invisible against a
// light-grey card, a dark-grey one, or both. Each pair here is a light-mode
// "800"-ish shade and a dark-mode "200"/"400"-ish shade of the same hue,
// the standard MUI convention for text-on-tinted-surface contrast.
const COLORS: { light: string; dark: string }[] = [
  { light: "#0052CC", dark: "#4C9AFF" }, // Open — blue
  { light: "#2E7D32", dark: "#66BB6A" }, // Work In Progress — green
  { light: "#AD1457", dark: "#F48FB1" }, // Awaiting Info — pink/magenta
  { light: "#5D4037", dark: "#BCAAA4" }, // Solution Proposed — brown
  { light: "#D84315", dark: "#FF8A65" }, // Waiting on WSO2 — deep orange
  { light: "#37474F", dark: "#B0BEC5" }, // Reopened — blue-grey
];

function useCaseStateQuery(state: string) {
  const [page, setPage] = useState(0);
  const [rowsPerPage, setRowsPerPage] = useState(10);
  const { data, isLoading, error } = useGetSplCases(state, page * rowsPerPage, rowsPerPage);
  return { data, loading: isLoading, error, page, setPage, rowsPerPage, setRowsPerPage };
}

export default function SplCasesPage() {
  const [caseState, setCaseState] = useState("");
  const [showTable, setShowTable] = useState(true);
  const theme = useTheme();
  // theme.palette.mode is NOT live here: oxygen-ui's theme is built with
  // extendTheme() (MUI's CSS-variables system), where a plain useTheme()
  // call returns the theme's static reference mode, not the mode actually
  // showing on screen — confirmed empirically (it kept resolving "light"
  // with data-color-scheme="dark" set). useColorScheme() is the hook that
  // actually tracks the live scheme.
  const { mode: colorMode, systemMode } = useColorScheme();
  const isDark = (colorMode === "system" ? systemMode : colorMode) === "dark";

  // Called unconditionally, one per fixed state — see file header comment.
  const q0 = useCaseStateQuery(STATES[0]);
  const q1 = useCaseStateQuery(STATES[1]);
  const q2 = useCaseStateQuery(STATES[2]);
  const q3 = useCaseStateQuery(STATES[3]);
  const q4 = useCaseStateQuery(STATES[4]);
  const q5 = useCaseStateQuery(STATES[5]);
  const queries = [q0, q1, q2, q3, q4, q5];

  return (
    <Box sx={{ p: 3 }}>
      <Typography variant="h4" sx={{ mb: 2 }}>
        Cases
      </Typography>
      <Box>
        <Search searchOption="case" setShowTable={setShowTable} />
      </Box>
      {caseState === "" && showTable && (
        <Grid container spacing={3} sx={{ p: 2, mt: 4 }}>
          <Grid size={{ xs: 12 }} sx={{ display: "flex", justifyContent: "center" }}>
            <Paper
              sx={{
                backgroundColor: isDark ? theme.palette.grey[900] : "#ECECEC",
                maxWidth: 1050,
              }}
            >
              <Grid container p={3}>
                <Grid size={{ xs: 12 }}>
                  <Typography align="left" gutterBottom variant="h4">
                    Overall Case Summary
                  </Typography>
                </Grid>
                {STATES.map((state, i) => (
                  <Grid key={state} size={{ xs: 4 }} p={2} sx={{ display: "flex", justifyContent: "center" }}>
                    <CaseStateCard
                      data={queries[i].data}
                      loading={queries[i].loading}
                      error={queries[i].error}
                      state={state}
                      setCaseState={setCaseState}
                      color={COLORS[i]}
                    />
                  </Grid>
                ))}
              </Grid>
            </Paper>
          </Grid>
        </Grid>
      )}
      {STATES.map((state, i) =>
        caseState === state && showTable && queries[i].data ? (
          <CaseStateView
            key={state}
            data={queries[i].data as CaseDetailsWithCount}
            loading={queries[i].loading}
            error={queries[i].error}
            state={state}
            setCaseState={setCaseState}
            page={queries[i].page}
            setPage={queries[i].setPage}
            rowsPerPage={queries[i].rowsPerPage}
            setRowsPerPage={queries[i].setRowsPerPage}
          />
        ) : null,
      )}
    </Box>
  );
}
