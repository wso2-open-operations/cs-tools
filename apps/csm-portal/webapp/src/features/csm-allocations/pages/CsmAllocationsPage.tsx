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
  Button,
  Skeleton,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TablePagination,
  TableRow,
  TableSortLabel,
  TextField,
  Typography,
} from "@wso2/oxygen-ui";
import {
  useCallback,
  useEffect,
  useMemo,
  useState,
  type ChangeEvent,
  type JSX,
  type KeyboardEvent,
} from "react";
import { useLocation, useSearchParams } from "react-router";
import { useSearchInternalUsersByName } from "@api/useSearchUsersByName";
import type { BeUser } from "@api/backend/types";
import AsyncEntitySelect from "@components/AsyncEntitySelect";
import MultiSelectField from "@components/MultiSelectField";
import QueryErrorState from "@components/QueryErrorState";
import RefreshButton from "@components/RefreshButton";
import RelativeTime from "@components/RelativeTime";
import { useDebouncedValue } from "@hooks/useDebouncedValue";
import { useNavTransition } from "@hooks/useNavTransition";
import type { Account } from "@features/csm-accounts/types/csmAccounts";
import { useAccountOptions } from "../api/useAccountOptions";
import { useSearchAllocations } from "../api/useAllocations";
import AllocationStateChip from "../components/AllocationStateChip";
import type { Allocation, AllocationSortField } from "../types";
import { ALLOCATION_STATE_OPTIONS, formatDateSpan, formatUtilization } from "../utils/allocations";
import {
  DEFAULT_SORT,
  ROWS_PER_PAGE_OPTIONS,
  buildSearchPayload,
  isBackwardsRange,
  readListParams,
  writeListParams,
  type ListParams,
} from "../utils/listParams";

interface Column {
  label: string;
  /** The API sort field this column orders by; absent when it cannot be sorted. */
  sortField?: AllocationSortField;
}

const COLUMNS: Column[] = [
  { label: "Allocation", sortField: "allocationId" },
  { label: "Customer", sortField: "accountName" },
  { label: "Engagement" },
  { label: "Engineer", sortField: "engineerName" },
  { label: "State", sortField: "state" },
  { label: "Start - End", sortField: "startDate" },
  { label: "Utilization", sortField: "utilization" },
  { label: "Latest update" },
];

const accountId = (a: Account): string => a.id;
const accountLabel = (a: Account): string => a.name;
const userId = (u: BeUser): string => u.id ?? "";
const userLabel = (u: BeUser): string =>
  [u.firstName, u.lastName].filter(Boolean).join(" ").trim() || u.email || u.userName || userId(u);

/** Cross-customer list of engineer allocations to customer engagements. */
export default function CsmAllocationsPage(): JSX.Element {
  const navigate = useNavTransition();
  const location = useLocation();
  const [searchParams, setSearchParams] = useSearchParams();
  const params = useMemo(() => readListParams(searchParams), [searchParams]);

  const update = useCallback(
    (patch: Partial<ListParams>, resetPage = true): void => {
      setSearchParams(
        (prev) =>
          writeListParams({
            ...readListParams(prev),
            ...patch,
            ...(resetPage && !("page" in patch) ? { page: 0 } : {}),
          }),
        { replace: true },
      );
    },
    [setSearchParams],
  );

  // The text box edits at typing speed; the URL (and so the request) follows
  // once typing pauses.
  const [searchInput, setSearchInput] = useState(params.q);
  const debouncedSearch = useDebouncedValue(searchInput, 300);
  useEffect(() => {
    if (debouncedSearch.trim() !== params.q) update({ q: debouncedSearch.trim() });
    // Only the debounced value drives this; `params.q` changing must not echo back.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [debouncedSearch]);

  const payload = useMemo(() => buildSearchPayload(params), [params]);
  const { data, isLoading, isFetching, isError, error, refetch, dataUpdatedAt } =
    useSearchAllocations(payload);

  const allocations = data?.allocations ?? [];
  const total = data?.total ?? 0;
  const backwards = isBackwardsRange(params.from, params.to);
  const hasFilters = Boolean(
    params.q || params.accountId || params.engineerId || params.states.length || params.from || params.to,
  );

  const handleSort = (field: AllocationSortField): void => {
    const active = params.sort === field;
    update({ sort: field, order: active ? (params.order === "asc" ? "desc" : "asc") : "asc" });
  };

  const clearFilters = (): void => {
    setSearchInput("");
    update({
      q: "",
      accountId: "",
      accountLabel: "",
      engineerId: "",
      engineerLabel: "",
      states: [],
      from: "",
      to: "",
    });
  };

  const open = (a: Allocation): void =>
    navigate(`/allocations/${a.id}`, { state: { from: `${location.pathname}${location.search}` } });

  const onRowKeyDown = (e: KeyboardEvent<HTMLTableRowElement>, a: Allocation): void => {
    if (e.key === "Enter" || e.key === " ") {
      e.preventDefault();
      open(a);
    }
  };

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
      <Box sx={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 1 }}>
        <Typography variant="body2" color="text.secondary">
          Engineer allocations to customer engagements.
        </Typography>
        <RefreshButton
          onRefresh={() => void refetch()}
          isFetching={isFetching}
          updatedAt={dataUpdatedAt}
          label="Refresh allocations"
        />
      </Box>

      <Box
        role="search"
        aria-label="Filter allocations"
        sx={{
          display: "grid",
          gap: 2,
          gridTemplateColumns: { xs: "1fr", sm: "repeat(2, 1fr)", lg: "repeat(3, 1fr)" },
          alignItems: "start",
        }}
      >
        <TextField
          size="small"
          label="Search"
          placeholder="Allocation, engagement name or ID"
          value={searchInput}
          onChange={(e: ChangeEvent<HTMLInputElement>) => setSearchInput(e.target.value)}
          slotProps={{ htmlInput: { "aria-label": "Search allocations by reference, engagement name or engagement ID" } }}
        />
        <AsyncEntitySelect<Account>
          id="allocations-filter-customer"
          label="Customer"
          value={params.accountId}
          knownLabel={params.accountLabel || undefined}
          onChange={(next, item) =>
            update({ accountId: next, accountLabel: next ? (item?.name ?? params.accountLabel) : "" })
          }
          useSearch={useAccountOptions}
          getId={accountId}
          getLabel={accountLabel}
        />
        <AsyncEntitySelect<BeUser>
          id="allocations-filter-engineer"
          label="Engineer"
          value={params.engineerId}
          knownLabel={params.engineerLabel || undefined}
          onChange={(next, item) =>
            update({ engineerId: next, engineerLabel: next ? (item ? userLabel(item) : params.engineerLabel) : "" })
          }
          useSearch={useSearchInternalUsersByName}
          getId={userId}
          getLabel={userLabel}
        />
        <MultiSelectField
          id="allocations-filter-state"
          label="State"
          values={params.states}
          options={ALLOCATION_STATE_OPTIONS.map((o) => ({ value: o.value as string, label: o.label }))}
          onChange={(states) => update({ states })}
        />
        <TextField
          size="small"
          type="date"
          label="From"
          value={params.from}
          onChange={(e) => update({ from: e.target.value })}
          slotProps={{ inputLabel: { shrink: true } }}
          error={backwards}
        />
        <TextField
          size="small"
          type="date"
          label="To"
          value={params.to}
          onChange={(e) => update({ to: e.target.value })}
          slotProps={{ inputLabel: { shrink: true } }}
          error={backwards}
          helperText={backwards ? "To must not be before From. The dates are not applied." : undefined}
        />
      </Box>
      {hasFilters && (
        <Box>
          <Button size="small" variant="text" onClick={clearFilters}>
            Clear filters
          </Button>
        </Box>
      )}

      <Box sx={{ border: 1, borderColor: "divider", borderRadius: 1, overflow: "hidden" }}>
        <TableContainer>
          <Table size="small" aria-label="Allocations" sx={{ "& .MuiTableCell-root": { borderColor: "divider" } }}>
            <TableHead>
              <TableRow sx={{ bgcolor: "action.hover" }}>
                {COLUMNS.map((c) => (
                  <TableCell
                    key={c.label}
                    sortDirection={c.sortField && params.sort === c.sortField ? params.order : false}
                  >
                    {c.sortField ? (
                      <TableSortLabel
                        active={params.sort === c.sortField}
                        direction={params.sort === c.sortField ? params.order : DEFAULT_SORT.order}
                        onClick={() => handleSort(c.sortField as AllocationSortField)}
                      >
                        {c.label}
                      </TableSortLabel>
                    ) : (
                      c.label
                    )}
                  </TableCell>
                ))}
              </TableRow>
            </TableHead>
            <TableBody>
              {isLoading ? (
                Array.from({ length: Math.min(params.rows, 10) }).map((_, i) => (
                  <TableRow key={i}>
                    {COLUMNS.map((c) => (
                      <TableCell key={c.label}>
                        <Skeleton variant="rounded" width="70%" height={18} />
                      </TableCell>
                    ))}
                  </TableRow>
                ))
              ) : isError ? (
                <TableRow>
                  <TableCell colSpan={COLUMNS.length} align="center">
                    <QueryErrorState
                      message={error.message.trim() || "Failed to load allocations."}
                      error={error}
                      onRetry={() => void refetch()}
                    />
                  </TableCell>
                </TableRow>
              ) : allocations.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={COLUMNS.length} align="center" sx={{ py: 4 }}>
                    <Typography variant="body2" color="text.secondary">
                      {hasFilters ? "No allocations match these filters." : "No allocations found."}
                    </Typography>
                  </TableCell>
                </TableRow>
              ) : (
                allocations.map((a) => (
                  <TableRow
                    key={a.id}
                    hover
                    tabIndex={0}
                    aria-label={`View allocation ${a.allocationId ?? a.id}`}
                    onClick={() => open(a)}
                    onKeyDown={(e) => onRowKeyDown(e, a)}
                    sx={{
                      cursor: "pointer",
                      opacity: isFetching ? 0.6 : 1,
                      "&:focus-visible": {
                        outline: "2px solid",
                        outlineColor: "primary.main",
                        outlineOffset: -2,
                      },
                    }}
                  >
                    <TableCell>{a.allocationId ?? "—"}</TableCell>
                    <TableCell sx={{ maxWidth: 240 }}>
                      <Typography variant="body2" noWrap title={a.account?.name}>
                        {a.account?.name ?? "—"}
                      </Typography>
                    </TableCell>
                    <TableCell sx={{ maxWidth: 280 }}>
                      <Typography variant="body2" noWrap title={a.engagement.name}>
                        {a.engagement.name ?? "—"}
                      </Typography>
                      {a.engagement.displayId && (
                        <Typography variant="caption" color="text.secondary">
                          {a.engagement.displayId}
                        </Typography>
                      )}
                    </TableCell>
                    <TableCell>{a.engineer?.name ?? "—"}</TableCell>
                    <TableCell>
                      <AllocationStateChip state={a.state} />
                    </TableCell>
                    <TableCell sx={{ whiteSpace: "nowrap" }}>{formatDateSpan(a.startDate, a.endDate)}</TableCell>
                    <TableCell>{formatUtilization(a.utilization)}</TableCell>
                    <TableCell>{a.latestUpdateDate ? <RelativeTime iso={a.latestUpdateDate} /> : "—"}</TableCell>
                  </TableRow>
                ))
              )}
            </TableBody>
          </Table>
        </TableContainer>
        <TablePagination
          component="div"
          count={total}
          page={params.page}
          onPageChange={(_, page) => update({ page }, false)}
          rowsPerPage={params.rows}
          onRowsPerPageChange={(e) => update({ rows: Number.parseInt(e.target.value, 10) })}
          rowsPerPageOptions={ROWS_PER_PAGE_OPTIONS}
          showFirstButton
          showLastButton
        />
      </Box>
    </Box>
  );
}
