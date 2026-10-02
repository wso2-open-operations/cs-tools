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
// features/spl/cases/pages/CasesPage.tsx — an overview of six case-state
// counts that drills into a per-state table. Six separate query hooks with
// per-state offset/rowsPerPage state (rather than one query re-fetched on
// filter change) is the source's own pattern, kept as-is: it's what avoids
// a pagination change on one state's table re-triggering every other
// state's count card.
//
// No SplShell wrapper here (unlike the source): RouteGuard (App.tsx)
// already gates the whole /spl/* route tree and mounts
// PermissionProvider above this page. No PageHeader either (this app has
// no equivalent helper) — a plain Typography stands in.
import { useState } from "react";
import { Box, Grid, Typography } from "@wso2/oxygen-ui";
import { useGetCases } from "../api/useCases";
import Search from "../components/Search";
import CaseStateCard from "../components/CaseStateCard";
import CaseStateView from "../components/CaseStateView";
import type { CaseDetailsWithCount } from "../api/caseTypes";

const STATES = ["Open", "Work In Progress", "Awaiting Info", "Solution Proposed", "Waiting on WSO2", "Reopened"];

function useCaseStateQuery(state: string) {
  const [page, setPage] = useState(0);
  const [rowsPerPage, setRowsPerPage] = useState(10);
  const { data, isLoading, error } = useGetCases(state, page * rowsPerPage, rowsPerPage);
  return { data, loading: isLoading, error, page, setPage, rowsPerPage, setRowsPerPage };
}

export default function CasesPage() {
  const [caseState, setCaseState] = useState("");
  const [showTable, setShowTable] = useState(true);

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
        <Box sx={{ mt: 5, width: "100%", maxWidth: 1050, mx: "auto" }}>
          <Typography align="left" gutterBottom variant="h6">
            Overall Case Summary
          </Typography>
          <Grid container spacing={3} justifyContent="center" sx={{ mt: 4 }}>
            {STATES.map((state, i) => (
              <Grid key={state} size={{ xs: 12, sm: 6, md: 4 }} sx={{ display: "flex", justifyContent: "center" }}>
                <CaseStateCard
                  data={queries[i].data}
                  loading={queries[i].loading}
                  error={queries[i].error}
                  state={state}
                  setCaseState={setCaseState}
                />
              </Grid>
            ))}
          </Grid>
        </Box>
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
