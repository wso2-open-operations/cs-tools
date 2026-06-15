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
  ASSIGNEE_ME_TOKEN,
  type CasesFilters,
} from "@features/csm-cases/components/CasesFilterBar";
import CasesList from "@features/csm-cases/components/CasesList";
import { useGetCsmCases } from "@features/csm-cases/api/useGetCsmCases";
import { useProjectOptions } from "@features/csm-cases/api/useProjectOptions";
import { useDirectoryUsers } from "@api/useDirectoryUsers";
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
    if (f.assignees.length) {
      const match = f.assignees.some((a) =>
        a === ASSIGNEE_ME_TOKEN ? c.assigneeIsMe : a === c.assignee,
      );
      if (!match) return false;
    }
    if (f.projects.length && !f.projects.includes(c.projectId)) return false;
    if (f.products.length && !f.products.includes(c.product)) return false;
    if (q) {
      const hay = `${c.caseNumber} ${c.subject} ${c.customer} ${c.projectName} ${c.assignee} ${c.product}`.toLowerCase();
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

  const { data, isLoading, isError, error } = useGetCsmCases(filters);
  const { data: projectDirectory } = useProjectOptions();
  const { data: directoryUsers } = useDirectoryUsers();
  const { showError } = useErrorBanner();
  const hasShownErrorRef = useRef(false);
  const [isFiltersOpen, setIsFiltersOpen] = useState(false);

  useEffect(() => {
    if (isError && !hasShownErrorRef.current) {
      hasShownErrorRef.current = true;
      showError("Could not load cases.", error);
    }
    if (!isError) hasShownErrorRef.current = false;
  }, [isError, error, showError]);

  const filtered = useMemo(() => {
    const list = data?.cases ?? [];
    return applyFilters(list, filters).slice().sort(sortBySlaUrgency);
  }, [data?.cases, filters]);

  // Derive option lists for the searchable filters. Assignees come from the
  // user directory (so typing finds anyone, not only owners present in the
  // loaded cases). Projects and products are still derived from the case
  // rows in scope. All sorted and de-duplicated; the lists are small.
  const availableAssigneeUsers = useMemo(() => {
    const list = (directoryUsers ?? [])
      .filter((u) => u.name)
      .map((u) => ({ name: u.name, email: u.email }));
    list.sort((a, b) => a.name.localeCompare(b.name));
    return list;
  }, [directoryUsers]);

  // Project filter is id-based (sends projectIds server-side), so options carry
  // id + name. The list is sourced from the full project directory rather than
  // the loaded cases: once a project is selected the result set is
  // server-filtered to it, so deriving options from `data.cases` would collapse
  // the selector to the chosen project and block adding a second one. Loaded
  // cases backfill the list (covers MOCK mode, where the directory is empty),
  // and selected ids are always kept visible.
  const availableProjects = useMemo(() => {
    const byId = new Map<string, string>();
    (projectDirectory ?? []).forEach((p) => byId.set(p.id, p.name || p.id));
    (data?.cases ?? []).forEach((c) => {
      if (c.projectId && !byId.has(c.projectId)) {
        byId.set(c.projectId, c.projectName || c.projectId);
      }
    });
    filters.projects.forEach((id) => {
      if (!byId.has(id)) byId.set(id, id);
    });
    return Array.from(byId, ([id, name]) => ({ id, name })).sort((a, b) =>
      a.name.localeCompare(b.name),
    );
  }, [projectDirectory, data?.cases, filters.projects]);

  const availableProducts = useMemo(() => {
    const set = new Set<string>();
    (data?.cases ?? []).forEach((c) => {
      if (c.product) set.add(c.product);
    });
    return Array.from(set).sort();
  }, [data?.cases]);

  const totalAvailable = data?.cases.length ?? 0;
  const breachedCount = filtered.filter((c) => c.minutesToBreach < 0 && c.state !== "closed").length;
  const myCount = filtered.filter((c) => c.assigneeIsMe && c.state !== "closed").length;

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
        availableAssigneeUsers={availableAssigneeUsers}
        availableProjects={availableProjects}
        availableProducts={availableProducts}
      />

      <CasesList cases={filtered} isLoading={isLoading} />
    </Box>
  );
}
