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

import {
  Box,
  Card,
  Chip,
  FormControl,
  InputLabel,
  Link,
  MenuItem,
  Select,
  Skeleton,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  Tooltip,
  Typography,
} from "@wso2/oxygen-ui";
import { useEffect, useMemo, useState, type JSX, type ReactNode } from "react";
import { Link as RouterLink, useLocation } from "react-router";
import Error403Page from "@components/error/Error403Page";
import RefreshButton from "@components/RefreshButton";
import SectionCard from "@features/csm-dashboard/components/SectionCard";
import DashboardBarChart from "@features/csm-dashboard/components/DashboardBarChart";
import DashboardLineChart from "@features/csm-dashboard/components/DashboardLineChart";
import { useProductIssueDashboard } from "@features/csm-dashboard/api/useProductIssueDashboard";
import type { PieSliceResult } from "@features/csm-dashboard/api/useWidgetPieData";
import type {
  ProductIssueBlockedCase,
  ProductIssueDashboard,
  ProductIssueDashboardItem,
  ProductIssueOpenBySla,
} from "@features/csm-dashboard/types/productIssueDashboard";
import {
  DEFAULT_PRODUCT_ISSUE_PERIOD,
  PRODUCT_ISSUE_PERIODS,
  productIssuePeriodRange,
  productIssuePeriodText,
  productIssueTrendPoints,
  type ProductIssuePeriodId,
} from "@features/csm-dashboard/utils/productIssueDashboardPeriod";
import { ProductIssueDetailDialog } from "@features/csm-cases/components/ProductIssueDetailDialog";
import { ProductIssueSlaChip } from "@features/csm-cases/components/ProductIssueSlaStanding";
import type { ProductBoardStatus } from "@features/csm-cases/types/productWorkItems";
import { productIssueStatusColor, productIssueStatusLabel } from "@features/csm-cases/utils/productWorkItems";
import { formatBackendTimestampForDisplay } from "@utils/dateTime";

/** The literal the dashboard route reads as "issues with no product". */
const UNMAPPED = "Unmapped";

/** Ageing bucket colours: young to old. */
const AGEING_COLORS: PieSliceResult["color"][] = ["success", "info", "warning", "warning", "error"];

function formatDay(value?: string | null): string {
  return (
    formatBackendTimestampForDisplay(value, { year: "numeric", month: "short", day: "numeric" }) ?? "—"
  );
}

function productLabel(name: string | null): string {
  return name ?? UNMAPPED;
}

function percentText(value: number | null): string {
  return value === null ? "—" : `${value}%`;
}

function itemStatusProps(boardStatus?: string | null): {
  label: string;
  color: ReturnType<typeof productIssueStatusColor>;
} {
  const item = {
    boardStatus: (boardStatus ?? undefined) as ProductBoardStatus | undefined,
    boardStatusRaw: null,
    github: { owner: "", repository: "", issueNumber: 0, state: "OPEN" as const },
  };
  return { label: productIssueStatusLabel(item), color: productIssueStatusColor(item) };
}

interface KpiTileProps {
  label: string;
  value: ReactNode;
  help: string;
  tone?: "default" | "warning" | "error" | "success";
}

/** One headline number; `help` says exactly what is counted. */
function KpiTile({ label, value, help, tone = "default" }: KpiTileProps): JSX.Element {
  return (
    <Tooltip title={help} placement="top" arrow>
      <Card
        data-testid={`pi-kpi-${label}`}
        sx={{ p: 2, display: "flex", flexDirection: "column", gap: 0.5, minWidth: 0 }}
      >
        <Typography variant="caption" color="text.secondary" noWrap>
          {label}
        </Typography>
        <Typography
          variant="h4"
          sx={{ fontWeight: 700, color: tone === "default" ? "text.primary" : `${tone}.main` }}
        >
          {value}
        </Typography>
      </Card>
    </Tooltip>
  );
}

function EmptyText({ children }: { children: ReactNode }): JSX.Element {
  return (
    <Typography variant="body2" color="text.secondary" sx={{ py: 1 }}>
      {children}
    </Typography>
  );
}

function slaBreakdown(s: ProductIssueOpenBySla): string {
  return `${s.breached} breached, ${s.atRisk} at risk, ${s.ok} ok, ${s.paused} paused, ${s.none} no SLA`;
}

interface IssueListProps {
  items: ProductIssueDashboardItem[];
  emptyMessage: string;
  showWocSince?: boolean;
  onOpenIssue: (id: string) => void;
}

/** A list of issues: the number opens the issue dialog, the case number the case. */
function IssueList({ items, emptyMessage, showWocSince, onOpenIssue }: IssueListProps): JSX.Element {
  const location = useLocation();
  if (items.length === 0) return <EmptyText>{emptyMessage}</EmptyText>;
  return (
    <TableContainer>
      <Table size="small">
        <TableHead>
          <TableRow>
            <TableCell>Issue</TableCell>
            <TableCell>Product</TableCell>
            <TableCell>Status</TableCell>
            <TableCell>SLA</TableCell>
            {showWocSince && <TableCell>Waiting on CS since</TableCell>}
            <TableCell>Case</TableCell>
            <TableCell>Opened</TableCell>
          </TableRow>
        </TableHead>
        <TableBody>
          {items.map((item) => {
            const status = itemStatusProps(item.boardStatus);
            return (
              <TableRow key={item.id} hover>
                <TableCell sx={{ maxWidth: 320 }}>
                  <Link
                    component="button"
                    type="button"
                    variant="body2"
                    underline="hover"
                    onClick={() => onOpenIssue(item.id)}
                    aria-label={`View product issue ${item.number}`}
                    sx={{ fontWeight: 600, textAlign: "left" }}
                  >
                    {item.number}
                  </Link>
                  <Typography variant="body2" noWrap title={item.title}>
                    {item.title}
                  </Typography>
                </TableCell>
                <TableCell>
                  <Typography variant="body2">{productLabel(item.productName)}</Typography>
                  <Typography variant="caption" color="text.secondary">
                    {`${item.owner}/${item.repository}#${item.issueNumber}`}
                  </Typography>
                </TableCell>
                <TableCell>
                  <Chip size="small" variant="outlined" color={status.color} label={status.label} />
                </TableCell>
                <TableCell>
                  <ProductIssueSlaChip sla={item.sla} />
                </TableCell>
                {showWocSince && <TableCell>{formatDay(item.wocSince)}</TableCell>}
                <TableCell>
                  {item.case ? (
                    <Link
                      component={RouterLink}
                      to={`/cases/${item.case.id}`}
                      state={{ from: location.pathname }}
                      variant="body2"
                    >
                      {item.case.number}
                    </Link>
                  ) : (
                    <Typography variant="body2" color="text.secondary">
                      Unmatched
                    </Typography>
                  )}
                </TableCell>
                <TableCell>{formatDay(item.openedOn)}</TableCell>
              </TableRow>
            );
          })}
        </TableBody>
      </Table>
    </TableContainer>
  );
}

function BlockedCasesTable({
  cases,
  onOpenIssue,
}: {
  cases: ProductIssueBlockedCase[];
  onOpenIssue: (id: string) => void;
}): JSX.Element {
  const location = useLocation();
  if (cases.length === 0) return <EmptyText>No cases are blocked by a product team.</EmptyText>;
  return (
    <TableContainer>
      <Table size="small">
        <TableHead>
          <TableRow>
            <TableCell>Case</TableCell>
            <TableCell>Subject</TableCell>
            <TableCell>Product issues</TableCell>
            <TableCell>Blocked since</TableCell>
          </TableRow>
        </TableHead>
        <TableBody>
          {cases.map((c) => (
            <TableRow key={c.caseId} hover>
              <TableCell>
                <Link
                  component={RouterLink}
                  to={`/cases/${c.caseId}`}
                  state={{ from: location.pathname }}
                  variant="body2"
                  sx={{ fontWeight: 600 }}
                >
                  {c.caseNumber}
                </Link>
              </TableCell>
              <TableCell sx={{ maxWidth: 360 }}>
                <Typography variant="body2" noWrap title={c.subject}>
                  {c.subject}
                </Typography>
              </TableCell>
              <TableCell>
                <Box sx={{ display: "flex", flexDirection: "column", gap: 0.25, alignItems: "flex-start" }}>
                  {c.issues.map((issue) => (
                    <Link
                      key={issue.id}
                      component="button"
                      type="button"
                      variant="body2"
                      underline="hover"
                      onClick={() => onOpenIssue(issue.id)}
                      aria-label={`View product issue ${issue.number}`}
                      title={issue.title}
                    >
                      {`${issue.number} (${productLabel(issue.productName)})`}
                    </Link>
                  ))}
                </Box>
              </TableCell>
              <TableCell>{formatDay(c.oldestOpenedOn)}</TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </TableContainer>
  );
}

function ByProductTable({ data }: { data: ProductIssueDashboard }): JSX.Element {
  if (data.byProduct.length === 0) return <EmptyText>No product issues in this period.</EmptyText>;
  return (
    <TableContainer>
      <Table size="small">
        <TableHead>
          <TableRow>
            <TableCell>Product</TableCell>
            <TableCell align="right">Open</TableCell>
            <TableCell align="right">Opened</TableCell>
            <TableCell align="right">Resolved</TableCell>
            <TableCell align="right">SLA compliance</TableCell>
            <TableCell align="right">Breached open</TableCell>
            <TableCell align="right">Waiting on CS</TableCell>
            <TableCell align="right">Waiting on product team</TableCell>
          </TableRow>
        </TableHead>
        <TableBody>
          {data.byProduct.map((row) => (
            <TableRow key={`${row.productName ?? ""}|${row.owner}/${row.repository}`} hover>
              <TableCell>
                <Typography variant="body2" sx={{ fontWeight: 600 }}>
                  {productLabel(row.productName)}
                </Typography>
                <Typography variant="caption" color="text.secondary">
                  {`${row.owner}/${row.repository}`}
                </Typography>
              </TableCell>
              <TableCell align="right">{row.open}</TableCell>
              <TableCell align="right">{row.openedInPeriod}</TableCell>
              <TableCell align="right">{row.resolvedInPeriod}</TableCell>
              <TableCell align="right" title={`${row.slaMet} met, ${row.slaBreached} breached`}>
                {percentText(row.compliancePercentage)}
              </TableCell>
              <TableCell align="right" sx={{ color: row.openBySla.breached > 0 ? "error.main" : undefined }}>
                {row.openBySla.breached}
              </TableCell>
              <TableCell align="right">{row.waitingOnCs}</TableCell>
              <TableCell align="right">{row.waitingOnProductTeam}</TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </TableContainer>
  );
}

function LoadingSkeleton(): JSX.Element {
  return (
    <Box data-testid="pi-loading" sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
      <Skeleton variant="rounded" height={88} />
      <Skeleton variant="rounded" height={248} />
      <Skeleton variant="rounded" height={180} />
    </Box>
  );
}

function DashboardBody({
  data,
  onOpenIssue,
}: {
  data: ProductIssueDashboard;
  onOpenIssue: (id: string) => void;
}): JSX.Element {
  const t = data.totals;
  const trendPoints = useMemo(() => productIssueTrendPoints(data.trend), [data.trend]);
  const ageingSlices: PieSliceResult[] = data.wocAgeing.buckets.map((b, i) => ({
    label: b.label,
    value: b.count,
    color: AGEING_COLORS[i % AGEING_COLORS.length],
    query: null,
    navigable: false,
  }));
  const ageingTotal = data.wocAgeing.buckets.reduce((sum, b) => sum + b.count, 0);

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 2.5 }}>
      <Box
        sx={{
          display: "grid",
          gap: 1.5,
          gridTemplateColumns: "repeat(auto-fill, minmax(170px, 1fr))",
        }}
      >
        <KpiTile
          label="Open"
          value={t.open}
          help="Issues open now: not resolved and not closed. As of the time shown above, not historical."
        />
        <KpiTile
          label="Opened in period"
          value={t.openedInPeriod}
          help="Issues opened on GitHub (or in the portal when GitHub has no date) during the selected period."
        />
        <KpiTile
          label="Resolved in period"
          value={t.resolvedInPeriod}
          tone="success"
          help="Issues resolved or closed during the selected period (the earlier of the GitHub close and the board's Resolved move)."
        />
        <KpiTile
          label="Waiting on CS"
          value={t.waitingOnCs}
          tone={t.waitingOnCs > 0 ? "warning" : "default"}
          help="Open issues where the product team is waiting for a reply from CS (their SLA clock is paused)."
        />
        <KpiTile
          label="Waiting on product team"
          value={t.waitingOnProductTeam}
          help="Open issues where the next move belongs to the product team."
        />
        <KpiTile
          label="SLA compliance"
          value={percentText(t.compliancePercentage)}
          tone={t.compliancePercentage !== null && t.compliancePercentage < 80 ? "error" : "default"}
          help={`Issues whose SLA clock finished in the period: ${t.slaMet} met, ${t.slaBreached} breached. Met divided by met plus breached; empty when none finished.`}
        />
        <KpiTile
          label="Breached open"
          value={t.openBySla.breached}
          tone={t.openBySla.breached > 0 ? "error" : "default"}
          help={`Open issues by SLA now: ${slaBreakdown(t.openBySla)}.`}
        />
        <KpiTile
          label="Unmatched open"
          value={t.unmatchedOpen}
          help="Open issues with no active link to a case, so no case to blame the wait on."
        />
        <KpiTile
          label="Non-standard column"
          value={t.nonStandardColumnOpen}
          help="Open issues sitting in a project board column outside the standard set; they are counted as open but not as waiting on anyone."
        />
      </Box>

      <SectionCard
        title="Opened and resolved"
        subtitle={`By ${data.bucket}, ${productIssuePeriodText(data.periodStart, data.periodEnd)}`}
      >
        <DashboardLineChart
          points={trendPoints}
          isLoading={false}
          isError={false}
          chart="bar"
          bucket={data.bucket}
          seriesLabels={{ opened: "Opened", closed: "Resolved" }}
        />
      </SectionCard>

      <SectionCard title="By product" subtitle="A product is found from the issue's repository; Unmapped means no product maps to it.">
        <ByProductTable data={data} />
      </SectionCard>

      <SectionCard
        title="Waiting on CS: age"
        subtitle="Calendar time since each open issue started waiting on CS, as of now."
      >
        <DashboardBarChart
          slices={ageingSlices}
          total={ageingTotal}
          isLoading={false}
          isError={false}
          onSliceClick={() => {}}
        />
        <Typography variant="subtitle2">Waiting longest</Typography>
        <IssueList
          items={data.wocAgeing.oldest}
          emptyMessage="No issues are waiting on CS."
          showWocSince
          onOpenIssue={onOpenIssue}
        />
      </SectionCard>

      <SectionCard
        title="Cases blocked by product teams"
        subtitle={`${data.blockedCases.total} open ${
          data.blockedCases.total === 1 ? "case has" : "cases have"
        } an open product issue waiting on the product team.`}
      >
        {data.blockedCases.byProduct.length > 0 && (
          <Box sx={{ display: "flex", flexWrap: "wrap", gap: 1 }}>
            {data.blockedCases.byProduct.map((p) => (
              <Chip
                key={`${p.productName ?? ""}|${p.owner}/${p.repository}`}
                size="small"
                variant="outlined"
                label={`${productLabel(p.productName)}: ${p.cases}`}
                title={`${p.owner}/${p.repository}`}
              />
            ))}
          </Box>
        )}
        <BlockedCasesTable cases={data.blockedCases.cases} onOpenIssue={onOpenIssue} />
      </SectionCard>

      <SectionCard title="SLA breached, still open" subtitle="Highest SLA consumption first.">
        <IssueList
          items={data.breachedOpen}
          emptyMessage="No open issue has breached its SLA."
          onOpenIssue={onOpenIssue}
        />
      </SectionCard>
    </Box>
  );
}

/**
 * `/dashboard/product-issues`: the product issues lead dashboard (cs_lead or
 * admin; the nav node's `requires` hides the route for everyone else, and the
 * backend answers 403 anyway). One request, `GET /product-work-items/dashboard`;
 * the period and product filters are its query. 404 (git internals off) shows
 * "Not available", 403 the standard 403 page.
 */
export default function ProductIssueDashboardPage(): JSX.Element {
  const [periodId, setPeriodId] = useState<ProductIssuePeriodId>(DEFAULT_PRODUCT_ISSUE_PERIOD);
  const [products, setProducts] = useState<string[]>([]);
  const [openIssueId, setOpenIssueId] = useState<string | null>(null);

  // The window ends tomorrow in Asia/Kolkata, so it depends on the clock: a
  // page left open past midnight must not keep asking for yesterday's `to`.
  // `now` is state (a render-time `new Date()` would be memoised away by the
  // React Compiler), refreshed every minute and on Refresh.
  const [now, setNow] = useState(() => new Date());
  useEffect(() => {
    const id = window.setInterval(() => setNow(new Date()), 60_000);
    return () => window.clearInterval(id);
  }, []);
  const range = useMemo(() => productIssuePeriodRange(periodId, now), [periodId, now]);
  const main = useProductIssueDashboard({ ...range, products });
  // The same query as `main` while no product is chosen (react-query shares
  // it); with a choice, the unfiltered load still supplies the options.
  const unfiltered = useProductIssueDashboard({ ...range, products: [] });

  const options = useMemo(() => {
    const names = new Set<string>();
    for (const row of unfiltered.data?.byProduct ?? []) names.add(productLabel(row.productName));
    for (const p of products) names.add(p);
    return [...names].sort((a, b) => {
      if (a === UNMAPPED) return 1;
      if (b === UNMAPPED) return -1;
      return a.localeCompare(b);
    });
  }, [unfiltered.data, products]);

  const status = (main.error as { status?: number } | null)?.status;
  if (status === 403) return <Error403Page />;

  let body: JSX.Element;
  if (main.isLoading) {
    body = <LoadingSkeleton />;
  } else if (main.isError) {
    body = (
      <Typography variant="body2" color="error">
        Could not load the product issues dashboard.
      </Typography>
    );
  } else if (main.data == null) {
    body = (
      <Typography variant="body1" color="text.secondary" data-testid="pi-not-available">
        Not available. Product issues are not enabled in this environment.
      </Typography>
    );
  } else {
    body = <DashboardBody data={main.data} onOpenIssue={setOpenIssueId} />;
  }

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 2.5 }}>
      <Box sx={{ display: "flex", flexWrap: "wrap", alignItems: "center", justifyContent: "space-between", gap: 2 }}>
        <Box>
          <Typography variant="h5">Product issues</Typography>
          <Typography variant="body2" color="text.secondary">
            {main.data
              ? `Open, waiting and SLA figures are as of ${
                  formatBackendTimestampForDisplay(main.data.generatedOn, {
                    year: "numeric",
                    month: "short",
                    day: "numeric",
                    hour: "2-digit",
                    minute: "2-digit",
                  }) ?? "now"
                }; opened, resolved and compliance cover the period.`
              : "Internal GitHub issues raised from support cases."}
          </Typography>
        </Box>
        <Box sx={{ display: "flex", flexWrap: "wrap", alignItems: "center", gap: 1.5 }}>
          <FormControl size="small" sx={{ minWidth: 170 }}>
            <InputLabel id="pi-period-label">Period</InputLabel>
            <Select
              labelId="pi-period-label"
              label="Period"
              value={periodId}
              onChange={(e) => setPeriodId(e.target.value as ProductIssuePeriodId)}
              inputProps={{ "data-testid": "pi-period" }}
            >
              {PRODUCT_ISSUE_PERIODS.map((p) => (
                <MenuItem key={p.id} value={p.id}>
                  {p.label}
                </MenuItem>
              ))}
            </Select>
          </FormControl>
          <FormControl size="small" sx={{ minWidth: 220 }}>
            <InputLabel id="pi-products-label">Products</InputLabel>
            <Select
              multiple
              labelId="pi-products-label"
              label="Products"
              value={products}
              onChange={(e) => {
                const v = e.target.value;
                setProducts(typeof v === "string" ? v.split(",") : (v as string[]));
              }}
              renderValue={(selected) => (selected.length === 0 ? "All products" : selected.join(", "))}
              displayEmpty
              inputProps={{ "data-testid": "pi-products" }}
            >
              {options.map((name) => (
                <MenuItem key={name} value={name}>
                  {name}
                </MenuItem>
              ))}
            </Select>
          </FormControl>
          <RefreshButton
            onRefresh={() => {
              // A new window is a new query key, which fetches by itself; the
              // same window is refetched.
              const current = new Date();
              const next = productIssuePeriodRange(periodId, current);
              setNow(current);
              if (next.to === range.to && next.from === range.from) void main.refetch();
            }}
            isFetching={main.isFetching}
            updatedAt={main.dataUpdatedAt}
            label="Refresh product issues"
          />
        </Box>
      </Box>

      {body}

      {openIssueId && (
        <ProductIssueDetailDialog workItemId={openIssueId} onClose={() => setOpenIssueId(null)} />
      )}
    </Box>
  );
}
