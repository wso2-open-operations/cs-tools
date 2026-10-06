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

// Ported from apps/support-portal-lite/webapp's SlaReportPage (itself via
// one-wso2, from SupportPortalLite's SLAReport.tsx). UI/JSX unchanged; data
// layer rewritten onto useBackendApi()/React Query — see useGetSlaReport.ts.
// No SplShell wrapper. Route: spl/projects/:projectId/sla-report/:sysId (see
// App.tsx) — matches the source's own :sysId param name.
//
// One deviation from source, defensive rather than cosmetic: the source
// indexes percentileDataList[1..4] directly (P1/P2/P3/Query rows assumed
// present at fixed positions). Optional-chained here too, same as source.
import { useState, type JSX } from "react";
import {
  Alert,
  Box,
  Button,
  Grid,
  LinearProgress,
  Paper,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  Tooltip,
  Typography,
} from "@mui/material";
import { ThemeProvider } from "@mui/material/styles";
import { reportPaperTheme } from "@features/sales-sa/reports/reportPaperTheme";
import DownloadIcon from "@mui/icons-material/Download";
import { DatePicker, LocalizationProvider } from "@mui/x-date-pickers";
import { AdapterDayjs } from "@mui/x-date-pickers/AdapterDayjs";
import dayjs, { type Dayjs } from "dayjs";
import DOMPurify from "dompurify";
import html2canvas from "html2canvas";
import jsPDF from "jspdf";
import { useParams } from "react-router";
import { BackendApiError } from "@api/backend/client";
import { useGetSlaReport } from "@features/sales-sa/reports/api/useGetSlaReport";
import DividedCell from "@features/sales-sa/reports/components/DividedCell";
import type { CaseDataList, SLAReportResponse } from "@features/sales-sa/reports/api/reportTypes";
import { sysidToUuid } from "@features/csm-cases/utils/inlineImages";
import "@features/sales-sa/reports/styles/SLAReport.css";

// Hoisted out of the page component — this codebase's lint config
// (react-hooks/static-components, for React Compiler compatibility) forbids
// components whose identity is recreated on every render.
function FormView({
  from,
  to,
  onFromChange,
  onToChange,
  onSubmit,
}: {
  from: string;
  to: string;
  onFromChange: (v: Dayjs | null) => void;
  onToChange: (v: Dayjs | null) => void;
  onSubmit: () => void;
}) {
  return (
    <>
      <div className="backdrop" />
      <Paper className="centered-form">
        <Typography gutterBottom variant="h4" component="div" marginBottom="20px">
          SLA Report
        </Typography>
        <Typography variant="body1" gutterBottom marginBottom="20px">
          Cases that were opened during this period will be listed
        </Typography>
        <LocalizationProvider dateAdapter={AdapterDayjs}>
          <Grid container spacing={2} marginBottom="30px">
            <Grid>
              <DatePicker label="From" value={dayjs(from)} onChange={onFromChange} />
            </Grid>
            <Grid>
              <DatePicker label="To" value={dayjs(to)} onChange={onToChange} />
            </Grid>
          </Grid>
        </LocalizationProvider>
        <Button
          variant="contained"
          sx={{ color: "white", ":hover": { bgcolor: "#e96900", borderColor: "primary.main", color: "white" } }}
          onClick={onSubmit}
        >
          Submit
        </Button>
      </Paper>
    </>
  );
}

function MainView({
  data,
  from,
  to,
  showReport,
  onDownload,
  onRowClick,
}: {
  data: SLAReportResponse | undefined;
  from: string;
  to: string;
  showReport: boolean;
  onDownload: () => void;
  onRowClick: (row: CaseDataList) => void;
}) {
  const colNameArray = ["WSO2 Case ID", "Case Type", "Priority", "State", "Opened", "Response", "Workaround", "Resolution"];
  const colAttributeArray: (keyof CaseDataList)[] = [
    "caseId",
    "caseType",
    "casepriority",
    "caseState",
    "opened",
    "response",
    "workaround",
    "resolution",
  ];
  const percentiles = data?.percentileDataList ?? [];

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
          sx={{ position: "fixed", top: "120px", right: "30px", zIndex: 2000 }}
        >
          <Tooltip title="Download Report">
            <Button
              variant="contained"
              sx={{
                color: "white",
                ":hover": { bgcolor: "#e96900", borderColor: "primary.main", color: "white" },
                borderRadius: "7px",
              }}
              onClick={onDownload}
            >
              <DownloadIcon />
            </Button>
          </Tooltip>
        </Box>
      )}

      {/* A printable document (captured to PDF by handleDownloadPDF below) —
          kept a fixed light "paper" theme regardless of the app's own
          light/dark mode. See reportPaperTheme.ts. */}
      <ThemeProvider theme={reportPaperTheme}>
      <div id="sla-report">
        <Box width="80%" padding="3%" margin="auto" sx={{ backgroundColor: "#f7f7f6" }}>
          <Box className="sla-header">
            <Typography variant="h4" className="sla-header-title">
              SLA Report - WSO2 Customer Service
            </Typography>
          </Box>

          <Box className="table-container table-info">
            <TableContainer component={Paper}>
              <Table size="small">
                <TableBody className="table-body">
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
                        From
                      </Typography>
                    </TableCell>
                    <TableCell>{from}</TableCell>
                  </TableRow>
                  <TableRow>
                    <TableCell>
                      <Typography variant="subtitle2" fontWeight="bold">
                        To
                      </Typography>
                    </TableCell>
                    <TableCell>{to}</TableCell>
                  </TableRow>
                  <TableRow>
                    <TableCell>
                      <Typography variant="subtitle2" fontWeight="bold">
                        Data Set
                      </Typography>
                    </TableCell>
                    <TableCell>Cases that were opened during the report period</TableCell>
                  </TableRow>
                </TableBody>
              </Table>
            </TableContainer>
          </Box>

          <Box className="table-container">
            <Typography variant="h6" className="subheading">
              95th percentile of elapsed time
            </Typography>
            <TableContainer component={Paper}>
              <Table>
                <TableHead className="table-header">
                  <TableRow>
                    <TableCell>Case Type / Priority</TableCell>
                    <TableCell>Response</TableCell>
                    <TableCell>Workaround</TableCell>
                    <TableCell>Resolution</TableCell>
                  </TableRow>
                </TableHead>
                <TableBody>
                  <TableRow>
                    <TableCell>
                      <Typography fontWeight="bold">Incident - P1</Typography>
                    </TableCell>
                    <DividedCell top={percentiles[1]?.responseTime} bottom="Target: 1 hour" />
                    <DividedCell top={percentiles[1]?.workaroundTime} bottom="Target: 1 day" />
                    <DividedCell top={percentiles[1]?.resolutionTime} bottom="Target: 2 days" />
                  </TableRow>
                  <TableRow>
                    <TableCell>
                      <Typography fontWeight="bold">Incident - P2</Typography>
                    </TableCell>
                    <DividedCell top={percentiles[2]?.responseTime} bottom="Target: 4 hour" />
                    <DividedCell top={percentiles[2]?.workaroundTime} bottom="Target: 2 days" />
                    <DividedCell top={percentiles[2]?.resolutionTime} bottom="Target: 3 days" />
                  </TableRow>
                  <TableRow>
                    <TableCell>
                      <Typography fontWeight="bold">Incident - P3</Typography>
                    </TableCell>
                    <DividedCell top={percentiles[3]?.responseTime} bottom="Target: 6 hours" />
                    <DividedCell top={percentiles[3]?.workaroundTime} bottom="Target: 3 days" />
                    <DividedCell top={percentiles[3]?.resolutionTime} bottom="Target: 1 week" />
                  </TableRow>
                  <TableRow>
                    <TableCell>
                      <Typography fontWeight="bold">Query</Typography>
                    </TableCell>
                    <DividedCell top={percentiles[4]?.responseTime} bottom="Target: 1 day" />
                    <TableCell>
                      <Typography>Not Applicable</Typography>
                    </TableCell>
                    <TableCell>
                      <Typography>Not Applicable</Typography>
                    </TableCell>
                  </TableRow>
                </TableBody>
              </Table>
            </TableContainer>
          </Box>

          <Box className="table-container">
            <Typography variant="h6" className="subheading">
              Cases
            </Typography>
            <TableContainer component={Paper}>
              <Table className="case-table">
                <TableHead className="table-header">
                  <TableRow>
                    {colNameArray.map((name) => (
                      <TableCell key={name}>{name}</TableCell>
                    ))}
                  </TableRow>
                </TableHead>
                <TableBody>
                  {!data?.caseDataList || data.caseDataList.length === 0 ? (
                    <TableRow>
                      <TableCell colSpan={colNameArray.length} align="center">
                        No data available.
                      </TableCell>
                    </TableRow>
                  ) : (
                    data.caseDataList.map((dataRow, index) => (
                      <TableRow
                        key={index}
                        onClick={() => onRowClick(dataRow)}
                        sx={{ cursor: "pointer", "&:hover": { backgroundColor: "#97c9ff" } }}
                      >
                        {colAttributeArray.map((attributeName) => (
                          <TableCell key={String(attributeName)} className="t-col-size">
                            {dataRow[attributeName]}
                          </TableCell>
                        ))}
                      </TableRow>
                    ))
                  )}
                </TableBody>
              </Table>
            </TableContainer>
          </Box>
        </Box>
      </div>
      </ThemeProvider>
    </>
  );
}

export default function SlaReportPage(): JSX.Element {
  const { sysId: id } = useParams<{ sysId: string }>();
  const projectId = id ? DOMPurify.sanitize(id) : "";
  const [errorMessage, setErrorMessage] = useState<string | null>(null);
  const [from, setFrom] = useState<string>(new Date().toLocaleDateString("en-CA"));
  const [to, setTo] = useState<string>(new Date().toLocaleDateString("en-CA"));
  const [submitted, setSubmitted] = useState(false);

  const { data, isFetching, error, refetch } = useGetSlaReport({
    projectSysId: projectId,
    from,
    to,
    enabled: submitted,
  });
  const showReport = Boolean(data);

  const handleSubmit = () => {
    setSubmitted(true);
    void refetch();
  };
  const handleFromChange = (newValue: Dayjs | null) => newValue && setFrom(newValue.format("YYYY-MM-DD"));
  const handleToChange = (newValue: Dayjs | null) => newValue && setTo(newValue.format("YYYY-MM-DD"));

  const handleRowClick = (rowData: CaseDataList) => {
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

  const downloadPDF = () => {
    const input = document.getElementById("sla-report");
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
      pdf.save("SLA_Report.pdf");
    });
  };

  return (
    <Box sx={{ p: 3 }}>
      {errorMessage && (
        <Alert severity="warning" onClose={() => setErrorMessage(null)} sx={{ mb: 2 }}>
          {errorMessage}
        </Alert>
      )}
      {isFetching ? (
        <>
          <LinearProgress />
          <Box className="blur">
            <MainView data={data} from={from} to={to} showReport={showReport} onDownload={downloadPDF} onRowClick={handleRowClick} />
          </Box>
        </>
      ) : error ? (
        <Typography color="error" sx={{ mt: 2 }}>
          {error instanceof BackendApiError && error.status === 404
            ? "SLA report not found."
            : "Something went wrong loading the SLA report."}
        </Typography>
      ) : (
        <>
          {showReport ? (
            <MainView data={data} from={from} to={to} showReport={showReport} onDownload={downloadPDF} onRowClick={handleRowClick} />
          ) : (
            <FormView from={from} to={to} onFromChange={handleFromChange} onToChange={handleToChange} onSubmit={handleSubmit} />
          )}
          {!showReport && (
            <Box className="blur">
              <MainView data={data} from={from} to={to} showReport={showReport} onDownload={downloadPDF} onRowClick={handleRowClick} />
            </Box>
          )}
        </>
      )}
    </Box>
  );
}
