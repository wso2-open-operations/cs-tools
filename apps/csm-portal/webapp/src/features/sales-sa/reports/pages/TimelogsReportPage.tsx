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

// Ported from apps/support-portal-lite/webapp's TimelogsReportPage
// (itself via one-wso2, from SupportPortalLite's TimelogsReport.tsx). UI/JSX
// unchanged; data layer rewritten onto useBackendApi()/React Query — see
// useGetTimelogsBreakdown.ts. No SplShell wrapper. Route:
// spl/projects/:projectId/timelogs-report.
//
// Deviation from source: MainView/ToggleReport were inline function
// components (recreated every render) in the source file. This app's lint
// config (react-hooks/static-components, React Compiler compatibility)
// forbids that — hoisted both to module scope, same treatment
// SlaReportPage's FormView/MainView already needed.
import { useState, type JSX, type SyntheticEvent } from "react";
import {
  Alert,
  Box,
  Button,
  LinearProgress,
  Paper,
  Tab,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  Tabs,
  Tooltip,
  Typography,
} from "@mui/material";
import { ThemeProvider } from "@mui/material/styles";
import { reportPaperTheme } from "@features/sales-sa/reports/reportPaperTheme";
import DownloadIcon from "@mui/icons-material/Download";
import DOMPurify from "dompurify";
import html2canvas from "html2canvas";
import jsPDF from "jspdf";
import { useParams } from "react-router";
import { BackendApiError } from "@api/backend/client";
import { useGetTimelogsBreakdown } from "@features/sales-sa/reports/api/useGetTimelogsBreakdown";
import PieChart from "@features/sales-sa/reports/components/PieChart";
import type { DataStruct, TimeCardDetails, TimeLogBreakdownDetails } from "@features/sales-sa/reports/api/reportTypes";
import { sysidToUuid } from "@features/csm-cases/utils/inlineImages";
import "@features/sales-sa/reports/styles/TimelogsReport.css";

function downloadPDF() {
  const input = document.getElementById("timelogs-report");
  if (!input) return;
  html2canvas(input, { scale: 2 }).then((canvas) => {
    const imgData = canvas.toDataURL("image/png", 1.0);
    const pdf = new jsPDF("p", "mm", "a4");
    const pdfWidth = pdf.internal.pageSize.getWidth();
    const pdfHeight = pdf.internal.pageSize.getHeight();
    const imgProps = pdf.getImageProperties(imgData);
    const imgHeight = (imgProps.height * pdfWidth) / imgProps.width;
    let heightLeft = imgHeight;
    let position = 0;
    pdf.addImage(imgData, "PNG", 0, position, pdfWidth, imgHeight);
    heightLeft -= pdfHeight;
    while (heightLeft >= 0) {
      position = heightLeft - imgHeight;
      pdf.addPage();
      pdf.addImage(imgData, "PNG", 0, position, pdfWidth, imgHeight);
      heightLeft -= pdfHeight;
    }
    pdf.save("Timelogs_Report.pdf");
  });
}

function MainView({
  data,
  showReport,
  isCustomerReport,
  onRowClick,
}: {
  data: TimeLogBreakdownDetails | undefined;
  showReport: boolean;
  isCustomerReport: boolean;
  onRowClick: (row: DataStruct) => void;
}) {
  const casesColNameArray = [
    "Case Number",
    "Case Id",
    "Case Type",
    "Short Description",
    "State",
    isCustomerReport ? "Consumed Query Time" : "Total Time",
  ];
  const casesColAttributeArray = [
    "caseNumber",
    "caseId",
    "caseType",
    "shortDescription",
    "state",
    isCustomerReport ? "consumedQueryHours" : "totalHours",
  ];
  const timecardColNameArray = ["Logged On", "Logged By", isCustomerReport ? "" : "State", "Billable", "Total Time"];
  const timecardColAttributeArray = ["createdOn", "createdBy", isCustomerReport ? "" : "state", "isBillable", "total"];

  const filteredCases = isCustomerReport ? data?.cases.filter((c) => c.caseType === "Query") : data?.cases;

  const filterBillableAndApproved = (timeCards: TimeCardDetails[]) =>
    isCustomerReport ? timeCards.filter((tc) => tc.isBillable === "true" && tc.state === "Approved") : timeCards;

  return (
    <>
      {showReport && (
        <Box
          flex={2}
          display="flex"
          justifyContent="flex-end"
          p={1}
          marginBottom="-80px"
          marginRight="30px"
          sx={{ position: "fixed", top: "100px", right: "30px", zIndex: 2000 }}
        >
          <Tooltip title="Download Report">
            <Button
              variant="contained"
              sx={{
                color: "white",
                ":hover": { bgcolor: "#e96900", borderColor: "primary.main", color: "white" },
                borderRadius: "7px",
              }}
              onClick={downloadPDF}
            >
              <DownloadIcon />
            </Button>
          </Tooltip>
        </Box>
      )}
      {/* A printable document (captured to PDF by downloadPDF above) —
          kept a fixed light "paper" theme regardless of the app's own
          light/dark mode. See reportPaperTheme.ts. */}
      <ThemeProvider theme={reportPaperTheme}>
      <div id="timelogs-report">
        <Box width="80%" padding="3%" margin="auto" sx={{ backgroundColor: "#f7f7f6", marginTop: "15px" }}>
          <Box className="tl-header">
            <Typography variant="h4" className="tl-header-title">
              {isCustomerReport ? "Query Hours Consumption Report" : "Time Report"}
            </Typography>
          </Box>

          <Box display="flex" justifyContent="space-between" alignItems="flex-start">
            <Box className="tl-table-container tl-table-info" sx={{ width: "60%" }}>
              <TableContainer component={Paper}>
                <Table size="small">
                  <TableBody className="tl-table-body">
                    <TableRow>
                      <TableCell>
                        <Typography variant="subtitle2" fontWeight="bold">
                          Project Name
                        </Typography>
                      </TableCell>
                      <TableCell>{data?.projectName}</TableCell>
                    </TableRow>
                    <TableRow>
                      <TableCell>
                        <Typography variant="subtitle2" fontWeight="bold">
                          Project Key
                        </Typography>
                      </TableCell>
                      <TableCell>{data?.projectKey}</TableCell>
                    </TableRow>
                    <TableRow>
                      <TableCell>
                        <Typography variant="subtitle2" fontWeight="bold">
                          Project Type
                        </Typography>
                      </TableCell>
                      <TableCell>{data?.projectType}</TableCell>
                    </TableRow>
                    <TableRow>
                      <TableCell>
                        <Typography variant="subtitle2" fontWeight="bold">
                          Remaining Query Hours
                        </Typography>
                      </TableCell>
                      <TableCell>{data?.remainingQueryHours}</TableCell>
                    </TableRow>
                    <TableRow>
                      <TableCell>
                        <Typography variant="subtitle2" fontWeight="bold">
                          Total Query Hours
                        </Typography>
                      </TableCell>
                      <TableCell>{data?.totalQueryHours}</TableCell>
                    </TableRow>
                  </TableBody>
                </Table>
              </TableContainer>
            </Box>
            <Box sx={{ width: "30%", marginLeft: "20px" }}>
              <PieChart
                remainingQueryHours={data?.remainingQueryHours || "0h 0m"}
                totalQueryHours={data?.totalQueryHours || "0h 0m"}
              />
            </Box>
          </Box>

          <Box className="tl-table-container">
            <Typography variant="h6" className="tl-subheading">
              Project Cases
            </Typography>
            <TableContainer component={Paper}>
              <Table className="tl-case-table">
                <TableHead className="tl-table-header">
                  <TableRow>
                    {casesColNameArray.map((value, index) => (
                      <TableCell key={index}>{value}</TableCell>
                    ))}
                  </TableRow>
                </TableHead>
                <TableBody>
                  {!filteredCases || filteredCases.length === 0 ? (
                    <TableRow>
                      <TableCell colSpan={casesColNameArray.length} align="center">
                        No data available.
                      </TableCell>
                    </TableRow>
                  ) : (
                    filteredCases.map((dataRow, index) => (
                      <TableRow key={index}>
                        {casesColAttributeArray.map((attributeName, i) => (
                          <TableCell
                            key={i}
                            className={`t-col-size ${attributeName === "totalHours" || attributeName === "consumedQueryHours" ? "tl-table-cell" : ""}`}
                            onClick={() => {
                              if (attributeName === "caseNumber") onRowClick(dataRow);
                            }}
                            sx={{
                              cursor: attributeName === "caseNumber" ? "pointer" : "default",
                              "&:hover": { backgroundColor: attributeName === "caseNumber" ? "#97c9ff" : "inherit" },
                            }}
                          >
                            {(attributeName === "totalHours" || attributeName === "consumedQueryHours") &&
                            parseFloat(dataRow[attributeName]) > 0 ? (
                              <a href={`#${dataRow.caseNumber}`} style={{ marginLeft: 5 }}>
                                {" "}
                                {dataRow[attributeName]}
                              </a>
                            ) : (
                              dataRow[attributeName]
                            )}
                          </TableCell>
                        ))}
                      </TableRow>
                    ))
                  )}
                </TableBody>
              </Table>
            </TableContainer>
          </Box>

          {showReport && (
            <Box sx={{ marginTop: "50px", textAlign: "center", marginBottom: "20px" }}>
              <Typography variant="h4" sx={{ fontWeight: "bold" }}>
                Time Cards
              </Typography>
            </Box>
          )}

          {filteredCases?.map(
            (caseItem) =>
              filterBillableAndApproved(caseItem.timeCards).length > 0 && (
                <div id={caseItem.caseNumber} key={caseItem.caseId}>
                  <Box className="tl-timecard-table-container">
                    <Typography variant="h6" className="tl-subheading">
                      {caseItem.caseNumber + " / " + caseItem.caseId}
                    </Typography>
                    <TableContainer component={Paper}>
                      <Table className="tl-case-table">
                        <TableHead className="tl-table-header">
                          <TableRow>
                            {timecardColNameArray.map((value, index) => (
                              <TableCell key={index} className={`t-col-size ${value === "Total Time" ? "tl-table-cell" : ""}`}>
                                {value}
                              </TableCell>
                            ))}
                          </TableRow>
                        </TableHead>
                        <TableBody>
                          {filterBillableAndApproved(caseItem.timeCards).map((dataRow, index) => (
                            <TableRow key={index}>
                              {timecardColAttributeArray.map((attributeName, i) => (
                                <TableCell key={i} className={`t-col-size ${attributeName === "total" ? "tl-table-cell" : ""}`}>
                                  {dataRow[attributeName]}
                                </TableCell>
                              ))}
                            </TableRow>
                          ))}
                        </TableBody>
                      </Table>
                    </TableContainer>
                  </Box>
                </div>
              ),
          )}
        </Box>
      </div>
      </ThemeProvider>
    </>
  );
}

function ToggleReport({
  isCustomerReport,
  onChange,
}: {
  isCustomerReport: boolean;
  onChange: (isCustomer: boolean) => void;
}) {
  const [value, setValue] = useState(isCustomerReport ? 1 : 0);

  const handleChange = (_event: SyntheticEvent, newValue: number) => {
    setValue(newValue);
    onChange(newValue === 1);
  };

  return (
    <div style={{ border: "1px solid #ff7300", borderRadius: 25, height: "49.5px", width: "fit-content" }}>
      <Tabs
        sx={{
          justifyContent: "flex-start",
          "& button": { borderRadius: 10, boxShadow: "insert", zIndex: 1 },
          "& button.Mui-selected": { color: "#fff" },
        }}
        value={value}
        onChange={handleChange}
        slotProps={{ indicator: { sx: { backgroundColor: "#ff7300", height: "100%", borderRadius: 10 } } }}
      >
        <Tab label="Internal Report" />
        <Tab label="Customer Report" />
      </Tabs>
    </div>
  );
}

export default function TimelogsReportPage(): JSX.Element {
  const { projectId: id } = useParams<{ projectId: string }>();
  const projectId = id ? DOMPurify.sanitize(id) : "";
  const [errorMessage, setErrorMessage] = useState<string | null>(null);
  const [isCustomerReport, setCustomerReport] = useState(false);

  const { data, isLoading, error } = useGetTimelogsBreakdown(projectId);
  const showReport = Boolean(data);

  const handleRowClick = (rowData: DataStruct) => {
    // This report's case rows are still ServiceNow-sourced (see
    // postgresReportsClient's own doc comment on the backend -- report
    // case data isn't part of the accounts/projects/cases entity-service
    // merge), so caseSysId is a bare ServiceNow sysid, not entity-service's
    // own dashed UUID -- sysidToUuid converts it to what the normal
    // /cases/:caseId route (which validates a dashed UUID) needs.
    if (rowData?.caseSysId) {
      window.open(`/cases/${sysidToUuid(rowData.caseSysId)}`, "_blank");
    } else {
      setErrorMessage("Case not found.");
    }
  };

  return (
    <Box sx={{ p: 3 }}>
      {errorMessage && (
        <Alert severity="warning" onClose={() => setErrorMessage(null)} sx={{ mb: 2 }}>
          {errorMessage}
        </Alert>
      )}
      {isLoading ? (
        <>
          <LinearProgress />
          <Box className="blur">
            <MainView data={data} showReport={showReport} isCustomerReport={isCustomerReport} onRowClick={handleRowClick} />
          </Box>
        </>
      ) : error ? (
        <Typography color="error" sx={{ mt: 2 }}>
          {error instanceof BackendApiError && error.status === 404
            ? "Timelogs report not found."
            : "Something went wrong loading the timelogs report."}
        </Typography>
      ) : (
        <>
          {showReport && (
            <>
              <ToggleReport isCustomerReport={isCustomerReport} onChange={setCustomerReport} />
              <MainView data={data} showReport={showReport} isCustomerReport={isCustomerReport} onRowClick={handleRowClick} />
            </>
          )}
          {!showReport && (
            <Box className="blur">
              <MainView data={data} showReport={showReport} isCustomerReport={isCustomerReport} onRowClick={handleRowClick} />
            </Box>
          )}
        </>
      )}
    </Box>
  );
}
