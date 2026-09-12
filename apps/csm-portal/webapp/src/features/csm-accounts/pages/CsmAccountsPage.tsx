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
  Chip,
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
} from "@wso2/oxygen-ui";
import { ArrowLeft } from "@wso2/oxygen-ui-icons-react";
import { useMemo, useState, type ChangeEvent, type JSX, type KeyboardEvent, type ReactNode } from "react";
import { useLocation } from "react-router";
import ColumnCustomizerButton from "@components/column-customizer/ColumnCustomizerButton";
import QueryErrorState from "@components/QueryErrorState";
import { useCurrentUser } from "@context/current-user/CurrentUserContext";
import { useDebouncedValue } from "@hooks/useDebouncedValue";
import { useIdTokenClaims } from "@hooks/useIdTokenClaims";
import {
  getColumnPreferencesUserKey,
  useColumnPreferences,
  type ColumnOption,
} from "@hooks/useColumnPreferences";
import { useNavTransition } from "@hooks/useNavTransition";
import { useSearchAccounts } from "@features/csm-accounts/api/useSearchAccounts";
import {
  resolveAccountTier,
  type Account,
  type SearchAccountsRequest,
} from "@features/csm-accounts/types/csmAccounts";
import { BE_MAX_PAGE_LIMIT } from "@constants/apiConstants";
import RefreshButton from "@components/RefreshButton";

const DEFAULT_ROWS_PER_PAGE = 20;
// Top option is the backend's max page limit; larger requests are rejected.
const ROWS_PER_PAGE_OPTIONS = [10, 20, BE_MAX_PAGE_LIMIT];

type AccountColumnId =
  | "name"
  | "sfId"
  | "tier"
  | "region"
  | "activationDate"
  | "deactivationDate"
  | "accountManager"
  | "technicalOwner"
  | "creTeam";

const ACCOUNT_COLUMNS: { id: AccountColumnId; label: string }[] = [
  { id: "name", label: "Name" },
  { id: "sfId", label: "SF ID" },
  { id: "tier", label: "Tier" },
  { id: "region", label: "Region" },
  { id: "activationDate", label: "Activated" },
  { id: "deactivationDate", label: "Deactivated" },
  { id: "accountManager", label: "Account manager" },
  { id: "technicalOwner", label: "Technical owner" },
  { id: "creTeam", label: "CRE team" },
];

// Matches the table's original, always-shown set. Account manager, technical
// owner, and CRE team are already on the account detail page — available
// here, but opt-in, for anyone who wants them in the list view too.
const DEFAULT_ACCOUNT_COLUMN_IDS: AccountColumnId[] = [
  "name",
  "sfId",
  "tier",
  "region",
  "activationDate",
  "deactivationDate",
];

function formatDate(value?: string | null): string {
  if (!value) return "—";
  const d = new Date(value);
  return Number.isNaN(d.getTime())
    ? value
    : d.toLocaleDateString("en-US", {
        year: "numeric",
        month: "short",
        day: "numeric",
      });
}

function renderAccountCell(id: AccountColumnId, a: Account): ReactNode {
  switch (id) {
    case "name":
      return (
        <Typography variant="body2" noWrap title={a.name}>
          {a.name}
        </Typography>
      );
    case "sfId":
      return a.sfId || "—";
    case "tier": {
      const tier = resolveAccountTier(a);
      return tier ? (
        <Chip
          size="small"
          label={tier}
          color={tier === "enterprise" ? "primary" : "default"}
          variant="outlined"
        />
      ) : (
        "—"
      );
    }
    case "region":
      return a.region ?? "—";
    case "activationDate":
      return formatDate(a.activationDate);
    case "deactivationDate":
      return formatDate(a.deactivationDate);
    case "accountManager":
      return a.accountManager?.name ?? "—";
    case "technicalOwner":
      return a.technicalOwner?.name ?? "—";
    case "creTeam":
      return a.creTeam?.name ?? "—";
  }
}

export default function CsmAccountsPage(): JSX.Element {
  const navigate = useNavTransition();
  const location = useLocation();
  // Set by a dashboard widget's click-through, since this page has no
  // dashboard context of its own.
  const backState = location.state as { from?: string } | undefined;
  const [searchInput, setSearchInput] = useState("");
  const [page, setPage] = useState(0);
  const [rowsPerPage, setRowsPerPage] = useState(DEFAULT_ROWS_PER_PAGE);

  const debouncedSearch = useDebouncedValue(searchInput, 300);

  const request = useMemo<SearchAccountsRequest>(() => {
    const searchQuery = debouncedSearch.trim() || undefined;
    return {
      pagination: { limit: rowsPerPage, offset: page * rowsPerPage },
      // Filter fields are nested under `filters` (matches the `/cases/search`
      // payload shape).
      ...(searchQuery && { filters: { searchQuery } }),
    };
  }, [debouncedSearch, page, rowsPerPage]);

  const { data, isLoading, isFetching, isError, error, refetch, dataUpdatedAt } =
    useSearchAccounts(request);

  const handleSearchChange = (e: ChangeEvent<HTMLInputElement>) => {
    setSearchInput(e.target.value);
    setPage(0);
  };

  const handleChangeRowsPerPage = (e: ChangeEvent<HTMLInputElement>) => {
    setRowsPerPage(parseInt(e.target.value, 10));
    setPage(0);
  };

  const columnOptions = useMemo<ColumnOption[]>(
    () => ACCOUNT_COLUMNS.map(({ id, label }) => ({ id, label })),
    [],
  );
  const currentUser = useCurrentUser().user;
  const currentUserEmail = useIdTokenClaims()?.email;
  const columnPrefs = useColumnPreferences({
    viewId: "accounts",
    userKey: getColumnPreferencesUserKey({ id: currentUser?.id, email: currentUserEmail }),
    columns: columnOptions,
    defaultVisibleIds: DEFAULT_ACCOUNT_COLUMN_IDS,
  });
  const visibleColumnIds = columnPrefs.visibleColumns.map((c) => c.id as AccountColumnId);

  const accounts = data?.accounts ?? [];
  const total = data?.total ?? 0;

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
      {backState?.from && (
        <Button
          variant="text"
          size="small"
          startIcon={<ArrowLeft size={16} />}
          onClick={() => navigate(backState.from as string)}
          sx={{ alignSelf: "flex-start" }}
        >
          Back
        </Button>
      )}

      <Box sx={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 1 }}>
        <Typography variant="body2" color="text.secondary">
          Search across account name and Salesforce ID (case-insensitive).
        </Typography>
        <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
          <RefreshButton
            onRefresh={() => void refetch()}
            isFetching={isFetching}
            updatedAt={dataUpdatedAt}
            label="Refresh accounts"
          />
          <ColumnCustomizerButton
            allColumns={columnPrefs.allColumns}
            isVisible={columnPrefs.isVisible}
            onToggle={columnPrefs.toggleColumn}
            onMove={columnPrefs.moveColumn}
            onReorder={columnPrefs.reorderColumn}
            onReset={columnPrefs.resetToDefault}
            label="Customise accounts columns"
          />
        </Box>
      </Box>

      <TextField
        size="small"
        label="Search accounts"
        placeholder="Search accounts by name or SF ID"
        value={searchInput}
        onChange={handleSearchChange}
        slotProps={{ htmlInput: { "aria-label": "Search accounts by name or SF ID" } }}
        sx={{ maxWidth: 480 }}
      />

      <Box sx={{ border: 1, borderColor: "divider", borderRadius: 1, overflow: "hidden" }}>
        <TableContainer>
          <Table size="small" sx={{ "& .MuiTableCell-root": { borderColor: "divider" } }}>
            <TableHead>
              <TableRow sx={{ bgcolor: "action.hover" }}>
                {visibleColumnIds.map((id) => (
                  <TableCell key={id}>
                    {ACCOUNT_COLUMNS.find((c) => c.id === id)?.label}
                  </TableCell>
                ))}
              </TableRow>
            </TableHead>
            <TableBody>
              {isLoading || isFetching ? (
                Array.from({ length: rowsPerPage }).map((_, i) => (
                  <TableRow key={i}>
                    {visibleColumnIds.map((id) => (
                      <TableCell key={id}>
                        <Skeleton variant="rounded" width="70%" height={18} />
                      </TableCell>
                    ))}
                  </TableRow>
                ))
              ) : isError ? (
                <TableRow>
                  <TableCell colSpan={visibleColumnIds.length} align="center">
                    <QueryErrorState
                      message={error instanceof Error && error.message.trim() ? error.message : "Failed to load accounts."}
                      error={error}
                    />
                  </TableCell>
                </TableRow>
              ) : accounts.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={visibleColumnIds.length} align="center" sx={{ py: 4 }}>
                    <Typography variant="body2" color="text.secondary">
                      No accounts found.
                    </Typography>
                  </TableCell>
                </TableRow>
              ) : (
                accounts.map((a) => {
                  const goToAccount = (): void =>
                    navigate(`/customers/accounts/${a.id}`, {
                      state: { from: `${location.pathname}${location.search}` },
                    });
                  const handleRowKeyDown = (e: KeyboardEvent<HTMLTableRowElement>): void => {
                    if (e.key === "Enter" || e.key === " ") {
                      e.preventDefault();
                      goToAccount();
                    }
                  };
                  return (
                    <TableRow
                      key={a.id}
                      hover
                      onClick={goToAccount}
                      onKeyDown={handleRowKeyDown}
                      tabIndex={0}
                      aria-label={`View account ${a.name}`}
                      sx={{
                        cursor: "pointer",
                        "&:focus-visible": {
                          outline: "2px solid",
                          outlineColor: "primary.main",
                          outlineOffset: -2,
                        },
                      }}
                    >
                      {visibleColumnIds.map((id) => (
                        <TableCell key={id} sx={id === "name" ? { maxWidth: 320 } : undefined}>
                          {renderAccountCell(id, a)}
                        </TableCell>
                      ))}
                    </TableRow>
                  );
                })
              )}
            </TableBody>
          </Table>
        </TableContainer>
        <TablePagination
          component="div"
          count={total}
          page={page}
          onPageChange={(_, newPage) => setPage(newPage)}
          rowsPerPage={rowsPerPage}
          onRowsPerPageChange={handleChangeRowsPerPage}
          rowsPerPageOptions={ROWS_PER_PAGE_OPTIONS}
          showFirstButton
          showLastButton
        />
      </Box>
    </Box>
  );
}
