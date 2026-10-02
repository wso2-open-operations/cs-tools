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
// features/spl/cases/components/CaseStateView.tsx. Route updated to this
// app's "/spl/cases/:id" (see csmNavItems.ts).
import { Box, Button, Grid, Paper, Typography } from "@wso2/oxygen-ui";
import { HomeIcon } from "@wso2/oxygen-ui-icons-react";
import { useNavigate } from "react-router";
import type { CaseDetails, CaseDetailsWithCount } from "../api/caseTypes";
import DefaultTable from "./DefaultTable";

export default function CaseStateView({
  setCaseState,
  state,
  data,
  loading,
  error,
  page,
  setPage,
  rowsPerPage,
  setRowsPerPage,
}: {
  setCaseState: (id: string) => void;
  state: string;
  data: CaseDetailsWithCount;
  loading: boolean;
  error: Error | null | undefined;
  page: number;
  setPage: (page: number) => void;
  rowsPerPage: number;
  setRowsPerPage: (rowsPerPage: number) => void;
}) {
  const navigate = useNavigate();

  const colNameArray = ["Number", "Case ID", "Short Description", "Case Type", "Priority", "State"];
  const colAttributeArray = ["number", "caseId", "shortDescription", "caseType", "priority", "state"];

  const handleRowClick = (rowData: CaseDetails) => navigate(`/spl/cases/${rowData.id}`);

  return (
    <>
      <Grid container>
        <Grid size={{ xs: 12 }}>
          <Box display="flex" justifyContent="flex-start" p={1}>
            <Button
              variant="contained"
              sx={{ color: "white", ":hover": { bgcolor: "#e96900", borderColor: "primary.main", color: "white" } }}
              onClick={() => setCaseState("")}
              startIcon={<HomeIcon size={16} />}
            >
              Home
            </Button>
          </Box>
        </Grid>
      </Grid>
      <Paper>
        <Grid container>
          <Grid size={{ xs: 4 }} />
          <Grid size={{ xs: 4 }}>
            <Typography p={1} align="center" gutterBottom variant="h4">
              {state} Cases
            </Typography>
          </Grid>
          <Grid size={{ xs: 4 }} />
        </Grid>
        <DefaultTable
          data={data}
          loading={loading}
          error={error}
          page={page}
          setPage={setPage}
          rowsPerPage={rowsPerPage}
          setRowsPerPage={setRowsPerPage}
          colNameArray={colNameArray}
          colAttributeArray={colAttributeArray}
          handleRowClick={(row) => handleRowClick(row as CaseDetails)}
        />
      </Paper>
    </>
  );
}
