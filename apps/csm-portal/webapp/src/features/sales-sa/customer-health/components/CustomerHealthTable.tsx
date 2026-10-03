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

import { type ChangeEvent, type ReactElement } from "react";
import { useNavigate } from "react-router";
import {
  Table, TableBody, TableCell, TableContainer, TableHead, TableRow, Paper, Typography, Box,
  TablePagination, Chip, Button, CircularProgress, Tooltip,
} from "@wso2/oxygen-ui";
import { DownloadIcon, InfoIcon, CircleAlertIcon } from "@wso2/oxygen-ui-icons-react";
import { colors, alpha, useTheme } from "@wso2/oxygen-ui";
import { useCustomerHealthSummary } from "../api/useCustomerHealthSummary";
import { TOOLTIP_TEXT, type AccountSummary, type RiskFilterKey } from "../api/customerHealthTypes";

const { green, red, amber } = colors;

type CustomerHealthTableProps = {
  accountScope: "my-accounts" | "all-accounts";
  userEmail: string | null;
  searchQuery: string;
  riskIndicators: RiskFilterKey[];
  region: string[];
  product: string | null;
  abtTeam: string | null;
  healthStatus: string | null;
  page: number;
  rowsPerPage: number;
  onPageChange: (newPage: number) => void;
  onRowsPerPageChange: (newRowsPerPage: number) => void;
  onExport: () => void;
  exporting: boolean;
};

export default function CustomerHealthTable(props: CustomerHealthTableProps): ReactElement {
  const theme = useTheme();
  const navigate = useNavigate();

  const { data, isLoading: loading, isError: error } = useCustomerHealthSummary({
    offset: props.page * props.rowsPerPage,
    limit: props.rowsPerPage,
    email: props.accountScope === "my-accounts" && props.userEmail ? props.userEmail : "",
    phrase: props.searchQuery && props.searchQuery.length >= 2 ? props.searchQuery : "",
    risks: Array.isArray(props.riskIndicators) && props.riskIndicators.length > 0 ? props.riskIndicators.join(",") : "",
    region: Array.isArray(props.region) && props.region.length > 0 ? props.region : [],
    product: props.product || "",
    abtTeam: props.abtTeam || "",
    healthStatus: props.healthStatus || "",
  });

  const accounts = data?.data || [];
  const totalCount = data?.totalCount || 0;

  const handleChangePage = (_event: unknown, newPage: number) => {
    props.onPageChange(newPage);
  };

  const handleChangeRowsPerPage = (event: ChangeEvent<HTMLInputElement>) => {
    props.onRowsPerPageChange(parseInt(event.target.value, 10));
  };

  const handleAccountTableRowClick = (rowData: AccountSummary) => {
    const localIndex = accounts.findIndex((a) => a.accountSysId === rowData.accountSysId);
    navigate(`/spl/customer-health/account/${rowData.accountSysId}`, {
      state: {
        accountList: accounts.map((a) => ({ accountSysId: a.accountSysId, accountName: a.accountName })),
        currentIndex: localIndex,
        absoluteIndex: props.page * props.rowsPerPage + localIndex,
        totalCount,
        filterPayload: {
          email: props.accountScope === "my-accounts" && props.userEmail ? props.userEmail : "",
          phrase: props.searchQuery && props.searchQuery.length >= 2 ? props.searchQuery : "",
          risks: Array.isArray(props.riskIndicators) && props.riskIndicators.length > 0 ? props.riskIndicators.join(",") : "",
          region: Array.isArray(props.region) && props.region.length > 0 ? props.region : [],
          product: props.product || "",
          abtTeam: props.abtTeam || "",
          healthStatus: props.healthStatus || "",
        },
      },
    });
  };

  if (loading) return <CircularProgress size={20} sx={{ mt: 2 }} />;

  return (
    <>
      <Box sx={{ my: 2 }}>
        <Typography variant="h6">Accounts Overview</Typography>
      </Box>
      <Box sx={{ display: "flex", justifyContent: "space-between", alignItems: "center", mb: 1 }}>
        <Typography variant="caption">
          {totalCount} accounts found
          {props.healthStatus === "at_risk" && " · Filtered by: At Risk"}
          {props.healthStatus === "healthy" && " · Filtered by: Healthy"}
          {!props.healthStatus && " at risk"}.
        </Typography>
        <Button
          variant="contained"
          size="small"
          startIcon={props.exporting ? <CircularProgress size={14} color="inherit" /> : <DownloadIcon size={16} />}
          onClick={props.onExport}
          disabled={props.exporting || totalCount === 0}
          sx={{ textTransform: "none", backgroundColor: "#e96900", color: "white", "&:hover": { backgroundColor: "#c45a00", color: "white" }, "&:disabled": { backgroundColor: alpha("#e96900", 0.3), color: "white" } }}
        >
          {props.exporting ? "Exporting…" : "Export CSV"}
        </Button>
      </Box>

      <Paper>
        <TableContainer sx={{ overflowX: "auto" }}>
          <Table stickyHeader aria-label="customer health summary table" sx={{ minWidth: 1200 }}>
            <TableHead>
              <TableRow>
                <TableCell
                  rowSpan={2}
                  sx={{
                    verticalAlign: "middle", whiteSpace: "nowrap", position: "sticky", left: 0, zIndex: 4,
                    backgroundColor: "action.hover", "&.MuiTableCell-stickyHeader": { backgroundColor: theme.palette.action.hover, zIndex: 4 },
                    borderBottom: `1px solid ${theme.palette.divider}`, boxShadow: "2px 0 4px rgba(0,0,0,0.06)", py: 1.5, px: 2,
                  }}
                >
                  Account Name
                </TableCell>
                <TableCell
                  rowSpan={2}
                  align="center"
                  sx={{ verticalAlign: "middle", backgroundColor: "action.hover", "&.MuiTableCell-stickyHeader": { backgroundColor: theme.palette.action.hover }, borderBottom: `1px solid ${theme.palette.divider}`, py: 1.5, px: 2 }}
                >
                  <HeaderTooltip label="Health Status" text={TOOLTIP_TEXT.healthStatus} />
                </TableCell>
                <TableCell
                  colSpan={6}
                  align="center"
                  sx={{ backgroundColor: "action.hover", "&.MuiTableCell-stickyHeader": { backgroundColor: theme.palette.action.hover }, color: "text.secondary", fontWeight: 500, fontSize: "0.72rem", letterSpacing: "1px", textTransform: "uppercase", borderBottom: `1px solid ${theme.palette.divider}`, py: 0.75, px: 2 }}
                >
                  Risk Indicators
                </TableCell>
              </TableRow>
              <TableRow>
                {[
                  { label: "Has Gone Live", tip: TOOLTIP_TEXT.goLive },
                  { label: "Support Activity (Last 6 Mo)", tip: TOOLTIP_TEXT.support },
                  { label: "Using EOL Products", tip: TOOLTIP_TEXT.eol },
                  { label: "Abandoned Migrations", tip: TOOLTIP_TEXT.abandoned },
                  { label: "Migration Delays", tip: TOOLTIP_TEXT.delays },
                  { label: "Escalations (Last 3 Mo)", tip: TOOLTIP_TEXT.escalations },
                ].map(({ label, tip }) => (
                  <TableCell key={label} align="center" sx={{ whiteSpace: "nowrap", backgroundColor: "action.hover", "&.MuiTableCell-stickyHeader": { backgroundColor: theme.palette.action.hover }, borderBottom: `1px solid ${theme.palette.divider}`, py: 1.5, px: 3 }}>
                    <HeaderTooltip label={label} text={tip} />
                  </TableCell>
                ))}
              </TableRow>
            </TableHead>
            <TableBody>
              {error ? (
                <TableRow>
                  <TableCell colSpan={8} sx={{ textAlign: "center", py: 5 }}>
                    <Box sx={{ display: "flex", flexDirection: "column", alignItems: "center", gap: 1 }}>
                      <Box sx={{ color: "text.disabled", display: "inline-flex" }}>
                        <CircleAlertIcon size={40} />
                      </Box>
                      <Typography variant="subtitle1" color="textSecondary" fontWeight="bold">
                        Something happened while fetching the accounts.
                      </Typography>
                      <Typography variant="body2" color="textSecondary">
                        Please refresh the page and try again later.
                      </Typography>
                    </Box>
                  </TableCell>
                </TableRow>
              ) : (
                accounts?.map((acc) => (
                  <TableRow
                    key={acc.accountSysId}
                    sx={{ "&:last-child td, &:last-child th": { border: 0 }, cursor: "pointer", "&:hover": { backgroundColor: "action.hover" } }}
                    onClick={() => handleAccountTableRowClick(acc)}
                  >
                    <TableCell component="th" scope="row" sx={{ position: "sticky", left: 0, zIndex: 1, backgroundColor: "background.paper", boxShadow: "2px 0 4px rgba(0,0,0,0.06)", whiteSpace: "nowrap" }}>
                      <Box component="span" sx={{ color: "#e96900", textDecoration: "none", "&:hover": { textDecoration: "underline" } }}>
                        {acc.accountName || `Account ID: ${acc.accountSysId.substring(0, 8)}...`}
                      </Box>
                    </TableCell>
                    <TableCell align="center">
                      <ReviewStatusBadge status={acc.healthStatus} />
                    </TableCell>
                    <TableCell align="center" sx={{ px: 3 }}><StatusIndicator isRisk={acc.hasNoGoLive.isRisk} state={acc.hasNoGoLive.state} /></TableCell>
                    <TableCell align="center" sx={{ px: 3 }}><StatusIndicator isRisk={acc.noSupportCases6mo} /></TableCell>
                    <TableCell align="center" sx={{ px: 3 }}><StatusIndicator isRisk={acc.hasEolProduct} riskShowsYes /></TableCell>
                    <TableCell align="center" sx={{ px: 3 }}><StatusIndicator isRisk={acc.hasAbandonedMigrations} riskShowsYes /></TableCell>
                    <TableCell align="center" sx={{ px: 3 }}><StatusIndicator isRisk={acc.hasMigrationDelays} riskShowsYes /></TableCell>
                    <TableCell align="center" sx={{ px: 3 }}><StatusIndicator isRisk={acc.hasRecentEscalations} riskShowsYes /></TableCell>
                  </TableRow>
                ))
              )}
            </TableBody>
          </Table>
        </TableContainer>

        <TablePagination
          rowsPerPageOptions={[5, 10, 25]}
          component="div"
          count={totalCount}
          rowsPerPage={props.rowsPerPage}
          page={props.page}
          onPageChange={handleChangePage}
          onRowsPerPageChange={handleChangeRowsPerPage}
          showFirstButton
          showLastButton
          sx={{
            borderTop: `1px solid ${theme.palette.divider}`,
            "& .MuiTablePagination-toolbar": { alignItems: "center", justifyContent: "flex-start", paddingLeft: "16px" },
            "& .MuiTablePagination-spacer": { display: "none" },
            "& .MuiTablePagination-selectLabel": { fontSize: "0.875rem", fontWeight: 500, color: "text.primary", margin: "0 8px 0 0" },
            "& .MuiTablePagination-select": { fontSize: "0.875rem", margin: "0 6px 0 2px", border: "none", borderRadius: "4px", padding: "4px 8px", minWidth: "50px" },
            "& .MuiTablePagination-displayedRows": { fontSize: "0.875rem", fontWeight: 500, color: "text.primary", margin: "0 12px 0 0" },
            "& .MuiTablePagination-actions": { marginLeft: "4px" },
          }}
        />
        {accounts?.length === 0 && !loading && (
          <Typography sx={{ p: 3, textAlign: "center" }}>No accounts found matching the criteria.</Typography>
        )}
      </Paper>
    </>
  );
}

function ReviewStatusBadge({ status }: { status?: string }) {
  const chipSx = {
    fontWeight: 600, fontSize: "0.8rem", borderRadius: "16px", backgroundColor: "transparent",
    border: "2px solid", minWidth: 120, height: 28, "& .MuiChip-label": { px: 1.5, textAlign: "center", width: "100%" },
  };
  if (!status || status === "to_be_reviewed") {
    return <Chip label="To Be Reviewed" sx={{ ...chipSx, borderColor: "#E65100", color: "#E65100", "&:hover": { backgroundColor: "transparent" } }} />;
  }
  if (status === "at_risk") {
    return <Chip label="At Risk" sx={{ ...chipSx, borderColor: "#C62828", color: "#C62828", "&:hover": { backgroundColor: "transparent" } }} />;
  }
  if (status === "healthy") {
    return <Chip label="Healthy" sx={{ ...chipSx, borderColor: "#2E7D32", color: "#2E7D32", "&:hover": { backgroundColor: "transparent" } }} />;
  }
  return null;
}

// By default the letter answers the positively-phrased column label (e.g. "Has Gone
// Live"): a risk shows "N". For columns phrased as the risk condition itself (e.g.
// "Using EOL Products"), pass riskShowsYes so a risk shows "Y" instead - the color
// (red on risk, green otherwise) never changes, only which letter reflects that state.
function StatusIndicator({ isRisk, state, riskShowsYes = false }: { isRisk: boolean; state?: string; riskShowsYes?: boolean }) {
  if (!isRisk && state === "pending") {
    return (
      <Tooltip title="Not Live Yet" arrow placement="top">
        <Chip label="P" size="small" sx={{ backgroundColor: amber[600], color: "#fff", fontWeight: 700, minWidth: 32 }} />
      </Tooltip>
    );
  }

  if (state === "failure" || isRisk === true) {
    return <Chip label={riskShowsYes ? "Y" : "N"} size="small" sx={{ backgroundColor: red[500], color: "#fff", fontWeight: 700, minWidth: 32 }} />;
  }

  return <Chip label={riskShowsYes ? "N" : "Y"} size="small" sx={{ backgroundColor: green[500], color: "#fff", fontWeight: 700, minWidth: 32 }} />;
}

function HeaderTooltip({ label, text }: { label: string; text: string }) {
  return (
    <Box component="span" sx={{ display: "inline-flex", alignItems: "center", gap: "4px", verticalAlign: "middle", whiteSpace: "nowrap" }}>
      <Typography component="span" sx={{ fontWeight: 600, fontSize: "0.85rem", color: "text.primary" }}>
        {label}
      </Typography>
      <Tooltip title={text} arrow placement="top">
        <Box component="span" sx={{ display: "inline-flex", color: "text.secondary", cursor: "help", "&:hover": { color: "text.primary" } }}>
          <InfoIcon size={16} />
        </Box>
      </Tooltip>
    </Box>
  );
}
