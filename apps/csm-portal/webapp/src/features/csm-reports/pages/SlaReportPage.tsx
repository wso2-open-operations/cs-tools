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

// SLA report: the response, workaround and resolution clocks of the cases a
// project opened in a period, read from Postgres by entity-service. The page,
// its states and the PDF export follow the Time report.
import { useState, type JSX, type ReactNode } from "react";
import {
  Alert,
  Box,
  Button,
  Card,
  Chip,
  CircularProgress,
  Skeleton,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TablePagination,
  TableRow,
  TextField,
  Typography,
  useTheme,
  type ChipProps,
} from "@wso2/oxygen-ui";
import { ArrowLeft } from "@wso2/oxygen-ui-icons-react";
import DOMPurify from "dompurify";
import { useParams } from "react-router";
import QueryErrorState from "@components/QueryErrorState";
import { useGetSlaReport } from "@features/csm-reports/api/useGetSlaReport";
import { slaReportPDF } from "@features/csm-reports/components/SlaReportPDF";
import type {
  SlaClockStatus,
  SlaReportCase,
  SlaReportClock,
  SlaReportDetails,
  SlaReportMetric,
  SlaReportPeriodParams,
  SlaReportSummaryRow,
} from "@features/csm-reports/api/reportTypes";
import { formatPeriod, formatPeriodDate, quarterStart, toDateInputValue } from "@features/csm-reports/utils/period";
import { sysidToUuid } from "@features/csm-cases/utils/inlineImages";
import { useNavTransition } from "@hooks/useNavTransition";

const CASE_ROWS_PER_PAGE = [10, 20, 50];

const STATUS_COLOR: Record<Exclude<SlaClockStatus, "">, ChipProps["color"]> = {
  Met: "success",
  Missed: "error",
  Running: "info",
  Paused: "warning",
  Cancelled: "default",
};

function display(value?: string | null): string {
  const trimmed = value?.trim();
  return trimmed ? trimmed : "—";
}

/** The default period: the current quarter so far. */
function defaultPeriod(): SlaReportPeriodParams {
  const now = new Date();
  return { from: toDateInputValue(quarterStart(now)), to: toDateInputValue(now) };
}

function SlaPeriodControl({
  from,
  to,
  onChange,
  onReset,
}: {
  from: string;
  to: string;
  onChange: (next: SlaReportPeriodParams) => void;
  onReset: () => void;
}): JSX.Element {
  const invalid = Boolean(from && to && from > to);
  return (
    <Box role="group" aria-label="Report period" sx={{ display: "flex", gap: 1, alignItems: "flex-start", flexWrap: "wrap" }}>
      <TextField
        type="date"
        size="small"
        label="From"
        value={from}
        error={invalid}
        onChange={(e) => onChange({ from: e.target.value, to })}
        slotProps={{ inputLabel: { shrink: true }, htmlInput: { max: to || undefined } }}
        sx={{ width: 170 }}
      />
      <TextField
        type="date"
        size="small"
        label="To"
        value={to}
        error={invalid}
        helperText={invalid ? "To must be on or after From." : undefined}
        onChange={(e) => onChange({ from, to: e.target.value })}
        slotProps={{ inputLabel: { shrink: true }, htmlInput: { min: from || undefined } }}
        sx={{ width: 170 }}
      />
      <Button variant="text" size="small" onClick={onReset} sx={{ height: 40 }}>
        This quarter
      </Button>
    </Box>
  );
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

function ProjectCard({ data }: { data: SlaReportDetails }): JSX.Element {
  return (
    <Card sx={{ p: 2.5, display: "flex", flexDirection: "column", gap: 2 }}>
      <Typography variant="subtitle2">Project</Typography>
      <Box sx={{ display: "grid", gap: 2, gridTemplateColumns: { xs: "1fr", sm: "repeat(4, minmax(0, 1fr))" } }}>
        <MetaCell label="Project name">
          <Typography variant="body2">{display(data.projectName)}</Typography>
        </MetaCell>
        <MetaCell label="Project key">
          <Typography variant="body2" sx={{ fontFamily: "monospace" }}>
            {display(data.projectKey)}
          </Typography>
        </MetaCell>
        <MetaCell label="From">
          <Typography variant="body2">{display(formatPeriodDate(data.periodStart))}</Typography>
        </MetaCell>
        <MetaCell label="To">
          <Typography variant="body2">{display(formatPeriodDate(data.periodEnd))}</Typography>
        </MetaCell>
      </Box>
      <Typography variant="caption" color="text.secondary">
        Cases opened during the report period.
      </Typography>
    </Card>
  );
}

function SummaryCell({ metric }: { metric: SlaReportMetric }): JSX.Element {
  return (
    <TableCell>
      <Typography variant="body2">{metric.elapsed}</Typography>
      <Typography variant="caption" color="text.secondary" component="div">
        Target: {metric.target}
      </Typography>
      <Typography variant="caption" color="text.secondary" component="div">
        {metric.met} met · {metric.missed} missed · {metric.running} running
      </Typography>
    </TableCell>
  );
}

function SummaryCard({ rows }: { rows: SlaReportSummaryRow[] }): JSX.Element {
  return (
    <Card sx={{ p: 2.5, display: "flex", flexDirection: "column", gap: 2 }}>
      <Box>
        <Typography variant="subtitle2">95th percentile of elapsed time</Typography>
        <Typography variant="caption" color="text.secondary">
          Of the clocks that finished. Missed counts clocks past their target, including ones still running.
        </Typography>
      </Box>
      <TableContainer>
        <Table size="small">
          <TableHead>
            <TableRow>
              <TableCell>Case type / severity</TableCell>
              <TableCell>Cases</TableCell>
              <TableCell>Response</TableCell>
              <TableCell>Workaround</TableCell>
              <TableCell>Resolution</TableCell>
            </TableRow>
          </TableHead>
          <TableBody>
            {rows.length === 0 ? (
              <EmptyRow colSpan={5} label="No cases were opened in this period." />
            ) : (
              rows.map((row) => (
                <TableRow key={`${row.caseType}-${row.severity}`}>
                  <TableCell>{display(row.label)}</TableCell>
                  <TableCell>{row.cases}</TableCell>
                  <SummaryCell metric={row.response} />
                  <SummaryCell metric={row.workaround} />
                  <SummaryCell metric={row.resolution} />
                </TableRow>
              ))
            )}
          </TableBody>
        </Table>
      </TableContainer>
    </Card>
  );
}

function ClockCell({ clock }: { clock: SlaReportClock }): JSX.Element {
  return (
    <TableCell>
      <Box sx={{ display: "flex", alignItems: "center", gap: 0.75, flexWrap: "wrap" }}>
        <Typography variant="body2">{display(clock.elapsed)}</Typography>
        {clock.status && <Chip size="small" variant="outlined" color={STATUS_COLOR[clock.status]} label={clock.status} />}
      </Box>
    </TableCell>
  );
}

function CasesCard({
  cases,
  onOpenCase,
}: {
  cases: SlaReportCase[];
  onOpenCase: (row: SlaReportCase) => void;
}): JSX.Element {
  const [page, setPage] = useState(0);
  const [rowsPerPage, setRowsPerPage] = useState(CASE_ROWS_PER_PAGE[0]);

  const pageCount = Math.max(1, Math.ceil(cases.length / rowsPerPage));
  const safePage = Math.min(page, pageCount - 1);
  const visibleCases = cases.slice(safePage * rowsPerPage, safePage * rowsPerPage + rowsPerPage);

  return (
    <Card sx={{ p: 2.5, display: "flex", flexDirection: "column", gap: 2 }}>
      <Typography variant="subtitle2">Cases</Typography>
      <TableContainer>
        <Table size="small">
          <TableHead>
            <TableRow>
              <TableCell>Case</TableCell>
              <TableCell>Type</TableCell>
              <TableCell>Severity</TableCell>
              <TableCell>State</TableCell>
              <TableCell>Opened</TableCell>
              <TableCell>Response</TableCell>
              <TableCell>Workaround</TableCell>
              <TableCell>Resolution</TableCell>
            </TableRow>
          </TableHead>
          <TableBody>
            {visibleCases.length === 0 ? (
              <EmptyRow colSpan={8} label="No cases were opened in this period." />
            ) : (
              visibleCases.map((row) => (
                <TableRow key={row.caseId || row.caseNumber}>
                  <TableCell>
                    <Button variant="text" size="small" sx={{ p: 0, minWidth: 0 }} onClick={() => onOpenCase(row)}>
                      {display(row.caseNumber)}
                    </Button>
                  </TableCell>
                  <TableCell>{display(row.caseType)}</TableCell>
                  <TableCell>{display(row.severity)}</TableCell>
                  <TableCell>{row.state && row.state !== "—" ? <Chip size="small" variant="outlined" label={row.state} /> : "—"}</TableCell>
                  <TableCell>{display(formatPeriodDate(row.opened))}</TableCell>
                  <ClockCell clock={row.response} />
                  <ClockCell clock={row.workaround} />
                  <ClockCell clock={row.resolution} />
                </TableRow>
              ))
            )}
          </TableBody>
        </Table>
      </TableContainer>
      {cases.length > 0 && (
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

export default function SlaReportPage(): JSX.Element {
  const { projectId: id } = useParams<{ projectId: string }>();
  const projectId = id ? DOMPurify.sanitize(id) : "";
  const navigate = useNavTransition();
  const theme = useTheme();
  const [errorMessage, setErrorMessage] = useState<string | null>(null);
  const [downloadError, setDownloadError] = useState("");
  const [downloading, setDownloading] = useState(false);
  // Draft inputs are what the user typed; `applied` is the last valid range
  // actually requested, so an invalid draft is never sent.
  const [draft, setDraft] = useState<SlaReportPeriodParams>(defaultPeriod);
  const [applied, setApplied] = useState<SlaReportPeriodParams>(draft);

  const handlePeriodChange = (next: SlaReportPeriodParams) => {
    setDraft(next);
    if (next.from && next.to && next.from <= next.to) setApplied(next);
  };

  const handleReset = () => {
    const next = defaultPeriod();
    setDraft(next);
    setApplied(next);
  };

  const { data, isLoading, isError, error, refetch, isPlaceholderData } = useGetSlaReport(projectId, applied);

  // Keep the toolbar once the report has loaded, and while the error state is
  // shown, so a failed uncached period never strands the user without controls.
  const [hasLoadedOnce, setHasLoadedOnce] = useState(false);
  if (data && !hasLoadedOnce) setHasLoadedOnce(true);
  const showControls = Boolean(data) || hasLoadedOnce || isError;

  const handleOpenCase = (row: SlaReportCase) => {
    // caseId is entity-service's case id; a dashed UUID is already the shape
    // /cases/:caseId expects, and sysidToUuid only rewrites a bare 32-character id.
    if (row.caseId) {
      window.open(`/cases/${sysidToUuid(row.caseId)}`, "_blank", "noopener,noreferrer");
    } else {
      setErrorMessage("Case not found.");
    }
  };

  const handleDownload = () => {
    if (!data) return;
    setDownloadError("");
    setDownloading(true);
    void slaReportPDF(data, theme.palette.primary.main)
      .catch(() => setDownloadError("Could not download the SLA report."))
      .finally(() => setDownloading(false));
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
          <Typography variant="h5">SLA report</Typography>
          <Typography variant="body2" color="text.secondary">
            {isLoading ? "Building the SLA report. This can take a moment." : "How this project's cases met their SLA targets."}
          </Typography>
        </Box>
      )}

      {showControls && (
        <Box className="csm-print-hide" sx={{ display: "flex", gap: 1, alignItems: "center", flexWrap: "wrap" }}>
          <SlaPeriodControl from={draft.from} to={draft.to} onChange={handlePeriodChange} onReset={handleReset} />
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
          <Skeleton variant="rounded" height={140} />
          <Skeleton variant="rounded" height={260} />
        </Box>
      )}

      {isError && <QueryErrorState message="Could not load the SLA report." error={error} onRetry={() => void refetch()} />}

      {data && (
        <Box
          aria-busy={isPlaceholderData}
          sx={{ opacity: isPlaceholderData ? 0.6 : 1, display: "flex", flexDirection: "column", gap: 2.5 }}
        >
          <Box sx={{ display: "flex", alignItems: "flex-start", justifyContent: "space-between", gap: 2, flexWrap: "wrap" }}>
            <Box sx={{ minWidth: 0 }}>
              <Typography variant="h5">SLA report</Typography>
              <Typography variant="body2" color="text.secondary">
                {data.projectName?.trim() || "How this project's cases met their SLA targets."}
              </Typography>
              <Typography variant="body2" color="text.secondary" data-testid="sla-report-period">
                {formatPeriod(data.periodStart, data.periodEnd)}
              </Typography>
            </Box>
            <Box
              component="span"
              role="status"
              sx={{
                position: "absolute",
                width: "1px",
                height: "1px",
                p: 0,
                m: "-1px",
                overflow: "hidden",
                clip: "rect(0 0 0 0)",
                whiteSpace: "nowrap",
                border: 0,
              }}
            >
              {downloading ? "Downloading PDF…" : ""}
            </Box>
            <Button
              variant="contained"
              className="csm-print-hide"
              onClick={handleDownload}
              disabled={downloading}
              startIcon={downloading ? <CircularProgress size={16} color="inherit" /> : undefined}
              sx={{ flexShrink: 0 }}
            >
              {downloading ? "Downloading…" : "Download PDF"}
            </Button>
          </Box>
          <ProjectCard data={data} />
          <SummaryCard rows={data.summary} />
          <CasesCard cases={data.cases} onOpenCase={handleOpenCase} />
        </Box>
      )}
    </Box>
  );
}
