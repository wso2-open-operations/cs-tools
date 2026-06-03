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

import { Box, Button, Chip, Typography } from "@wso2/oxygen-ui";
import { Plus } from "@wso2/oxygen-ui-icons-react";
import { useCallback, useEffect, useMemo, useRef, useState, type JSX } from "react";
import { useNavigate, useSearchParams } from "react-router";
import { useErrorBanner } from "@context/error-banner/ErrorBannerContext";
import CasesFilterBar, {
  type CasesFilters,
} from "@features/csm-cases/components/CasesFilterBar";
import CasesList from "@features/csm-cases/components/CasesList";
import { useGetCsmCases } from "@features/csm-cases/api/useGetCsmCases";
import type { CsmCaseRow } from "@features/csm-cases/types/csmCases";
import {
  DEFAULT_CASES_FILTERS,
  readCasesFiltersFromUrl,
  writeCasesFiltersToUrl,
} from "@features/csm-cases/utils/casesFiltersUrl";

const SLA_AT_RISK_THRESHOLD_MINUTES = 60;

function applyFilters(cases: CsmCaseRow[], f: CasesFilters): CsmCaseRow[] {
  const q = f.search.trim().toLowerCase();
  return cases.filter((c) => {
    if (f.severities.length && !f.severities.includes(c.severity)) return false;
    if (f.states.length && !f.states.includes(c.state)) return false;
    if (f.sla === "breached" && c.minutesToBreach >= 0) return false;
    if (
      f.sla === "at_risk" &&
      !(c.minutesToBreach >= 0 && c.minutesToBreach <= SLA_AT_RISK_THRESHOLD_MINUTES)
    )
      return false;
    if (f.owner === "me" && !c.ownerIsMe) return false;
    if (f.owner === "unassigned" && c.owner !== "Unassigned") return false;
    if (f.customers.length && !f.customers.includes(c.customer)) return false;
    if (f.projects.length && !f.projects.includes(c.projectName)) return false;
    if (q) {
      const hay = `${c.caseNumber} ${c.subject} ${c.customer} ${c.projectName} ${c.owner}`.toLowerCase();
      if (!hay.includes(q)) return false;
    }
    return true;
  });
}

function sortBySlaUrgency(a: CsmCaseRow, b: CsmCaseRow): number {
  // Closed last
  const aClosed = a.state === "closed" ? 1 : 0;
  const bClosed = b.state === "closed" ? 1 : 0;
  if (aClosed !== bClosed) return aClosed - bClosed;
  return a.minutesToBreach - b.minutesToBreach;
}

export default function CsmCasesPage(): JSX.Element {
  const navigate = useNavigate();
  const [searchParams, setSearchParams] = useSearchParams();
  const filters = useMemo<CasesFilters>(
    () => readCasesFiltersFromUrl(searchParams),
    [searchParams],
  );

  const setFilters = useCallback(
    (next: CasesFilters) => {
      setSearchParams(writeCasesFiltersToUrl(next), { replace: true });
    },
    [setSearchParams],
  );

  const { data, isLoading, isError } = useGetCsmCases(filters.scope);
  const { showError } = useErrorBanner();
  const hasShownErrorRef = useRef(false);
  const [isFiltersOpen, setIsFiltersOpen] = useState(false);

  useEffect(() => {
    if (isError && !hasShownErrorRef.current) {
      hasShownErrorRef.current = true;
      showError("Could not load cases.");
    }
    if (!isError) hasShownErrorRef.current = false;
  }, [isError, showError]);

  const filtered = useMemo(() => {
    const list = data?.cases ?? [];
    return applyFilters(list, filters).slice().sort(sortBySlaUrgency);
  }, [data?.cases, filters]);

  // Derive multi-select option lists from the cases currently in scope.
  // Sorted and de-duplicated; cheap because the lists are small (~tens).
  const availableCustomers = useMemo(() => {
    const set = new Set<string>();
    (data?.cases ?? []).forEach((c) => {
      if (c.customer) set.add(c.customer);
    });
    return Array.from(set).sort();
  }, [data?.cases]);

  const availableProjects = useMemo(() => {
    const set = new Set<string>();
    (data?.cases ?? []).forEach((c) => {
      if (c.projectName) set.add(c.projectName);
    });
    return Array.from(set).sort();
  }, [data?.cases]);

  const totalAvailable = data?.cases.length ?? 0;
  const breachedCount = filtered.filter((c) => c.minutesToBreach < 0 && c.state !== "closed").length;
  const myCount = filtered.filter((c) => c.ownerIsMe && c.state !== "closed").length;

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
      <Box
        sx={{
          display: "flex",
          alignItems: "center",
          justifyContent: "space-between",
          gap: 2,
          flexWrap: "wrap",
        }}
      >
        <Box>
          <Typography variant="h5">Cases</Typography>
          <Typography variant="body2" color="text.secondary">
            {isLoading
              ? "Loading…"
              : `${filtered.length} of ${totalAvailable} shown`}
          </Typography>
        </Box>
        <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
          {breachedCount > 0 && (
            <Chip
              size="small"
              color="error"
              label={`${breachedCount} breached`}
            />
          )}
          {myCount > 0 && (
            <Chip
              size="small"
              color="primary"
              variant="outlined"
              label={`${myCount} mine`}
            />
          )}
          <Button
            variant="contained"
            color="primary"
            size="small"
            startIcon={<Plus size={16} />}
            onClick={() => navigate("/cases/new")}
          >
            Create case
          </Button>
        </Box>
      </Box>

      <CasesFilterBar
        filters={filters}
        onChange={setFilters}
        onReset={() =>
          setFilters({ ...DEFAULT_CASES_FILTERS, scope: filters.scope })
        }
        isFiltersOpen={isFiltersOpen}
        onFiltersToggle={() => setIsFiltersOpen((v) => !v)}
        availableCustomers={availableCustomers}
        availableProjects={availableProjects}
      />

      <CasesList cases={filtered} isLoading={isLoading} />
    </Box>
  );
}
