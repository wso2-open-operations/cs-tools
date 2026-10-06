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

// Ported from apps/support-portal-lite/webapp's CustomerHealthDashboardPage.tsx.
// SplShell (that app's own auth/theme wrapper) is dropped — RouteGuard +
// this app's own AppLayout/AuthGuard already provide the equivalent
// chrome/auth boundary. No local PageHeader component exists in this app
// for a single two-line title, so it's inlined directly below.
import { useState, useEffect } from "react";
import { Box, Alert, Typography } from "@wso2/oxygen-ui";
import { useBackendApi } from "@api/backend/client";
import { fetchCustomerHealthSummaryPage, type CustomerHealthSummaryPayload } from "../api/useCustomerHealthSummary";
import SearchCustomerHealth from "../components/SearchCustomerHealth";
import CustomerHealthFilters, { type HealthFilters } from "../components/CustomerHealthFilters";
import CustomerHealthTable from "../components/CustomerHealthTable";
import type { AccountSummary } from "../api/customerHealthTypes";

const DEFAULT_FILTERS: HealthFilters = {
  accountScope: "all-accounts",
  userEmail: null,
  riskIndicators: [],
  region: [],
  product: null,
  abtTeam: null,
  healthStatus: null,
};

function buildExportPayload(
  filters: HealthFilters,
  searchQuery: string,
  offset: number,
  limit: number,
): CustomerHealthSummaryPayload {
  return {
    offset,
    limit,
    email: filters.accountScope === "my-accounts" && filters.userEmail ? filters.userEmail : "",
    phrase: searchQuery && searchQuery.length >= 2 ? searchQuery : "",
    risks: Array.isArray(filters.riskIndicators) && filters.riskIndicators.length > 0 ? filters.riskIndicators.join(",") : "",
    region: Array.isArray(filters.region) && filters.region.length > 0 ? filters.region : [],
    product: filters.product || "",
    abtTeam: filters.abtTeam || "",
    healthStatus: filters.healthStatus || "",
  };
}

export default function CustomerHealthDashboardPage() {
  const backendApi = useBackendApi();

  const [exporting, setExporting] = useState(false);
  const [exportError, setExportError] = useState<string | null>(null);

  const [filters, setFilters] = useState<HealthFilters>(() => {
    const saved = sessionStorage.getItem("dashboard_filters");
    if (!saved) return DEFAULT_FILTERS;
    try {
      return { ...DEFAULT_FILTERS, ...JSON.parse(saved) };
    } catch {
      return DEFAULT_FILTERS;
    }
  });

  useEffect(() => {
    sessionStorage.setItem("dashboard_filters", JSON.stringify(filters));
  }, [filters]);

  const [page, setPage] = useState<number>(() => {
    const saved = sessionStorage.getItem("dashboard_page");
    if (!saved) return 0;
    const parsed = parseInt(saved, 10);
    return Number.isFinite(parsed) && parsed >= 0 ? parsed : 0;
  });

  const [rowsPerPage, setRowsPerPage] = useState<number>(() => {
    const saved = sessionStorage.getItem("dashboard_rows_per_page");
    if (!saved) return 10;
    const parsed = parseInt(saved, 10);
    return Number.isFinite(parsed) && parsed > 0 ? parsed : 10;
  });

  useEffect(() => {
    sessionStorage.setItem("dashboard_page", String(page));
  }, [page]);

  useEffect(() => {
    sessionStorage.setItem("dashboard_rows_per_page", String(rowsPerPage));
  }, [rowsPerPage]);

  const [searchQuery, setSearchQuery] = useState<string>(() => sessionStorage.getItem("dashboard_search") || "");

  useEffect(() => {
    sessionStorage.setItem("dashboard_search", searchQuery);
  }, [searchQuery]);

  const handleFilterChange = (newFilters: HealthFilters) => {
    setFilters(newFilters);
    setPage(0);
  };

  const handleSearch = (query: string) => {
    if (query !== searchQuery) setPage(0);
    setSearchQuery(query);
  };

  const handleExport = async () => {
    setExporting(true);
    setExportError(null);
    try {
      const BATCH = 20;
      const first = await fetchCustomerHealthSummaryPage(backendApi, buildExportPayload(filters, searchQuery, 0, BATCH));
      const totalCount = first.totalCount;
      let allAccounts: AccountSummary[] = [...first.data];

      let offset = BATCH;
      while (offset < totalCount) {
        const result = await fetchCustomerHealthSummaryPage(backendApi, buildExportPayload(filters, searchQuery, offset, BATCH));
        allAccounts = [...allAccounts, ...result.data];
        offset += BATCH;
      }

      const statusLabel = (s?: string) => (!s || s === "to_be_reviewed" ? "To Be Reviewed" : s === "at_risk" ? "At Risk" : "Healthy");

      const CSV_HEADERS = [
        "Account Name", "Health Status",
        "Has Gone Live", "Support Activity (Last 6 Mo)", "Using EOL Products",
        "Abandoned Migrations", "Migration Delays", "Escalations (Last 3 Mo)",
      ];

      const yesNo = (v: boolean) => (v ? "Yes" : "No");
      const rows = allAccounts.map((acc) => [
        acc.accountName || acc.accountSysId,
        statusLabel(acc.healthStatus),
        !acc.hasNoGoLive.isRisk && acc.hasNoGoLive.state === "pending" ? "Pending (Not Live Yet)" : yesNo(!acc.hasNoGoLive.isRisk),
        yesNo(!acc.noSupportCases6mo),
        yesNo(acc.hasEolProduct),
        yesNo(acc.hasAbandonedMigrations),
        yesNo(acc.hasMigrationDelays),
        yesNo(acc.hasRecentEscalations),
      ]);

      const sanitizeCSV = (v: string) => (/^[=+\-@\t\r]/.test(v) ? `'${v}` : v);
      const csv = [CSV_HEADERS, ...rows]
        .map((row) => row.map((v) => `"${sanitizeCSV(String(v)).replace(/"/g, '""')}"`).join(","))
        .join("\n");

      const blob = new Blob(["﻿" + csv], { type: "text/csv;charset=utf-8;" });
      const url = URL.createObjectURL(blob);
      const a = document.createElement("a");
      a.href = url;
      a.download = `customer-health-${new Date().toISOString().split("T")[0]}.csv`;
      a.click();
      URL.revokeObjectURL(url);
    } catch (err) {
      console.error("Export failed:", err);
      setExportError("Export failed. Please try again.");
    } finally {
      setExporting(false);
    }
  };

  return (
    <Box>
      <Box sx={{ mb: 3 }}>
        <Typography variant="h5">Customer Health Dashboard</Typography>
        <Typography variant="body2" color="text.secondary">Monitor account parameters and identify risks</Typography>
      </Box>
      <SearchCustomerHealth onSearch={handleSearch} initialValue={searchQuery} />
      <CustomerHealthFilters onFilterChange={handleFilterChange} currentFilters={filters} />
      {exportError && (
        <Alert severity="error" sx={{ mb: 1 }} onClose={() => setExportError(null)}>{exportError}</Alert>
      )}
      <CustomerHealthTable
        accountScope={filters.accountScope}
        userEmail={filters.userEmail}
        searchQuery={searchQuery}
        riskIndicators={filters.riskIndicators}
        region={filters.region}
        product={filters.product}
        abtTeam={filters.abtTeam}
        healthStatus={filters.healthStatus}
        page={page}
        rowsPerPage={rowsPerPage}
        onPageChange={setPage}
        onRowsPerPageChange={(newRows) => { setRowsPerPage(newRows); setPage(0); }}
        onExport={handleExport}
        exporting={exporting}
      />
    </Box>
  );
}
