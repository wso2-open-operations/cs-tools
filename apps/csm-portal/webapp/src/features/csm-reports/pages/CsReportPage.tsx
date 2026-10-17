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

// Customer Success report. The data and the PDF export are the Support
// Portal Lite report; the page itself uses the same oxygen-ui cards, tables,
// and type as the rest of CSM. Section class names (subscription-widget,
// cases-charts-widget, and the rest) are what CsReportPDF.ts looks up when
// it builds the download, so they stay on the cards.
import { useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState, type JSX, type ReactNode } from "react";
import {
  Accordion,
  AccordionDetails,
  AccordionSummary,
  Alert,
  Box,
  Button,
  Card,
  Chip,
  Divider,
  FormControl,
  InputLabel,
  MenuItem,
  Select,
  Skeleton,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  TextField,
  Typography,
  useTheme,
} from "@wso2/oxygen-ui";
import { ArrowLeft, ChevronDown } from "@wso2/oxygen-ui-icons-react";
import dayjs, { type Dayjs } from "dayjs";
import minMax from "dayjs/plugin/minMax";
import quarterOfYear from "dayjs/plugin/quarterOfYear";
import { DatePicker } from "@mui/x-date-pickers/DatePicker";
import { LocalizationProvider } from "@mui/x-date-pickers/LocalizationProvider";
import { AdapterDayjs } from "@mui/x-date-pickers/AdapterDayjs";
import { useParams } from "react-router";
import QueryErrorState from "@components/QueryErrorState";
import { useGetCsReportDetails } from "@features/csm-reports/api/useGetCsReportDetails";
import { csReportPDF } from "@features/csm-reports/components/CsReportPDF";
import BarChart from "@features/csm-reports/components/CSBarChart";
import CSPieChart from "@features/csm-reports/components/CSPieChart";
import CSLineChart from "@features/csm-reports/components/CSLineChart";
import type { ProductDetails, ProjectDeployment } from "@features/csm-reports/api/reportTypes";
import { useNavTransition } from "@hooks/useNavTransition";

dayjs.extend(minMax);
dayjs.extend(quarterOfYear);

function getLastFourQuarters() {
  const quarters: { label: string; value: string }[] = [];
  let current = dayjs().startOf("quarter");
  let count = 0;
  while (count < 4) {
    const year = current.year();
    const start = current.startOf("quarter");
    const end = current.endOf("quarter");
    if (end.isBefore(dayjs())) {
      quarters.push({
        label: `${year} Q${start.month() / 3 + 1} — ${start.format("MMMM")} to ${end.format("MMMM")}`,
        value: `${start.format("YYYY-MM-DD")}_${end.format("YYYY-MM-DD")}`,
      });
      count++;
    }
    current = current.subtract(3, "months");
  }
  return quarters;
}

function display(value?: string | null): string {
  const trimmed = value?.trim();
  return trimmed ? trimmed : "—";
}

function formatSubscriptionType(value?: string): string {
  if (!value?.trim()) return "—";
  return value.replace(/_/g, " ").replace(/\b\w/g, (c) => c.toUpperCase());
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

function SectionCard({
  className,
  title,
  children,
}: {
  className: string;
  title: string;
  children: ReactNode;
}): JSX.Element {
  return (
    <Card className={className} sx={{ p: 2.5, display: "flex", flexDirection: "column", gap: 2 }}>
      <Typography className="widget-title" variant="subtitle2">
        {title}
      </Typography>
      {children}
    </Card>
  );
}

function LocalImage({
  url,
  alt,
  onFile,
  onClear,
}: {
  url: string;
  alt: string;
  onFile: (file: File | undefined) => void;
  onClear: () => void;
}): JSX.Element {
  if (url) {
    return (
      <Box sx={{ display: "flex", flexDirection: "column", gap: 1, alignItems: "flex-start" }}>
        <Box component="img" src={url} alt={alt} sx={{ maxWidth: 480, maxHeight: 320, borderRadius: 1 }} />
        <Button variant="text" size="small" onClick={onClear}>
          Remove image
        </Button>
      </Box>
    );
  }
  return (
    <Button variant="outlined" component="label" size="small" sx={{ alignSelf: "flex-start" }}>
      Upload image
      <input
        hidden
        type="file"
        accept="image/png,image/jpeg,image/gif,image/webp"
        onChange={(e) => {
          onFile(e.target.files?.[0]);
          e.target.value = "";
        }}
      />
    </Button>
  );
}

function EngagementDescription({ text }: { text: string }): JSX.Element {
  const [open, setOpen] = useState(false);
  const [overflows, setOverflows] = useState(false);
  const ref = useRef<HTMLElement>(null);

  useLayoutEffect(() => {
    const el = ref.current;
    if (!el || open) return;
    const measure = () => setOverflows(el.scrollHeight > el.clientHeight + 1);
    measure();
    const observer = new ResizeObserver(measure);
    observer.observe(el);
    return () => observer.disconnect();
  }, [text, open]);

  return (
    <Box>
      <Typography
        ref={ref}
        component="p"
        className="engagement-description"
        variant="body2"
        sx={{
          m: 0,
          wordBreak: "break-word",
          overflowWrap: "anywhere",
          whiteSpace: "normal",
          ...(open
            ? {}
            : {
                display: "-webkit-box",
                WebkitLineClamp: 2,
                WebkitBoxOrient: "vertical",
                overflow: "hidden",
              }),
        }}
      >
        {text}
      </Typography>
      {(overflows || open) && (
        <Button
          className="engagement-show-more csm-print-hide"
          size="small"
          variant="text"
          onClick={() => setOpen((value) => !value)}
          sx={{ p: 0, minWidth: 0, mt: 0.25, fontSize: "0.75rem", textTransform: "none" }}
        >
          {open ? "Show less" : "Show more"}
        </Button>
      )}
    </Box>
  );
}

function getReportMonth(opened: string): number | null {
  const match = /^(\d{4})-(\d{2})-\d{2}/.exec(opened);
  if (!match) return null;
  const month = Number(match[2]) - 1;
  return month >= 0 && month <= 11 ? month : null;
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

export default function CsReportPage(): JSX.Element {
  const theme = useTheme();
  const { projectId, sysId: id } = useParams<{ projectId: string; sysId: string }>();
  const navigate = useNavTransition();
  const projectSysId = id || "";
  const lastFourQuarters = getLastFourQuarters();
  const [dateRangeError, setDateRangeError] = useState("");
  const [dateComparisonError, setDateComparisonError] = useState("");
  const [selectedOption, setSelectedOption] = useState(lastFourQuarters[0]?.value ?? "custom");
  const [from, setFrom] = useState<Dayjs | null>(
    lastFourQuarters[0] ? dayjs(lastFourQuarters[0].value.split("_")[0]) : dayjs(),
  );
  const [to, setTo] = useState<Dayjs | null>(
    lastFourQuarters[0] ? dayjs(lastFourQuarters[0].value.split("_")[1]) : dayjs(),
  );
  const [useCustomDates, setUseCustomDates] = useState(false);
  const [showReport, setShowReport] = useState(false);
  const [deploymentImage, setDeploymentImage] = useState("");
  const [businessOverviewText, setBusinessOverviewText] = useState("");
  const [notesImage, setNotesImage] = useState("");
  const [notesText, setNotesText] = useState("");
  const businessOverviewTextareaRef = useRef<HTMLTextAreaElement | null>(null);
  const notesTextareaRef = useRef<HTMLTextAreaElement | null>(null);
  const [submittedRange, setSubmittedRange] = useState<{ from: string; to: string } | null>(null);

  const {
    data: csReportData,
    isLoading,
    isError,
    error,
    refetch: refetchReportData,
  } = useGetCsReportDetails({
    projectSysId,
    from: submittedRange?.from ?? "",
    to: submittedRange?.to ?? "",
    enabled: submittedRange !== null,
  });
  const reportFrom = submittedRange ? dayjs(submittedRange.from) : from;
  const reportTo = submittedRange ? dayjs(submittedRange.to) : to;

  const handleOptionChange = (value: string) => {
    setSelectedOption(value);
    if (value === "custom") {
      const currentDate = dayjs();
      setFrom(currentDate);
      setTo(currentDate);
      setUseCustomDates(true);
    } else {
      setUseCustomDates(false);
      const [start, end] = value.split("_");
      setFrom(dayjs(start));
      setTo(dayjs(end));
    }
  };

  const handleTextareaChange = useCallback((e: React.ChangeEvent<HTMLTextAreaElement>) => {
    if (businessOverviewTextareaRef.current) {
      const cursorPosition = businessOverviewTextareaRef.current.selectionStart;
      setBusinessOverviewText(e.target.value);
      setTimeout(() => {
        if (businessOverviewTextareaRef.current) {
          businessOverviewTextareaRef.current.setSelectionRange(cursorPosition, cursorPosition);
          businessOverviewTextareaRef.current.focus();
        }
      }, 0);
    }
  }, []);

  const handleNotesTextareaChange = useCallback((e: React.ChangeEvent<HTMLTextAreaElement>) => {
    if (notesTextareaRef.current) {
      const cursorPosition = notesTextareaRef.current.selectionStart;
      setNotesText(e.target.value);
      setTimeout(() => {
        if (notesTextareaRef.current) {
          notesTextareaRef.current.setSelectionRange(cursorPosition, cursorPosition);
          notesTextareaRef.current.focus();
        }
      }, 0);
    }
  }, []);

  const [uploadError, setUploadError] = useState("");
  const [downloadError, setDownloadError] = useState("");

  // Blob URLs are local. Revoke the previous one when it is replaced, and the
  // current one when the page unmounts. A data URL preview is what left image
  // requests sitting in the network tab.
  useEffect(() => {
    if (!deploymentImage.startsWith("blob:")) return;
    return () => URL.revokeObjectURL(deploymentImage);
  }, [deploymentImage]);
  useEffect(() => {
    if (!notesImage.startsWith("blob:")) return;
    return () => URL.revokeObjectURL(notesImage);
  }, [notesImage]);

  const handleImageUpload = (file: File | undefined, section: "deployment" | "notes") => {
    if (!file) return;
    if (!/^image\/(png|jpeg|gif|webp)$/.test(file.type)) {
      setUploadError("Choose a PNG, JPEG, GIF, or WebP image.");
      return;
    }
    const url = URL.createObjectURL(file);
    const img = new Image();
    img.onload = () => {
      setUploadError("");
      if (section === "deployment") setDeploymentImage(url);
      else setNotesImage(url);
    };
    img.onerror = () => {
      URL.revokeObjectURL(url);
      setUploadError("That image could not be opened.");
    };
    img.src = url;
  };

  const handleDownloadPDF = () => {
    setDownloadError("");
    void csReportPDF(
      "cs-report",
      csReportData?.subscriptionDetails?.accountName ?? "Unknown Account",
      csReportData?.subscriptionDetails?.projectKey ?? "Unknown Project Key",
      csReportData?.slaDetails?.slaRecords ?? [],
      reportFrom,
      reportTo,
      notesText,
      notesImage,
      deploymentImage,
      businessOverviewText,
      theme.palette.primary.main,
    ).catch((err: unknown) => {
      setDownloadError(err instanceof Error ? err.message : "Could not build the PDF.");
    });
  };

  const handleSubmit = () => {
    if (useCustomDates && from && to) {
      const diffInMonths = to.diff(from, "month");
      if (diffInMonths > 6) {
        setDateRangeError("The date range cannot exceed 6 months.");
        return;
      }
      if (from.isAfter(to)) {
        setDateComparisonError("The From date must be earlier than the To date.");
        return;
      }
    }
    if (!from || !to) return;
    setDateComparisonError("");
    setDateRangeError("");
    setShowReport(true);
    const nextRange = { from: from.format("YYYY-MM-DD"), to: to.format("YYYY-MM-DD") };
    if (submittedRange?.from === nextRange.from && submittedRange?.to === nextRange.to) {
      void refetchReportData();
      return;
    }
    setSubmittedRange(nextRange);
  };

  const transformedData = useMemo(() => {
    if (!csReportData || !Array.isArray(csReportData.casesRecords)) return null;

    const filteredCaseRecords = csReportData.casesRecords.filter(
      (record) => record.caseType === "Incident" || record.caseType === "Query",
    );

    const casesByDayAndState = filteredCaseRecords.reduce(
      (dayWiseCases: { day: string; opened: number; closed: number }[], record) => {
        const openedDate = record.opened.split(" ")[0];
        const caseState = record.caseState;
        let dayStateCount = dayWiseCases.find((entry) => entry.day === openedDate);
        if (!dayStateCount) {
          dayStateCount = { day: openedDate, opened: 0, closed: 0 };
          dayWiseCases.push(dayStateCount);
        }
        if (caseState === "Open") dayStateCount.opened++;
        else if (caseState === "Closed") dayStateCount.closed++;
        return dayWiseCases;
      },
      [],
    );

    const casesByDeployment = filteredCaseRecords.reduce((acc: { [key: string]: number }, record) => {
      if (!record.deployment) return acc;
      acc[record.deployment] = (acc[record.deployment] || 0) + 1;
      return acc;
    }, {});

    const deploymentNames = Object.keys(casesByDeployment);
    const distinctPriorities = Array.from(new Set(filteredCaseRecords.map((record) => record.casePriority)));

    const casesByMonthAndPriority = filteredCaseRecords.reduce(
      (acc: { [key: number]: { [key: string]: number } }, record) => {
        const month = getReportMonth(record.opened);
        if (month === null) return acc;
        const priority = record.casePriority;
        acc[month] = acc[month] || {};
        acc[month][priority] = (acc[month][priority] || 0) + 1;
        return acc;
      },
      {},
    );

    const distinctProductNames = Array.from(new Set(filteredCaseRecords.map((record) => record.productName)));

    const casesByMonthAndProduct = filteredCaseRecords.reduce(
      (acc: { [key: number]: { [key: string]: number } }, record) => {
        const month = getReportMonth(record.opened);
        if (month === null) return acc;
        const product = record.productName;
        acc[month] = acc[month] || {};
        acc[month][product] = (acc[month][product] || 0) + 1;
        return acc;
      },
      {},
    );

    const MonthlyCases = csReportData.monthlyCounts
      .slice()
      .reverse()
      .map((entry) => ({
        yearAndMonth: entry.yearAndMonth,
        incidentCount: entry.counts.incidentCount || 0,
        queryCount: entry.counts.queryCount || 0,
      }));

    return {
      casesByDayAndState,
      casesByDeployment,
      deploymentNames,
      casesByMonthAndPriority,
      distinctPriorities,
      casesByMonthAndProduct,
      distinctProductNames,
      MonthlyCases,
    };
  }, [csReportData]);

  const subscription = csReportData?.subscriptionDetails;
  const deployments = csReportData?.projectDeployments ?? [];
  const allProducts = deployments.flatMap((d) => d.products || []);
  const uniqueProducts = Array.from(
    allProducts.reduce((acc, product) => {
      if (!acc.has(product.name)) acc.set(product.name, product);
      return acc;
    }, new Map<string, ProductDetails>()),
  );
  const engagementCases = (csReportData?.casesRecords ?? []).filter((record) => record.caseType === "Engagement");
  const rangeLabel = reportFrom && reportTo ? `${reportFrom.format("MMM D, YYYY")} – ${reportTo.format("MMM D, YYYY")}` : "";

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

      <Box sx={{ minWidth: 0 }}>
        <Typography variant="h5">Customer Success Report</Typography>
        <Typography variant="body2" color="text.secondary">
          {display(subscription?.projectName) === "—" ? "Choose a date range to load the report." : display(subscription?.projectName)}
          {rangeLabel && showReport ? ` · ${rangeLabel}` : ""}
        </Typography>
      </Box>
      {uploadError && (
        <Alert severity="error" className="csm-print-hide" onClose={() => setUploadError("")}>
          {uploadError}
        </Alert>
      )}

      <Card className="csm-print-hide" sx={{ p: 2.5, display: "flex", flexDirection: "column", gap: 2 }}>
        <Typography variant="subtitle2">Date range</Typography>
        <Box sx={{ display: "flex", gap: 2, flexWrap: "wrap", alignItems: "flex-start" }}>
          <FormControl size="small" sx={{ minWidth: 280 }}>
            <InputLabel id="cs-report-range-label">Range</InputLabel>
            <Select
              labelId="cs-report-range-label"
              label="Range"
              value={selectedOption}
              onChange={(event) => handleOptionChange(String(event.target.value))}
            >
              {lastFourQuarters.map((quarter) => (
                <MenuItem key={quarter.value} value={quarter.value}>
                  {quarter.label}
                </MenuItem>
              ))}
              <MenuItem value="custom">Custom date range</MenuItem>
            </Select>
          </FormControl>
          {useCustomDates && (
            <LocalizationProvider dateAdapter={AdapterDayjs}>
              <DatePicker label="From" value={from} onChange={setFrom} slotProps={{ textField: { size: "small" } }} />
              <DatePicker label="To" value={to} onChange={setTo} slotProps={{ textField: { size: "small" } }} />
            </LocalizationProvider>
          )}
          <Button variant="outlined" onClick={handleSubmit}>
            Generate
          </Button>
        </Box>
        {dateRangeError && <Alert severity="error">{dateRangeError}</Alert>}
        {dateComparisonError && <Alert severity="error">{dateComparisonError}</Alert>}
      </Card>

      {showReport && isLoading && (
        <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
          <Skeleton variant="rounded" height={180} />
          <Skeleton variant="rounded" height={280} />
        </Box>
      )}

      {showReport && isError && (
        <QueryErrorState message="Could not load the CS report." error={error} onRetry={() => void refetchReportData()} />
      )}

      {showReport && csReportData && (
        <Box id="cs-report" sx={{ display: "flex", flexDirection: "column", gap: 2.5 }}>
          <SectionCard className="subscription-widget" title="Subscription">
            <Box
              sx={{
                display: "grid",
                gap: 2,
                gridTemplateColumns: { xs: "1fr", sm: "repeat(2, minmax(0, 1fr))", md: "repeat(3, minmax(0, 1fr))" },
              }}
            >
              <MetaCell label="Project name">
                <Typography variant="body2">{display(subscription?.projectName)}</Typography>
              </MetaCell>
              <MetaCell label="Project key">
                <Typography variant="body2" sx={{ fontFamily: "monospace" }}>
                  {display(subscription?.projectKey)}
                </Typography>
              </MetaCell>
              <MetaCell label="Project type">
                <Typography variant="body2">{formatSubscriptionType(subscription?.projectType)}</Typography>
              </MetaCell>
              <MetaCell label="Subscription start">
                <Typography variant="body2">{display(subscription?.startDate)}</Typography>
              </MetaCell>
              <MetaCell label="Subscription end">
                <Typography variant="body2">{display(subscription?.endDate)}</Typography>
              </MetaCell>
              <MetaCell label="Support tier">
                <Typography variant="body2">{display(subscription?.supportTier)}</Typography>
              </MetaCell>
              <MetaCell label="Account">
                <Typography variant="body2">{display(subscription?.accountName)}</Typography>
              </MetaCell>
              <MetaCell label="Query hours">
                <Typography variant="body2">
                  {display(subscription?.totalQueryHours)}
                  {subscription?.consumedQueryHours ? ` (${subscription.consumedQueryHours} utilized)` : ""}
                </Typography>
              </MetaCell>
            </Box>
            <Box sx={{ display: "flex", flexDirection: "column", gap: 1 }}>
              <Typography variant="body2" color="text.secondary">
                Products
              </Typography>
              <TableContainer>
                <Table size="small">
                  <TableHead>
                    <TableRow>
                      <TableCell>Product</TableCell>
                      <TableCell>Version</TableCell>
                    </TableRow>
                  </TableHead>
                  <TableBody>
                    {uniqueProducts.length === 0 ? (
                      <EmptyRow colSpan={2} label="No products for this project." />
                    ) : (
                      uniqueProducts.map(([name, product]) => (
                        <TableRow key={name}>
                          <TableCell sx={{ overflowWrap: "anywhere" }}>{name}</TableCell>
                          <TableCell>{product.version ? `v${product.version}` : "—"}</TableCell>
                        </TableRow>
                      ))
                    )}
                  </TableBody>
                </Table>
              </TableContainer>
            </Box>
          </SectionCard>

          {allProducts.length > 0 && (
            <SectionCard className="current-products-widget" title="Usage">
              <Box
                sx={{
                  display: "grid",
                  gap: 2,
                  gridTemplateColumns: { xs: "1fr 1fr", sm: "repeat(3, minmax(0, 1fr))", md: "repeat(4, minmax(0, 1fr))" },
                }}
              >
                {allProducts.map((product, index) => (
                  <MetaCell key={`${product.name}-${index}`} label={product.cores ? `${product.cores} cores` : "Cores"}>
                    <Typography variant="body2">{product.name || "—"}</Typography>
                  </MetaCell>
                ))}
              </Box>
            </SectionCard>
          )}

          {deployments.some((d: ProjectDeployment) => d.products && d.products.length > 0) && (
            <SectionCard className="product-versions-widget" title="Product versions and EOL">
              {deployments.map((deployment, index) =>
                deployment.products.length > 0 ? (
                  <Box key={index} sx={{ display: "flex", flexDirection: "column", gap: 1 }}>
                    <Typography variant="body2" color="text.secondary">
                      {deployment.name} environment
                    </Typography>
                    {deployment.products.map((product, i) => (
                      <Accordion
                        key={`${product.name}-${product.version}-${i}`}
                        disableGutters
                        defaultExpanded
                        sx={{
                          border: "1px solid",
                          borderColor: "divider",
                          borderRadius: 1,
                          "&:before": { display: "none" },
                        }}
                      >
                        <AccordionSummary expandIcon={<ChevronDown size={16} />}>
                          <Box sx={{ display: "flex", alignItems: "center", gap: 1, minWidth: 0, flexWrap: "wrap" }}>
                            <Typography variant="body2">
                              {product.name}
                              {product.version ? ` v${product.version}` : ""}
                            </Typography>
                            {product.supportStatus ? (
                              <Chip size="small" variant="outlined" label={product.supportStatus} />
                            ) : null}
                          </Box>
                        </AccordionSummary>
                        <AccordionDetails>
                          <Box
                            data-eol-environment={deployment.name}
                            data-eol-product={product.name}
                            data-eol-version={product.version || ""}
                            data-eol-status={product.supportStatus || ""}
                            data-eol-earliest={product.earliestPossibleSupportEOLDate || ""}
                            data-eol-date={product.eolDate || ""}
                            sx={{ display: "grid", gap: 2, gridTemplateColumns: { xs: "1fr", sm: "1fr 1fr" } }}
                          >
                            <MetaCell label="Earliest support EOL">
                              <Typography variant="body2" sx={{ overflowWrap: "anywhere" }}>
                                {product.earliestPossibleSupportEOLDate || "—"}
                              </Typography>
                            </MetaCell>
                            <MetaCell label="Support EOL">
                              <Typography variant="body2" sx={{ overflowWrap: "anywhere" }}>
                                {product.eolDate || "—"}
                              </Typography>
                            </MetaCell>
                          </Box>
                        </AccordionDetails>
                      </Accordion>
                    ))}
                  </Box>
                ) : null,
              )}
            </SectionCard>
          )}

          <SectionCard className="cases-charts-widget" title="Cases">
            <Box
              sx={{
                display: "grid",
                gridTemplateColumns: { xs: "minmax(0, 1fr)", lg: "minmax(0, 1fr) minmax(0, 1fr)" },
                gap: 4,
                alignItems: "start",
              }}
            >
              <Box sx={{ minWidth: 0, overflow: "hidden" }}>
                {transformedData?.casesByDayAndState ? (
                  <BarChart data={transformedData.casesByDayAndState} chartType="state" from={reportFrom ?? dayjs()} to={reportTo ?? dayjs()} />
                ) : (
                  <Typography variant="body2" color="text.secondary">
                    No opened or resolved cases in this range.
                  </Typography>
                )}
              </Box>
              <Box sx={{ minWidth: 0, overflow: "hidden" }}>
                <CSPieChart deploymentNames={transformedData?.deploymentNames || []} data={transformedData?.casesByDeployment || {}} />
              </Box>
              <Box sx={{ minWidth: 0, overflow: "hidden" }}>
                {transformedData?.casesByMonthAndProduct && transformedData.distinctProductNames ? (
                  <BarChart
                    data={transformedData.casesByMonthAndProduct}
                    distinctProductNames={transformedData.distinctProductNames}
                    from={reportFrom ?? dayjs()}
                    to={reportTo ?? dayjs()}
                    chartType="product"
                  />
                ) : (
                  <Typography variant="body2" color="text.secondary">
                    No product cases in this range.
                  </Typography>
                )}
              </Box>
              <Box sx={{ minWidth: 0, overflow: "hidden" }}>
                {transformedData?.casesByMonthAndPriority && transformedData.distinctPriorities ? (
                  <BarChart
                    data={transformedData.casesByMonthAndPriority}
                    distinctPriorities={transformedData.distinctPriorities}
                    from={reportFrom ?? dayjs()}
                    to={reportTo ?? dayjs()}
                    chartType="priority"
                  />
                ) : (
                  <Typography variant="body2" color="text.secondary">
                    No priority cases in this range.
                  </Typography>
                )}
              </Box>
            </Box>
          </SectionCard>

          <SectionCard className="cases-volume-widget" title="Case volume by month">
            {transformedData?.MonthlyCases && transformedData.MonthlyCases.length > 0 ? (
              <CSLineChart data={transformedData.MonthlyCases} from={reportFrom ?? dayjs().startOf("month")} to={reportTo ?? dayjs().endOf("month")} />
            ) : (
              <Typography variant="body2" color="text.secondary">
                No case volume in this range.
              </Typography>
            )}
          </SectionCard>

          <SectionCard className="incident-sla-compliance-widget" title="Incident SLA compliance">
            {csReportData.slaDetails?.slaPerformanceStats ? (
              <Box sx={{ display: "grid", gap: 2, gridTemplateColumns: { xs: "1fr", sm: "repeat(3, minmax(0, 1fr))" } }}>
                {(
                  [
                    ["Resolution", csReportData.slaDetails.slaPerformanceStats.Resolution],
                    ["Workaround", csReportData.slaDetails.slaPerformanceStats.Workaround],
                    ["Response", csReportData.slaDetails.slaPerformanceStats.Response],
                  ] as const
                ).map(([label, metric]) => (
                  <Card key={label} variant="outlined" sx={{ p: 2 }}>
                    <Typography variant="h5">{metric.percentage ? `${metric.percentage}%` : "—"}</Typography>
                    <Typography variant="body2" color="text.secondary">
                      {label}
                      {metric.fraction != null && metric.fraction !== "" ? ` (${metric.fraction})` : ""}
                    </Typography>
                  </Card>
                ))}
              </Box>
            ) : (
              <Typography variant="body2" color="text.secondary">
                No SLA statistics available.
              </Typography>
            )}
          </SectionCard>

          {deployments.some((d) => (d.products?.length ?? 0) > 0) && (
            <SectionCard className="product-updates-widget" title="Product update levels">
              {deployments.map((deployment, index) => {
                const products = deployment.products ?? [];
                if (products.length === 0) return null;
                return (
                  <Box key={index} sx={{ display: "flex", flexDirection: "column", gap: 1 }}>
                    <Typography variant="body2" color="text.secondary">
                      {deployment.name} environment
                    </Typography>
                    <TableContainer>
                      <Table size="small">
                        <TableHead>
                          <TableRow>
                            <TableCell>Product</TableCell>
                            <TableCell>Current update level</TableCell>
                          </TableRow>
                        </TableHead>
                        <TableBody>
                          {products.map((product, i) => (
                            <TableRow key={i}>
                              <TableCell>
                                {product.name}
                                {product.version ? ` v${product.version}` : ""}
                              </TableCell>
                              <TableCell>
                                {product.updateLevelInfo != null ? product.updateLevelInfo : "No update history"}
                              </TableCell>
                            </TableRow>
                          ))}
                        </TableBody>
                      </Table>
                    </TableContainer>
                  </Box>
                );
              })}
            </SectionCard>
          )}

          <SectionCard className="engagements-widget" title="Engagements">
            <TableContainer>
              <Table size="small">
                <TableHead>
                  <TableRow>
                    <TableCell>Created</TableCell>
                    <TableCell>Case</TableCell>
                    <TableCell>Description</TableCell>
                    <TableCell>Type</TableCell>
                    <TableCell>Updated</TableCell>
                  </TableRow>
                </TableHead>
                <TableBody>
                  {engagementCases.length === 0 ? (
                    <EmptyRow colSpan={5} label="No engagements in this range." />
                  ) : (
                    engagementCases.map((record, index) => (
                      <TableRow key={index}>
                        <TableCell>{record.opened || "—"}</TableCell>
                        <TableCell>{record.caseNumber || "—"}</TableCell>
                        <TableCell>
                          <EngagementDescription text={record.description || "—"} />
                        </TableCell>
                        <TableCell>{record.engagementType || "—"}</TableCell>
                        <TableCell>{record.updated || "—"}</TableCell>
                      </TableRow>
                    ))
                  )}
                </TableBody>
              </Table>
            </TableContainer>
          </SectionCard>

          <Card sx={{ p: 2.5, display: "flex", flexDirection: "column", gap: 2.5 }}>
            <Typography variant="body2" sx={{ color: "primary.main", fontStyle: "italic" }}>
              Included in the downloaded report only. This text and these images are not saved.
            </Typography>
            <Box className="deployment-image-section" sx={{ display: "flex", flexDirection: "column", gap: 1.5 }}>
              <Typography className="widget-title" variant="subtitle2">
                Deployment
              </Typography>
              <LocalImage
                url={deploymentImage}
                alt="Deployment"
                onFile={(file) => handleImageUpload(file, "deployment")}
                onClear={() => setDeploymentImage("")}
              />
            </Box>
            <Divider />
            <Box className="business-overview-section" sx={{ display: "flex", flexDirection: "column", gap: 1.5 }}>
              <Typography className="widget-title" variant="subtitle2">
                Business overview
              </Typography>
              <TextField
                inputRef={businessOverviewTextareaRef}
                className="paragraph-input"
                placeholder="Enter details"
                multiline
                minRows={4}
                fullWidth
                value={businessOverviewText}
                onChange={handleTextareaChange}
              />
            </Box>
            <Divider />
            <Box className="extra-notes-section" sx={{ display: "flex", flexDirection: "column", gap: 1.5 }}>
              <Typography className="widget-title" variant="subtitle2">
                Notes
              </Typography>
              <LocalImage
                url={notesImage}
                alt="Notes"
                onFile={(file) => handleImageUpload(file, "notes")}
                onClear={() => setNotesImage("")}
              />
              <TextField
                inputRef={notesTextareaRef}
                className="extra-notes-input"
                placeholder="Enter details"
                multiline
                minRows={4}
                fullWidth
                value={notesText}
                onChange={handleNotesTextareaChange}
              />
            </Box>
          </Card>
        </Box>
      )}

      {showReport && (
        <Box className="csm-print-hide" sx={{ display: "flex", flexDirection: "column", gap: 1, alignItems: "flex-start" }}>
          {downloadError && (
            <Alert severity="error" onClose={() => setDownloadError("")}>
              {downloadError}
            </Alert>
          )}
          <Button variant="contained" onClick={handleDownloadPDF} disabled={!csReportData}>
            Download PDF
          </Button>
        </Box>
      )}
    </Box>
  );
}
