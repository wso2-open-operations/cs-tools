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

// Time report. The data and the PDF export are the Support Portal Lite
// report; the page uses the same oxygen-ui cards, tables, and type as the
// CS report and the rest of CSM.
import { useState, type JSX, type ReactNode } from "react";
import { flushSync } from "react-dom";
import {
  Alert,
  Box,
  Button,
  Card,
  Chip,
  Skeleton,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TablePagination,
  TableRow,
  ToggleButton,
  ToggleButtonGroup,
  Typography,
} from "@wso2/oxygen-ui";
import { ArrowLeft } from "@wso2/oxygen-ui-icons-react";
import DOMPurify from "dompurify";
import html2canvas from "html2canvas";
import jsPDF from "jspdf";
import { useParams } from "react-router";
import QueryErrorState from "@components/QueryErrorState";
import { useGetTimelogsBreakdown } from "@features/csm-reports/api/useGetTimelogsBreakdown";
import PieChart from "@features/csm-reports/components/PieChart";
import type { TimeCardDetails, TimeLogBreakdownCase, TimeLogBreakdownDetails } from "@features/csm-reports/api/reportTypes";
import { sysidToUuid } from "@features/csm-cases/utils/inlineImages";
import { useNavTransition } from "@hooks/useNavTransition";

function display(value?: string | null): string {
  const trimmed = value?.trim();
  return trimmed ? trimmed : "—";
}

function formatProjectType(value?: string): string {
  if (!value?.trim()) return "—";
  return value.replace(/_/g, " ").replace(/\b\w/g, (c) => c.toUpperCase());
}

function yesNo(value: string): string {
  if (value === "true") return "Yes";
  if (value === "false") return "No";
  return display(value);
}

function isApproved(state: string): boolean {
  return state.trim().toLowerCase() === "approved";
}

function MetaCell({ label, children }: { label: string; children: ReactNode }): JSX.Element {
  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 0.25, minWidth: 0 }}>
      <Typography variant="caption" color="text.secondary" sx={{ textTransform: "uppercase", letterSpacing: 0.4 }}>
        {label}
      </Typography>
      <Box sx={{ minWidth: 0 }}>{children}</Box>
    </Box>
  );
}

function EmptyRow({ colSpan, label }: { colSpan: number; label: string }): JSX.Element {
  return (
    <TableRow>
      <TableCell colSpan={colSpan}>
        <Typography variant="body2" color="text.secondary">
          {label}
        </Typography>
      </TableCell>
    </TableRow>
  );
}

async function downloadPDF(): Promise<void> {
  const input = document.getElementById("timelogs-report");
  if (!input) return;
  const canvas = await html2canvas(input, { scale: 2, backgroundColor: "#ffffff", logging: false, imageTimeout: 2000 });
  const imgData = canvas.toDataURL("image/jpeg", 0.92);
  const pdf = new jsPDF("p", "mm", "a4");
  const pdfWidth = pdf.internal.pageSize.getWidth();
  const pdfHeight = pdf.internal.pageSize.getHeight();
  const imgProps = pdf.getImageProperties(imgData);
  const imgHeight = (imgProps.height * pdfWidth) / imgProps.width;
  let heightLeft = imgHeight;
  let position = 0;
  pdf.addImage(imgData, "JPEG", 0, position, pdfWidth, imgHeight);
  heightLeft -= pdfHeight;
  while (heightLeft >= 0) {
    position = heightLeft - imgHeight;
    pdf.addPage();
    pdf.addImage(imgData, "JPEG", 0, position, pdfWidth, imgHeight);
    heightLeft -= pdfHeight;
  }
  pdf.save("Timelogs_Report.pdf");
}

function ReportToggle({
  isCustomerReport,
  onChange,
}: {
  isCustomerReport: boolean;
  onChange: (isCustomer: boolean) => void;
}): JSX.Element {
  return (
    <ToggleButtonGroup
      exclusive
      size="small"
      color="primary"
      value={isCustomerReport ? "customer" : "internal"}
      onChange={(_, next: string | null) => {
        if (next) onChange(next === "customer");
      }}
    >
      <ToggleButton value="internal">Internal</ToggleButton>
      <ToggleButton value="customer">Customer</ToggleButton>
    </ToggleButtonGroup>
  );
}

function ProjectCard({ data }: { data: TimeLogBreakdownDetails }): JSX.Element {
  return (
    <Card sx={{ p: 2.5, display: "flex", flexDirection: "column", gap: 2 }}>
      <Typography variant="subtitle2">Project</Typography>
      <Box
        sx={{
          display: "grid",
          gap: 2,
          alignItems: "start",
          gridTemplateColumns: { xs: "1fr", md: "minmax(0, 1.2fr) minmax(0, 1fr)" },
        }}
      >
        <Box
          sx={{
            display: "grid",
            gap: 2,
            gridTemplateColumns: { xs: "1fr", sm: "1fr 1fr" },
          }}
        >
          <MetaCell label="Project name">
            <Typography variant="body2">{display(data.projectName)}</Typography>
          </MetaCell>
          <MetaCell label="Project key">
            <Typography variant="body2" sx={{ fontFamily: "monospace" }}>
              {display(data.projectKey)}
            </Typography>
          </MetaCell>
          <MetaCell label="Project type">
            <Typography variant="body2">{formatProjectType(data.projectType)}</Typography>
          </MetaCell>
          <MetaCell label="Total query hours">
            <Typography variant="body2">{display(data.totalQueryHours)}</Typography>
          </MetaCell>
          <MetaCell label="Remaining query hours">
            <Typography variant="body2">{display(data.remainingQueryHours)}</Typography>
          </MetaCell>
        </Box>
        <PieChart remainingQueryHours={data.remainingQueryHours || "0"} totalQueryHours={data.totalQueryHours || "0"} />
      </Box>
    </Card>
  );
}

const CASE_ROWS_PER_PAGE = [10, 20, 50];

function CasesCard({
  cases,
  linkedCaseNumbers,
  isCustomerReport,
  showAll,
  onOpenCase,
}: {
  cases: TimeLogBreakdownCase[];
  linkedCaseNumbers: Set<string>;
  isCustomerReport: boolean;
  showAll: boolean;
  onOpenCase: (row: TimeLogBreakdownCase) => void;
}): JSX.Element {
  const hoursKey = isCustomerReport ? "consumedQueryHours" : "totalHours";
  const hoursLabel = isCustomerReport ? "Consumed query time" : "Total time";
  const [page, setPage] = useState(0);
  const [rowsPerPage, setRowsPerPage] = useState(CASE_ROWS_PER_PAGE[0]);

  const pageCount = Math.max(1, Math.ceil(cases.length / rowsPerPage));
  const safePage = Math.min(page, pageCount - 1);
  const visibleCases = showAll ? cases : cases.slice(safePage * rowsPerPage, safePage * rowsPerPage + rowsPerPage);

  return (
    <Card sx={{ p: 2.5, display: "flex", flexDirection: "column", gap: 2 }}>
      <Typography variant="subtitle2">Project cases</Typography>
      <TableContainer>
        <Table size="small">
          <TableHead>
            <TableRow>
              <TableCell>Case number</TableCell>
              <TableCell>Case id</TableCell>
              <TableCell>Case type</TableCell>
              <TableCell>Short description</TableCell>
              <TableCell>State</TableCell>
              <TableCell>{hoursLabel}</TableCell>
            </TableRow>
          </TableHead>
          <TableBody>
            {visibleCases.length === 0 ? (
              <EmptyRow colSpan={6} label="No cases for this report." />
            ) : (
              visibleCases.map((row) => {
                const hours = row[hoursKey] as string;
                return (
                  <TableRow key={row.caseId || row.caseNumber}>
                    <TableCell>
                      <Button variant="text" size="small" sx={{ p: 0, minWidth: 0 }} onClick={() => onOpenCase(row)}>
                        {display(row.caseNumber)}
                      </Button>
                    </TableCell>
                    <TableCell>{display(row.caseId)}</TableCell>
                    <TableCell>{display(row.caseType)}</TableCell>
                    <TableCell>{display(row.shortDescription)}</TableCell>
                    <TableCell>{row.state ? <Chip size="small" variant="outlined" label={row.state} /> : "—"}</TableCell>
                    <TableCell>
                      {parseFloat(hours) > 0 && linkedCaseNumbers.has(row.caseNumber) ? (
                        <Button variant="text" size="small" sx={{ p: 0, minWidth: 0 }} href={`#${row.caseNumber}`}>
                          {hours}
                        </Button>
                      ) : (
                        display(hours)
                      )}
                    </TableCell>
                  </TableRow>
                );
              })
            )}
          </TableBody>
        </Table>
      </TableContainer>
      {!showAll && cases.length > 0 && (
        <TablePagination
          component="div"
          count={cases.length}
          page={safePage}
          onPageChange={(_, nextPage) => setPage(nextPage)}
          rowsPerPage={rowsPerPage}
          onRowsPerPageChange={(event) => {
            setRowsPerPage(parseInt(event.target.value, 10));
            setPage(0);
          }}
          rowsPerPageOptions={CASE_ROWS_PER_PAGE}
          showFirstButton
          showLastButton
        />
      )}
    </Card>
  );
}

function TimeCardsCard({
  caseItem,
  cards,
  isCustomerReport,
}: {
  caseItem: TimeLogBreakdownCase;
  cards: TimeCardDetails[];
  isCustomerReport: boolean;
}): JSX.Element {
  return (
    <Card id={caseItem.caseNumber} sx={{ p: 2.5, display: "flex", flexDirection: "column", gap: 2 }}>
      <Typography variant="subtitle2">
        {display(caseItem.caseNumber)}
        {caseItem.caseId ? ` · ${caseItem.caseId}` : ""}
      </Typography>
      <TableContainer>
        <Table size="small">
          <TableHead>
            <TableRow>
              <TableCell>Logged on</TableCell>
              <TableCell>Logged by</TableCell>
              {!isCustomerReport && <TableCell>State</TableCell>}
              <TableCell>Billable</TableCell>
              <TableCell>Total time</TableCell>
            </TableRow>
          </TableHead>
          <TableBody>
            {cards.map((card, index) => (
              <TableRow key={`${card.createdOn}-${card.createdBy}-${index}`}>
                <TableCell>{display(card.createdOn)}</TableCell>
                <TableCell>{display(card.createdBy)}</TableCell>
                {!isCustomerReport && (
                  <TableCell>{card.state ? <Chip size="small" variant="outlined" label={card.state} /> : "—"}</TableCell>
                )}
                <TableCell>{yesNo(card.isBillable)}</TableCell>
                <TableCell>{display(card.total)}</TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </TableContainer>
    </Card>
  );
}

export default function TimelogsReportPage(): JSX.Element {
  const { projectId: id } = useParams<{ projectId: string }>();
  const projectId = id ? DOMPurify.sanitize(id) : "";
  const navigate = useNavTransition();
  const [errorMessage, setErrorMessage] = useState<string | null>(null);
  const [downloadError, setDownloadError] = useState("");
  const [isCustomerReport, setCustomerReport] = useState(false);
  const [printing, setPrinting] = useState(false);

  const { data, isLoading, isError, error, refetch } = useGetTimelogsBreakdown(projectId);

  const cases = (isCustomerReport ? data?.cases.filter((c) => c.caseType === "Query") : data?.cases) ?? [];
  const timeCardCases = cases
    .map((caseItem) => ({
      caseItem,
      cards: (isCustomerReport
        ? (caseItem.timeCards ?? []).filter((tc) => tc.isBillable === "true" && isApproved(tc.state))
        : (caseItem.timeCards ?? [])),
    }))
    .filter((entry) => entry.cards.length > 0);

  const handleOpenCase = (row: TimeLogBreakdownCase) => {
    // caseSysId is entity-service's case id. A dashed UUID is already the
    // shape /cases/:caseId expects; sysidToUuid only rewrites a bare
    // 32-character id.
    if (row.caseSysId) {
      window.open(`/cases/${sysidToUuid(row.caseSysId)}`, "_blank");
    } else {
      setErrorMessage("Case not found.");
    }
  };

  const handleDownload = () => {
    setDownloadError("");
    flushSync(() => setPrinting(true));
    void downloadPDF()
      .catch(() => setDownloadError("Could not download the time report."))
      .finally(() => setPrinting(false));
  };

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 2.5 }}>
      <Button
        variant="text"
        size="small"
        className="csm-print-hide"
        startIcon={<ArrowLeft size={16} />}
        onClick={() => navigate(projectId ? `/customers/projects/${projectId}` : "/customers/projects")}
        sx={{ alignSelf: "flex-start" }}
      >
        Back
      </Button>

      {!data && (
        <Box>
          <Typography variant="h5">Time report</Typography>
          <Typography variant="body2" color="text.secondary">
            {isLoading ? "Building the time report. This can take a moment." : "Time logged against this project."}
          </Typography>
        </Box>
      )}

      {data && (
        <Box className="csm-print-hide" sx={{ display: "flex", gap: 1, alignItems: "center", flexWrap: "wrap" }}>
          <ReportToggle isCustomerReport={isCustomerReport} onChange={setCustomerReport} />
        </Box>
      )}

      {errorMessage && (
        <Alert severity="warning" className="csm-print-hide" onClose={() => setErrorMessage(null)}>
          {errorMessage}
        </Alert>
      )}
      {downloadError && (
        <Alert severity="error" className="csm-print-hide" onClose={() => setDownloadError("")}>
          {downloadError}
        </Alert>
      )}

      {isLoading && (
        <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
          <Skeleton variant="rounded" height={220} />
          <Skeleton variant="rounded" height={280} />
        </Box>
      )}

      {isError && <QueryErrorState message="Could not load the time report." error={error} onRetry={() => void refetch()} />}

      {data && (
        <Box id="timelogs-report" sx={{ display: "flex", flexDirection: "column", gap: 2.5 }}>
          <Box sx={{ display: "flex", alignItems: "flex-start", justifyContent: "space-between", gap: 2, flexWrap: "wrap" }}>
            <Box sx={{ minWidth: 0 }}>
              <Typography variant="h5">{isCustomerReport ? "Query hours report" : "Time report"}</Typography>
              <Typography variant="body2" color="text.secondary">
                {data.projectName?.trim() || "Time logged against this project."}
              </Typography>
            </Box>
            {!printing && (
              <Button variant="contained" className="csm-print-hide" onClick={handleDownload} sx={{ flexShrink: 0 }}>
                Download PDF
              </Button>
            )}
          </Box>
          <ProjectCard data={data} />
          <CasesCard
            key={isCustomerReport ? "customer" : "internal"}
            cases={cases}
            linkedCaseNumbers={new Set(timeCardCases.map((entry) => entry.caseItem.caseNumber))}
            isCustomerReport={isCustomerReport}
            showAll={printing}
            onOpenCase={handleOpenCase}
          />
          {timeCardCases.map(({ caseItem, cards }) => (
            <TimeCardsCard key={caseItem.caseId || caseItem.caseNumber} caseItem={caseItem} cards={cards} isCustomerReport={isCustomerReport} />
          ))}
        </Box>
      )}
    </Box>
  );
}
