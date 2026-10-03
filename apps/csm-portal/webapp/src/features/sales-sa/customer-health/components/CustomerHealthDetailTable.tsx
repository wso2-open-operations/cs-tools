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

import { useState, useEffect, type ReactNode } from "react";
import {
  Table, TableBody, TableCell, TableContainer, TableHead, TableRow, Paper, Typography, Box,
  List, ListItem, ListItemText, Collapse, IconButton, Card, CardContent,
  Accordion, AccordionSummary, AccordionDetails, Chip, CircularProgress,
} from "@wso2/oxygen-ui";
import { TriangleAlertIcon, ChevronDownIcon, ChevronUpIcon, CircleCheckIcon, CircleXIcon } from "@wso2/oxygen-ui-icons-react";
import { colors, alpha, useTheme } from "@wso2/oxygen-ui";
import { useAccountHealthDetail, useAccountHealthStatus, useInitHealthTracking, isNotFoundError } from "../api/useAccountHealthDetail";
import ReviewStatusCell from "./ReviewStatusCell";
import ActionItemsSection from "./ActionItemsSection";
import type { ProjectDetail, CaseGroup, CaseLink, EolProduct } from "../api/customerHealthTypes";

const { green, red } = colors;

interface RiskCellProps {
  project: ProjectDetail;
  indicator: keyof ProjectDetail;
  detailData?: unknown[];
  renderDetailItem: (item: never, index: number) => ReactNode;
}

function RiskCell({ project, indicator, detailData, renderDetailItem }: RiskCellProps) {
  const [open, setOpen] = useState(false);
  const hasDetails = (detailData?.length || 0) > 0;

  let statusIcon: ReactNode = null;
  let statusText = "";
  let showToggle = false;

  switch (indicator) {
    case "goLiveStatus": {
      const { status, isRisk } = project.goLiveStatus;
      statusText = status;
      if (isRisk) {
        statusIcon = <StatusIndicator isRisk />;
      } else if (status.includes("Hasn't Gone Live Since")) {
        statusIcon = (
          <Box sx={{ color: "warning.main", display: "inline-flex" }}>
            <TriangleAlertIcon size={20} />
          </Box>
        );
      } else {
        statusIcon = <StatusIndicator isRisk={false} />;
      }
      showToggle = hasDetails;
      break;
    }
    case "hasRecentCases": {
      const hasCases = project.hasRecentCases;
      const count = project.totalRecentCases;
      if (hasCases) {
        statusIcon = <StatusIndicator isRisk={false} />;
        statusText = `${count} Cases`;
        showToggle = true;
      } else {
        statusIcon = <StatusIndicator isRisk />;
        statusText = "None";
        showToggle = false;
      }
      break;
    }
    case "isUsingEolProduct": {
      const isUsing = project.isUsingEolProduct;
      if (isUsing) {
        statusIcon = <StatusIndicator isRisk />;
        statusText = "In Use";
        showToggle = hasDetails;
      } else {
        statusIcon = <StatusIndicator isRisk={false} />;
        statusText = "No";
        showToggle = false;
      }
      break;
    }
    default: {
      const riskExists = !!project[indicator];
      const count = detailData?.length || 0;
      if (riskExists) {
        statusIcon = <StatusIndicator isRisk />;
        statusText = hasDetails ? `${count} Exist` : "Exist";
        showToggle = hasDetails;
      } else {
        statusIcon = <StatusIndicator isRisk={false} />;
        statusText = "None";
        showToggle = false;
      }
      break;
    }
  }

  return (
    <TableCell>
      <Box display="flex" alignItems="center">
        {statusIcon}
        <Typography variant="body2" sx={{ ml: 1 }}>{statusText}</Typography>
        {showToggle && (
          <IconButton size="small" onClick={() => setOpen(!open)}>
            {open ? <ChevronUpIcon size={18} /> : <ChevronDownIcon size={18} />}
          </IconButton>
        )}
      </Box>
      {showToggle && (
        <Collapse in={open} timeout="auto" unmountOnExit>
          <List dense disablePadding sx={{ pl: 2 }}>
            {detailData!.map((item, index) => renderDetailItem(item as never, index))}
          </List>
        </Collapse>
      )}
    </TableCell>
  );
}

interface CustomerHealthDetailTableProps {
  accountId: string;
  onAccountNameLoaded?: (name: string) => void;
}

export default function CustomerHealthDetailTable({ accountId, onAccountNameLoaded }: CustomerHealthDetailTableProps) {
  const theme = useTheme();
  const { data: accountData, isLoading: loading, isError, error } = useAccountHealthDetail(accountId);
  const { data: healthStatusData, refetch: refetchHealthStatus } = useAccountHealthStatus(accountId);
  const initHealthTracking = useInitHealthTracking();

  // Once the project list is known, ensure all projects have a health status row in the DB.
  // Idempotent — existing rows are never touched.
  useEffect(() => {
    if (accountData?.customerProjects && accountData.customerProjects.length > 0) {
      const projectSysIds = accountData.customerProjects.map((p) => p.sysId);
      initHealthTracking.mutate({ accountId, projectSysIds });
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [accountData]);

  const handleHealthStatusChanged = () => {
    void refetchHealthStatus();
  };

  const [addForProject, setAddForProject] = useState<string | null>(null);

  const getProjectHealthStatus = (projectSysId: string) =>
    healthStatusData?.find((s) => s.projectSysId === projectSysId) ?? null;

  useEffect(() => {
    if (accountData?.accountName && onAccountNameLoaded) onAccountNameLoaded(accountData.accountName);
  }, [accountData, onAccountNameLoaded]);

  if (loading) return <CircularProgress size={20} sx={{ mt: 3 }} />;

  // FIXED (was `error.status === 500`, matching the OLD Ballerina backend's
  // behavior of 500-ing on an unknown account — the new Go backend
  // (GetAccountDetail/servicenow.GetCustomerHealthDetail) deliberately
  // returns 404 instead, see servicenow.ErrAccountNotFound in
  // internal/servicenow/customer_health.go). Checking 404 via
  // isNotFoundError (BackendApiError.status === 404).
  if (isError && isNotFoundError(error)) {
    return <Typography sx={{ mt: 3, textAlign: "center" }}>Couldn't find that account.</Typography>;
  }

  if (!accountData) return <Typography sx={{ mt: 3, textAlign: "center" }}>No data available for this account.</Typography>;

  if (!accountData.customerProjects || accountData.customerProjects.length === 0) {
    return <Typography sx={{ mt: 3, textAlign: "center" }}>No projects found for this account.</Typography>;
  }

  const riskTypesCount = [
    accountData.customerProjects.some((p) => !p.hasRecentCases),
    accountData.customerProjects.some((p) => p.hasAbandonedCases),
    accountData.customerProjects.some((p) => p.isUsingEolProduct),
    accountData.customerProjects.some((p) => p.hasMigrationDelays),
    accountData.customerProjects.some((p) => p.hasEscalatedCases),
    accountData.customerProjects.some((p) => p.goLiveStatus.isRisk),
  ].filter(Boolean).length;

  return (
    <>
      <Box sx={{ mt: 3, mb: 2 }}>
        <Box sx={{ display: "flex", alignItems: "center", justifyContent: "space-between", mb: 1.5 }}>
          <Box>
            <Typography variant="subtitle1" fontWeight={600} sx={{ color: "text.primary", lineHeight: 1.3 }}>Project Health Overview</Typography>
            <Typography variant="caption" sx={{ color: "text.secondary" }}>Review and update the health status of each project</Typography>
          </Box>
        </Box>
        <Box sx={{ display: "flex", gap: 2, overflowX: "auto", pb: 1 }}>
          {accountData.customerProjects.map((project) => {
            const phs = getProjectHealthStatus(project.sysId);
            const status = phs?.healthStatus?.status ?? "to_be_reviewed";
            const borderColor =
              status === "at_risk"
                ? theme.palette.error.main
                : status === "healthy"
                  ? theme.palette.success.main
                  : theme.palette.warning.main;
            return (
              <Card key={project.sysId} elevation={1} sx={{ minWidth: 320, maxWidth: 400, flexShrink: 0, borderLeft: `4px solid ${borderColor}`, backgroundColor: "background.paper" }}>
                <CardContent sx={{ p: 3, pb: "24px !important" }}>
                  <Typography variant="subtitle1" fontWeight="bold" sx={{ mb: 2, fontSize: "1.1rem", lineHeight: 1.5 }}>{project.name}</Typography>
                  <ReviewStatusCell
                    project={project}
                    accountSysId={accountId}
                    healthStatus={phs}
                    onStatusChanged={handleHealthStatusChanged}
                    onAddActionItem={() => {
                      setAddForProject(project.sysId);
                      setTimeout(() => document.getElementById("action-items-section")?.scrollIntoView({ behavior: "smooth" }), 150);
                    }}
                  />
                </CardContent>
              </Card>
            );
          })}
        </Box>
      </Box>
      <Accordion sx={{ mt: 2 }} disableGutters>
        <AccordionSummary
          expandIcon={
            <Box sx={{ color: "text.secondary", display: "inline-flex" }}>
              <ChevronDownIcon size={20} />
            </Box>
          }
          sx={{ minHeight: 0, backgroundColor: "action.hover", borderBottom: `1px solid ${theme.palette.divider}`, "& .MuiAccordionSummary-content": { my: 1, alignItems: "center", gap: 1.5 } }}
        >
          <Typography fontWeight={700} sx={{ fontSize: "0.95rem", color: "text.primary" }}>Risk Indicators</Typography>
          <Chip
            label={`${riskTypesCount} of 6 flagged`}
            size="small"
            sx={{
              fontSize: "0.72rem", fontWeight: 600, height: 20,
              bgcolor: alpha(riskTypesCount > 0 ? theme.palette.warning.main : theme.palette.success.main, 0.16),
              color: riskTypesCount > 0 ? theme.palette.warning.main : theme.palette.success.main,
              border: `1px solid ${alpha(riskTypesCount > 0 ? theme.palette.warning.main : theme.palette.success.main, 0.5)}`,
              "& .MuiChip-label": { px: 1 },
            }}
          />
        </AccordionSummary>
        <AccordionDetails sx={{ p: 0 }}>
          <Paper elevation={0}>
            <TableContainer sx={{ overflowX: "auto" }}>
              <Table stickyHeader aria-label="customer health detail table">
                <TableHead>
                  <TableRow>
                    <TableCell sx={{ borderBottom: "none", position: "sticky", left: 0, zIndex: 5, backgroundColor: "background.paper", minWidth: 180, py: "6px" }} />
                    <TableCell
                      colSpan={accountData.customerProjects.length}
                      align="center"
                      sx={{ py: "6px", fontSize: "0.7rem", fontWeight: 600, letterSpacing: "1.5px", color: "text.secondary", textTransform: "uppercase", borderBottom: `1px solid ${theme.palette.divider}` }}
                    >
                      Projects
                    </TableCell>
                  </TableRow>
                  <TableRow>
                    <TableCell sx={{ fontWeight: "bold", verticalAlign: "middle", borderTop: "none", position: "sticky", left: 0, zIndex: 5, backgroundColor: "background.paper", minWidth: 180 }}>
                      Risk Indicator
                    </TableCell>
                    {accountData.customerProjects.map((project) => (
                      <TableCell key={project.sysId} sx={{ fontWeight: "bold", minWidth: 250 }}>{project.name}</TableCell>
                    ))}
                  </TableRow>
                </TableHead>
                <TableBody>
                  <TableRow>
                    <TableCell component="th" scope="row" sx={{ position: "sticky", left: 0, backgroundColor: "background.paper", zIndex: 1 }}>Cases Within Past 6 Months</TableCell>
                    {accountData.customerProjects.map((project) => (
                      <RiskCell
                        key={project.sysId + "-cases"}
                        project={project}
                        indicator="hasRecentCases"
                        detailData={project.detailedRecentCases}
                        renderDetailItem={(item: CaseGroup, index) => (
                          <ListItem key={index} dense sx={{ pl: 4 }}>
                            <ListItemText primary={`${item.priority} - ${item.count}`} />
                          </ListItem>
                        )}
                      />
                    ))}
                  </TableRow>
                  <TableRow>
                    <TableCell component="th" scope="row" sx={{ position: "sticky", left: 0, backgroundColor: "background.paper", zIndex: 1 }}>Abandoned Product Migrations</TableCell>
                    {accountData.customerProjects.map((project) => (
                      <RiskCell
                        key={project.sysId + "-abandoned"}
                        project={project}
                        indicator="hasAbandonedCases"
                        detailData={project.detailedAbandonedCases}
                        renderDetailItem={(item: CaseLink, index) => (
                          <ListItem key={index} dense sx={{ pl: 4 }}>
                            <ListItemText primary={item.number} />
                          </ListItem>
                        )}
                      />
                    ))}
                  </TableRow>
                  <TableRow>
                    <TableCell component="th" scope="row" sx={{ position: "sticky", left: 0, backgroundColor: "background.paper", zIndex: 1 }}>Using EOL Products</TableCell>
                    {accountData.customerProjects.map((project) => (
                      <RiskCell
                        key={project.sysId + "-eol"}
                        project={project}
                        indicator="isUsingEolProduct"
                        detailData={project.softwareModel}
                        renderDetailItem={(item: EolProduct, index) => (
                          <ListItem key={index} dense sx={{ pl: 4, display: "block" }}>
                            <Typography variant="body2"><strong>Product:</strong> {item.name}</Typography>
                            <Typography variant="body2"><strong>EOL Date:</strong> {item.eolDate}</Typography>
                            {item.deployments && item.deployments.length > 0 && (
                              <>
                                <Typography variant="body2"><strong>Deployments:</strong></Typography>
                                <List dense disablePadding sx={{ pl: 2 }}>
                                  {item.deployments.map((dep) => (
                                    <ListItem key={dep.sysId} dense><ListItemText primary={dep.name} /></ListItem>
                                  ))}
                                </List>
                              </>
                            )}
                          </ListItem>
                        )}
                      />
                    ))}
                  </TableRow>
                  <TableRow>
                    <TableCell component="th" scope="row" sx={{ position: "sticky", left: 0, backgroundColor: "background.paper", zIndex: 1 }}>Migration Delays</TableCell>
                    {accountData.customerProjects.map((project) => (
                      <RiskCell
                        key={project.sysId + "-delays"}
                        project={project}
                        indicator="hasMigrationDelays"
                        detailData={project.detailedMigrationDelays}
                        renderDetailItem={(item: CaseLink, index) => (
                          <ListItem key={index} dense sx={{ pl: 4 }}>
                            <ListItemText primary={item.number} />
                          </ListItem>
                        )}
                      />
                    ))}
                  </TableRow>
                  <TableRow>
                    <TableCell component="th" scope="row" sx={{ position: "sticky", left: 0, backgroundColor: "background.paper", zIndex: 1 }}>Recent Escalations (3mo)</TableCell>
                    {accountData.customerProjects.map((project) => (
                      <RiskCell
                        key={project.sysId + "-escalations"}
                        project={project}
                        indicator="hasEscalatedCases"
                        detailData={project.detailedEscalatedCases}
                        renderDetailItem={(item: CaseLink, index) => (
                          <ListItem key={index} dense sx={{ pl: 4 }}>
                            <ListItemText primary={item.escalationNumber} />
                          </ListItem>
                        )}
                      />
                    ))}
                  </TableRow>
                  <TableRow>
                    <TableCell component="th" scope="row" sx={{ position: "sticky", left: 0, backgroundColor: "background.paper", zIndex: 1 }}>Go Live Status</TableCell>
                    {accountData.customerProjects.map((project) => (
                      <RiskCell
                        key={project.sysId + "-golive"}
                        project={project}
                        indicator="goLiveStatus"
                        detailData={[]}
                        renderDetailItem={() => null}
                      />
                    ))}
                  </TableRow>
                </TableBody>
              </Table>
            </TableContainer>
          </Paper>
        </AccordionDetails>
      </Accordion>
      <ActionItemsSection
        accountId={accountId}
        projects={accountData.customerProjects}
        healthStatusData={healthStatusData ?? []}
        onDataChanged={handleHealthStatusChanged}
        openAddForProject={addForProject}
        onAddForProjectConsumed={() => setAddForProject(null)}
      />
    </>
  );
}

function StatusIndicator({ isRisk }: { isRisk: boolean }) {
  const color = isRisk ? red[700] : green[700];
  return (
    <Box sx={{ color, display: "inline-flex" }}>
      {isRisk ? <CircleXIcon size={22} /> : <CircleCheckIcon size={22} />}
    </Box>
  );
}
