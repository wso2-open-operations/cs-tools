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

// Ported from apps/support-portal-lite/webapp's CsReportPage (itself from
// SupportPortalLite's CSReport.tsx). UI/JSX unchanged; data layer rewritten
// onto useBackendApi()/React Query — see useGetCsReportDetails.ts. No
// SplShell wrapper. Route: spl/projects/:projectId/cs-report/:sysId.
import { useCallback, useMemo, useRef, useState, type JSX } from "react";
import {
  Box,
  Button,
  FormControl,
  Grid,
  MenuItem,
  Paper,
  Select,
  Tooltip,
  Typography,
  type SelectChangeEvent,
} from "@mui/material";
import DownloadIcon from "@mui/icons-material/Download";
import dayjs, { type Dayjs } from "dayjs";
import minMax from "dayjs/plugin/minMax";
import quarterOfYear from "dayjs/plugin/quarterOfYear";
import { DatePicker } from "@mui/x-date-pickers/DatePicker";
import { LocalizationProvider } from "@mui/x-date-pickers/LocalizationProvider";
import { AdapterDayjs } from "@mui/x-date-pickers/AdapterDayjs";
import { useParams } from "react-router";
import { useGetCsReportDetails } from "@features/sales-sa/reports/api/useGetCsReportDetails";
import { csReportPDF } from "@features/sales-sa/reports/components/CsReportPDF";
import BarChart from "@features/sales-sa/reports/components/CSBarChart";
import CSPieChart from "@features/sales-sa/reports/components/CSPieChart";
import CSLineChart from "@features/sales-sa/reports/components/CSLineChart";
import type { ProjectDeployment } from "@features/sales-sa/reports/api/reportTypes";
import { ThemeProvider } from "@mui/material/styles";
import { reportPaperTheme } from "@features/sales-sa/reports/reportPaperTheme";
import "@features/sales-sa/reports/styles/CSReport.css";

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
        label: `${year} Q${start.month() / 3 + 1} - ${start.format("MMMM")} to ${end.format("MMMM")}`,
        value: `${start.format("YYYY-MM-DD")}_${end.format("YYYY-MM-DD")}`,
      });
      count++;
    }
    current = current.subtract(3, "months");
  }
  return quarters;
}

export default function CsReportPage(): JSX.Element {
  const { sysId: id } = useParams<{ sysId: string }>();
  const projectSysId = id || "";
  const lastFourQuarters = getLastFourQuarters();
  const [dateRangeError, setDateRangeError] = useState("");
  const [dateComparisonError, setDateComparisonError] = useState("");
  const [selectedOption, setSelectedOption] = useState(lastFourQuarters[0].value);
  const [from, setFrom] = useState<Dayjs | null>(dayjs(lastFourQuarters[0].value.split("_")[0]));
  const [to, setTo] = useState<Dayjs | null>(dayjs(lastFourQuarters[0].value.split("_")[1]));
  const [useCustomDates, setUseCustomDates] = useState(false);
  const [showReport, setShowReport] = useState(false);
  const [deploymentImage, setDeploymentImage] = useState("");
  const [businessOverviewText, setBusinessOverviewText] = useState("");
  const [notesImage, setNotesImage] = useState("");
  const [notesText, setNotesText] = useState("");
  const businessOverviewTextareaRef = useRef<HTMLTextAreaElement | null>(null);
  const notesTextareaRef = useRef<HTMLTextAreaElement | null>(null);
  const [submitted, setSubmitted] = useState(false);

  const { data: csReportData, refetch: refetchReportData } = useGetCsReportDetails({
    projectSysId,
    from: from?.format("YYYY-MM-DD") ?? "",
    to: to?.format("YYYY-MM-DD") ?? "",
    enabled: submitted,
  });

  const handleOptionChange = (event: SelectChangeEvent) => {
    const value = event.target.value;
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

  const handleImageUpload = (event: React.ChangeEvent<HTMLInputElement>, section: "deployment" | "notes") => {
    const file = event.target.files?.[0];
    if (!file) return;
    const reader = new FileReader();
    reader.onloadend = () => {
      const img = new Image();
      img.src = reader.result as string;
      img.onload = () => {
        const TARGET_WIDTH = 794;
        const TARGET_HEIGHT = 500;
        let width = img.width;
        let height = img.height;
        const scaleFactor = Math.min(TARGET_WIDTH / width, TARGET_HEIGHT / height);
        width *= scaleFactor;
        height *= scaleFactor;

        const canvas = document.createElement("canvas");
        canvas.width = TARGET_WIDTH;
        canvas.height = TARGET_HEIGHT;
        const ctx = canvas.getContext("2d");
        if (ctx) {
          ctx.fillStyle = "#FFFFFF";
          ctx.fillRect(0, 0, canvas.width, canvas.height);
          const offsetX = (TARGET_WIDTH - width) / 2;
          const offsetY = (TARGET_HEIGHT - height) / 2;
          ctx.drawImage(img, offsetX, offsetY, width, height);
          const resizedImage = canvas.toDataURL("image/jpeg");
          if (section === "deployment") setDeploymentImage(resizedImage);
          else setNotesImage(resizedImage);
        }
      };
    };
    reader.readAsDataURL(file);
  };

  const handleDownloadPDF = () => {
    void csReportPDF(
      "cs-report",
      csReportData?.subscriptionDetails?.accountName ?? "Unknown Account",
      csReportData?.subscriptionDetails?.projectKey ?? "Unknown Project Key",
      csReportData?.slaDetails?.slaRecords ?? [],
      from,
      to,
      notesText,
      notesImage,
      deploymentImage,
      businessOverviewText,
    );
  };

  const handleSubmit = () => {
    if (useCustomDates && from && to) {
      const diffInMonths = to.diff(from, "month");
      if (diffInMonths > 6) {
        setDateRangeError("The date range cannot exceed 6 months.");
        return;
      }
      if (from.isAfter(to)) {
        setDateComparisonError("The 'From' date must be earlier than the 'To' date.");
        return;
      }
    }
    setDateComparisonError("");
    setDateRangeError("");
    setShowReport(true);
    setSubmitted(true);
    void refetchReportData();
  };

  // Derived from csReportData — a useMemo rather than a useState+useEffect
  // pair (this codebase's lint config forbids setState-in-effect for React
  // Compiler compatibility), which is also just the more direct fit: this
  // was never anything but a pure transform of csReportData.
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
        const month = new Date(record.opened).getMonth();
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
        const month = new Date(record.opened).getMonth();
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

  // Plain JSX values, not components (see file header note on
  // react-hooks/static-components) — each is used exactly once below, so
  // there's no need for a named function wrapper at all.
  const formView = (
    <div className="form-container blurred-background">
      <Paper className="centered-form" elevation={3} sx={{ padding: "20px", width: 400, margin: "0 auto" }}>
        <Typography gutterBottom variant="h4" component="div" sx={{ textAlign: "center", marginBottom: "20px" }}>
          CS Report
        </Typography>
        <Typography variant="body1" gutterBottom sx={{ marginBottom: "20px", textAlign: "center" }}>
          Please select the date range for the CS Report
        </Typography>
        <FormControl fullWidth sx={{ marginBottom: "20px" }}>
          <Select value={selectedOption} onChange={handleOptionChange}>
            {lastFourQuarters.map((quarter) => (
              <MenuItem key={quarter.value} value={quarter.value}>
                {quarter.label}
              </MenuItem>
            ))}
            <MenuItem value="custom">Custom Date Range</MenuItem>
          </Select>
        </FormControl>
        {dateRangeError && (
          <Typography variant="body2" color="error" sx={{ marginBottom: "10px", textAlign: "center" }}>
            {dateRangeError}
          </Typography>
        )}
        {dateComparisonError && (
          <Typography variant="body2" color="error" sx={{ marginBottom: "10px", textAlign: "center" }}>
            {dateComparisonError}
          </Typography>
        )}
        {useCustomDates && (
          <LocalizationProvider dateAdapter={AdapterDayjs}>
            <Grid container spacing={2} sx={{ marginBottom: "20px" }}>
              <Grid size={{ xs: 6 }}>
                <DatePicker label="From" value={from} onChange={setFrom} slotProps={{ textField: { fullWidth: true } }} />
              </Grid>
              <Grid size={{ xs: 6 }}>
                <DatePicker label="To" value={to} onChange={setTo} slotProps={{ textField: { fullWidth: true } }} />
              </Grid>
            </Grid>
          </LocalizationProvider>
        )}
        <Button variant="contained" className="submitButton" onClick={handleSubmit}>
          Submit
        </Button>
      </Paper>
    </div>
  );

  const mainView = (
    <>
      <Box id="downloadReportButton" flex={2}>
        <Tooltip title="Download Report">
          <Button variant="contained" className="downloadButton" onClick={handleDownloadPDF}>
            <DownloadIcon />
          </Button>
        </Tooltip>
      </Box>

      {/* A printable document (captured to PDF by handleDownloadPDF below) —
          kept a fixed light "paper" theme regardless of the app's own
          light/dark mode, same convention as SlaReportPage/
          TimelogsReportPage. See reportPaperTheme.ts. */}
      <ThemeProvider theme={reportPaperTheme}>
      <div id="cs-report">
        <div className="main-view-container">
          <h1 className="main-view-header">Customer Success Report for {csReportData?.subscriptionDetails?.projectName}</h1>
          <div className="subscription-widget">
            <h2 className="widget-title">Subscription Details</h2>
            <div className="subscription-details">
              <div className="detail-item">
                <strong>Project Name</strong>
                <div className="data-info">{csReportData?.subscriptionDetails?.projectName || "-"}</div>
              </div>
              <div className="detail-item">
                <strong>Project Key</strong>
                <div className="data-info">{csReportData?.subscriptionDetails?.projectKey || "-"}</div>
              </div>
              <div className="detail-item">
                <strong>Project Type</strong>
                <div className="data-info">{csReportData?.subscriptionDetails?.projectType || "-"}</div>
              </div>
              <div className="detail-item">
                <strong>Subscription Start Date</strong>
                <div className="data-info">{csReportData?.subscriptionDetails?.startDate || "-"}</div>
              </div>
              <div className="detail-item">
                <strong>Subscription End Date</strong>
                <div className="data-info">{csReportData?.subscriptionDetails?.endDate || "-"}</div>
              </div>
              <div className="detail-item">
                <strong>Support Tier</strong>
                <div className="data-info">{csReportData?.subscriptionDetails?.supportTier || "-"}</div>
              </div>
            </div>

            <div className="price-table">
              <table className="table-container">
                <thead>
                  <tr className="table-header">
                    <th>Product(s)</th>
                    <th>Subscription</th>
                    <th>Query Hours</th>
                  </tr>
                </thead>
                <tbody className="table-data">
                  {(() => {
                    const allProducts = (csReportData?.projectDeployments || []).flatMap((d) => d.products || []);
                    const uniqueProductsMap = allProducts.reduce((acc, product) => {
                      if (!acc.has(product.name)) acc.set(product.name, product);
                      return acc;
                    }, new Map<string, (typeof allProducts)[number]>());
                    const uniqueProducts = Array.from(uniqueProductsMap);
                    const rowSpanCount = uniqueProducts.length;
                    return uniqueProducts.map(([name], index) => (
                      <tr key={name}>
                        <td>{name}</td>
                        {index === 0 && (
                          <>
                            <td rowSpan={rowSpanCount}>${csReportData?.subscriptionDetails?.subscription}</td>
                            <td rowSpan={rowSpanCount}>
                              {csReportData?.subscriptionDetails?.totalQueryHours} <br />(
                              {csReportData?.subscriptionDetails?.consumedQueryHours} Utilized)
                            </td>
                          </>
                        )}
                      </tr>
                    ));
                  })()}
                </tbody>
              </table>
            </div>
          </div>
          <br />
          <div className="current-products-widget">
            {(csReportData?.projectDeployments || []).flatMap((d) => d.products || []).length > 0 && (
              <>
                <h2 className="widget-title">Usage</h2>
                <div className="products-table-container">
                  <table className="table-container">
                    <thead>
                      <tr className="product-table-header">
                        {(csReportData?.projectDeployments || [])
                          .flatMap((d) => d.products || [])
                          .map((product, index) => (
                            <th key={index}>{product.cores || "-"} Cores</th>
                          ))}
                      </tr>
                    </thead>
                    <tbody className="product-table-data">
                      <tr>
                        {(csReportData?.projectDeployments || [])
                          .flatMap((d) => d.products || [])
                          .map((product, index) => (
                            <td key={index}>{product.name}</td>
                          ))}
                      </tr>
                    </tbody>
                  </table>
                </div>
              </>
            )}
          </div>
          <br />
          <div className="product-versions-widget">
            {csReportData?.projectDeployments?.some((d: ProjectDeployment) => d.products && d.products.length > 0) && (
              <>
                <h2 className="widget-title">Product Versions and EOL Status</h2>
                {csReportData?.projectDeployments?.map((deployment, index) => {
                  const { name: deploymentName, products } = deployment;
                  return (
                    products.length > 0 && (
                      <div key={index} className="deployment-section">
                        <h3 className="table-title">{deploymentName} Environment</h3>
                        <table className="table-container">
                          <thead>
                            <tr className="table-header">
                              <th>Product</th>
                              <th>Earliest Possible Support EOL Date</th>
                              <th>Support EOL Date</th>
                              <th>Current Support Status</th>
                              <th>Comments</th>
                            </tr>
                          </thead>
                          <tbody className="table-data">
                            {products.map((product, i) => (
                              <tr key={i}>
                                <td>
                                  {product.name} v{product.version}{" "}
                                </td>
                                <td>{product.earliestPossibleSupportEOLDate || "N/A"}</td>
                                <td>{product.eolDate || "N/A"}</td>
                                <td>{product.supportStatus || "N/A"}</td>
                                <td> </td>
                              </tr>
                            ))}
                          </tbody>
                        </table>
                      </div>
                    )
                  );
                })}
              </>
            )}
          </div>
          <br />
          <div className="cases-charts-widget">
            <h2 className="widget-title">Cases </h2>
            <div className="charts">
              <div className="opened-resolved-bar-chart">
                {transformedData?.casesByDayAndState ? (
                  <BarChart data={transformedData?.casesByDayAndState} chartType="state" from={from ?? dayjs()} to={to ?? dayjs()} />
                ) : (
                  <p>No data available for opened/resolved cases.</p>
                )}
              </div>
              <div className="environment-chart">
                <CSPieChart deploymentNames={transformedData?.deploymentNames || []} data={transformedData?.casesByDeployment || {}} />
              </div>
              <div className="product-bar-chart">
                {transformedData?.casesByMonthAndProduct && transformedData?.distinctProductNames ? (
                  <BarChart
                    data={transformedData?.casesByMonthAndProduct}
                    distinctProductNames={transformedData?.distinctProductNames}
                    from={from ?? dayjs()}
                    to={to ?? dayjs()}
                    chartType="product"
                  />
                ) : (
                  <p>No data available for product cases.</p>
                )}
              </div>
              <div className="priority-bar-chart">
                {transformedData?.casesByMonthAndPriority && transformedData?.distinctPriorities ? (
                  <BarChart
                    data={transformedData?.casesByMonthAndPriority}
                    distinctPriorities={transformedData?.distinctPriorities}
                    from={from ?? dayjs()}
                    to={to ?? dayjs()}
                    chartType="priority"
                  />
                ) : (
                  <p>No data available for priority cases.</p>
                )}
              </div>
            </div>
          </div>
          <br />
          <div className="cases-volume-widget">
            <h2 className="widget-title">Case Volume by Months</h2>
            <div className="chart-container">
              {transformedData?.MonthlyCases && transformedData?.MonthlyCases.length > 0 ? (
                <CSLineChart data={transformedData.MonthlyCases} from={from ?? dayjs().startOf("month")} to={to ?? dayjs().endOf("month")} />
              ) : (
                <p style={{ textAlign: "center", color: "red" }}>No data available for case volume by quarters.</p>
              )}
            </div>
          </div>
          <br />
          <div className="incident-sla-compliance-widget">
            <h2 className="widget-title">Incident SLA Compliance </h2>
            {csReportData?.slaDetails?.slaPerformanceStats ? (
              <div className="sla-stats" style={{ display: "flex", flexDirection: "row", justifyContent: "space-around" }}>
                <div className="sla-metric">
                  <h3>{csReportData.slaDetails.slaPerformanceStats.Resolution.percentage}%</h3>
                  <p>Resolution ({csReportData.slaDetails.slaPerformanceStats.Resolution.fraction})</p>
                </div>
                <div className="sla-metric">
                  <h3>{csReportData.slaDetails.slaPerformanceStats.Workaround.percentage}%</h3>
                  <p>Workaround ({csReportData.slaDetails.slaPerformanceStats.Workaround.fraction})</p>
                </div>
                <div className="sla-metric">
                  <h3>{csReportData.slaDetails.slaPerformanceStats.Response.percentage}%</h3>
                  <p>Response ({csReportData.slaDetails.slaPerformanceStats.Response.fraction})</p>
                </div>
              </div>
            ) : (
              <p>No SLA statistics available</p>
            )}
          </div>
          <br />
          <div className="product-updates-widget">
            {csReportData?.projectDeployments?.some((d) => d.products?.some((p) => p.updateLevelInfo != null)) && (
              <>
                <h2 className="widget-title">Product Update Levels </h2>
                {csReportData?.projectDeployments?.map((deployment, index) => {
                  const productsWithUpdateInfo = deployment.products?.filter((p) => p.updateLevelInfo != null) ?? [];
                  return productsWithUpdateInfo.length > 0 ? (
                    <div key={index} className="environment-table-container">
                      <h3 className="table-title">{deployment.name} Environment</h3>
                      <table className="table-container">
                        <thead className="table-header">
                          <tr>
                            <th>Product</th>
                            <th>Current Update Level</th>
                          </tr>
                        </thead>
                        <tbody className="table-data">
                          {productsWithUpdateInfo.map((product, i) => (
                            <tr key={i}>
                              <td>
                                {product.name} v{product.version}
                              </td>
                              <td>{product.updateLevelInfo ?? "N/A"}</td>
                            </tr>
                          ))}
                        </tbody>
                      </table>
                    </div>
                  ) : null;
                })}
              </>
            )}
          </div>
          <br />
          <div className="engagements-widget">
            <h2 className="widget-title">Engagements</h2>
            <table className="table-container">
              <thead>
                <tr className="table-header">
                  <th>Created</th>
                  <th>Case</th>
                  <th>Description</th>
                  <th>Type</th>
                  <th>Updated</th>
                </tr>
              </thead>
              <tbody className="table-data">
                {csReportData?.casesRecords && csReportData.casesRecords.length > 0 ? (
                  (() => {
                    const engagementCases = csReportData.casesRecords.filter((r) => r.caseType === "Engagement");
                    return engagementCases.length > 0 ? (
                      engagementCases.map((record, index) => (
                        <tr key={index}>
                          <td>{record.opened || "N/A"}</td>
                          <td>{record.caseNumber || "N/A"}</td>
                          <td>{record.description || "N/A"}</td>
                          <td>{record.engagementType || "N/A"}</td>
                          <td>{record.updated || "N/A"}</td>
                        </tr>
                      ))
                    ) : (
                      <tr>
                        <td colSpan={5}>No Engagement Data Available</td>
                      </tr>
                    );
                  })()
                ) : (
                  <tr>
                    <td colSpan={5}>No Engagement Data Available</td>
                  </tr>
                )}
              </tbody>
            </table>
          </div>
          <br />
          <div className="deployment-image-section">
            <h3 className="widget-title">Deployment</h3>
            {!deploymentImage ? (
              <input type="file" className="image-input" accept="image/*" onChange={(e) => handleImageUpload(e, "deployment")} />
            ) : (
              <div className="image-preview">
                <img src={deploymentImage} alt="Deployment Preview" />
              </div>
            )}
          </div>
          <br />
          <div className="business-overview-section">
            <h2 className="widget-title">Business Overview</h2>
            <textarea
              ref={businessOverviewTextareaRef}
              className="paragraph-input"
              placeholder="Enter details"
              rows={6}
              value={businessOverviewText}
              onChange={handleTextareaChange}
            ></textarea>
          </div>
          <br />
          <div className="extra-notes-section">
            <h2 className="widget-title">Notes</h2>
            {!notesImage ? (
              <input type="file" className="image-input" accept="image/*" onChange={(e) => handleImageUpload(e, "notes")} />
            ) : (
              <div className="image-preview">
                <img src={notesImage} alt="Extra Image Preview" />
              </div>
            )}
            <textarea
              ref={notesTextareaRef}
              className="extra-notes-input"
              placeholder="Enter details"
              rows={6}
              value={notesText}
              onChange={handleNotesTextareaChange}
            ></textarea>
          </div>
        </div>
      </div>
      </ThemeProvider>
    </>
  );

  return <Box sx={{ p: 3 }}>{showReport ? mainView : formView}</Box>;
}
