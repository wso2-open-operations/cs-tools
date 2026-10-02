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

// Ported from apps/support-portal-lite/webapp's own
// features/spl/cases/components/DefaultTable.tsx — generic, reusable
// paginated table. Kept local to features/spl/cases/ (not shared across
// domains) to avoid cross-domain file collisions, same precedent the
// source app itself already established (see its own file header comment).
// Error prop type swapped from useSplApi's GetApiResponseError to this
// app's own React-Query Error (BackendApiError | null), otherwise
// unchanged.
import type { ChangeEvent, MouseEvent } from "react";
import {
  Box,
  IconButton,
  Paper,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableFooter,
  TableHead,
  TablePagination,
  TableRow,
} from "@wso2/oxygen-ui";
import { alpha, useTheme, useColorScheme } from "@mui/material/styles";
import { ChevronsLeftIcon, ChevronsRightIcon, ChevronLeftIcon, ChevronRightIcon } from "@wso2/oxygen-ui-icons-react";
import type { CaseDetailsWithCount, DataStruct } from "../api/caseTypes";
import { ErrorPanel, LinearLoadingPanel, NoDataPanel } from "./StatePanels";

export interface TableDataProps {
  data: DataStruct[] | CaseDetailsWithCount | undefined;
  loading: boolean;
  error: Error | null | undefined;
  page: number;
  setPage: (page: number) => void;
  rowsPerPage: number;
  setRowsPerPage: (rowsPerPage: number) => void;
  colNameArray: string[];
  colAttributeArray: string[];
  handleRowClick?: (rowData: DataStruct) => void;
}

function isCaseDetailsWithCount(
  data: DataStruct[] | CaseDetailsWithCount | undefined,
): data is CaseDetailsWithCount {
  return (
    !!data &&
    !Array.isArray(data) &&
    typeof (data as CaseDetailsWithCount).count === "number" &&
    Array.isArray((data as CaseDetailsWithCount).cases)
  );
}

export default function DefaultTable(props: TableDataProps) {
  const handleChangePage = (_event: unknown, newPage: number) => props.setPage(newPage);
  const handleChangeRowsPerPage = (event: ChangeEvent<HTMLInputElement | HTMLTextAreaElement>) => {
    props.setRowsPerPage(parseInt(event.target.value, 10));
    props.setPage(0);
  };

  const { data } = props;
  const { count, actualData, hasTotal } = isCaseDetailsWithCount(data)
    ? { count: data.count, actualData: data.cases as DataStruct[], hasTotal: true }
    : Array.isArray(data)
      ? { count: -1, actualData: data, hasTotal: false }
      : { count: 0, actualData: [] as DataStruct[], hasTotal: false };

  if (props.loading) return <LinearLoadingPanel />;
  if (props.error) return <ErrorPanel />;
  if (!data) return null;

  return (
    <PopulateTable
      data={actualData}
      rowsPerPage={props.rowsPerPage}
      page={props.page}
      count={count}
      hasTotal={hasTotal}
      handleChangePage={handleChangePage}
      handleChangeRowsPerPage={handleChangeRowsPerPage}
      colNameArray={props.colNameArray}
      colAttributeArray={props.colAttributeArray}
      handleRowClick={props.handleRowClick}
    />
  );
}

function PopulateTable({
  data,
  rowsPerPage,
  page,
  count,
  hasTotal,
  handleChangePage,
  handleChangeRowsPerPage,
  colNameArray,
  colAttributeArray,
  handleRowClick,
}: {
  data: DataStruct[];
  rowsPerPage: number;
  page: number;
  count: number;
  hasTotal: boolean;
  handleChangePage: (event: unknown, newPage: number) => void;
  handleChangeRowsPerPage: (event: ChangeEvent<HTMLInputElement | HTMLTextAreaElement>) => void;
  colNameArray: string[];
  colAttributeArray: string[];
  handleRowClick?: (rowData: DataStruct) => void;
}) {
  const theme = useTheme();
  // theme.palette.mode is not live under oxygen-ui's CSS-variables theme
  // (extendTheme()) — confirmed empirically. useColorScheme() is the hook
  // that actually tracks the live scheme.
  const { mode: colorMode, systemMode } = useColorScheme();
  const isDark = (colorMode === "system" ? systemMode : colorMode) === "dark";
  // Warm-orange hover identity used throughout this domain (CaseStateCard,
  // SearchResultBox) — an alpha overlay composites correctly against either
  // mode's row background instead of a literal light-peach hex.
  const rowHoverBg = alpha("#ff7300", isDark ? 0.24 : 0.35);

  if (data.length === 0) return <NoDataPanel />;

  return (
    <Paper
      sx={{
        width: "100%",
        border: `2px solid ${theme.palette.divider}`,
        borderRadius: "8px",
        overflow: "hidden",
        boxShadow: "0 2px 8px rgba(0,0,0,0.1)",
      }}
    >
      <TableContainer sx={{ maxHeight: "70vh", overflow: "auto" }}>
        <Table stickyHeader aria-label="sticky table">
          <TableHead>
            <TableRow>
              {colNameArray.map((value) => (
                <TableCell
                  key={value}
                  sx={{
                    backgroundColor: isDark ? theme.palette.grey[800] : "#e0e0e0",
                    color: theme.palette.text.primary,
                    borderBottom: `2px solid ${theme.palette.divider}`,
                    fontWeight: "bold",
                    padding: "12px 16px",
                  }}
                >
                  {value}
                </TableCell>
              ))}
            </TableRow>
          </TableHead>
          <TableBody>
            {data.map((dataRow, rowIndex) => (
              <TableRow
                key={rowIndex}
                onClick={() => handleRowClick?.(dataRow)}
                sx={{
                  ...(handleRowClick ? { cursor: "pointer" } : {}),
                  "&:nth-of-type(even)": { backgroundColor: theme.palette.action.hover },
                  "&:hover": { backgroundColor: rowHoverBg },
                  borderBottom: `1px solid ${theme.palette.divider}`,
                }}
              >
                {colAttributeArray.map((attributeName) => (
                  <TableCell key={attributeName}>{String(dataRow[attributeName] ?? "")}</TableCell>
                ))}
              </TableRow>
            ))}
          </TableBody>
          <TableFooter />
        </Table>
      </TableContainer>

      <Box
        sx={{
          borderTop: "3px solid rgb(133, 142, 149)",
          backgroundColor: isDark ? theme.palette.grey[900] : "#f8f9fa",
          padding: "8px 16px",
          position: "sticky",
          bottom: 0,
        }}
      >
        <TablePagination
          rowsPerPageOptions={[5, 10, 25]}
          component="div"
          count={count}
          rowsPerPage={rowsPerPage}
          page={page}
          onPageChange={handleChangePage}
          onRowsPerPageChange={handleChangeRowsPerPage}
          slotProps={{ select: { inputProps: { "aria-label": "rows per page" }, native: false } }}
          ActionsComponent={(actionProps) => (
            <TablePaginationActions {...actionProps} hasTotal={hasTotal} currentDataLength={data.length} />
          )}
        />
      </Box>
    </Paper>
  );
}

function TablePaginationActions({
  count,
  page,
  rowsPerPage,
  onPageChange,
  hasTotal,
  currentDataLength,
}: {
  count: number;
  page: number;
  rowsPerPage: number;
  onPageChange: (event: MouseEvent<HTMLButtonElement>, newPage: number) => void;
  hasTotal: boolean;
  currentDataLength: number;
}) {
  const theme = useTheme();

  const isNextDisabled = hasTotal && count !== -1 ? page >= Math.ceil(count / rowsPerPage) - 1 : currentDataLength < rowsPerPage;
  const isLastDisabled = hasTotal && count !== -1 ? page >= Math.ceil(count / rowsPerPage) - 1 : true;

  return (
    <Box sx={{ flexShrink: 0, ml: 2.5 }}>
      <IconButton onClick={(e) => onPageChange(e, 0)} disabled={page === 0} aria-label="first page">
        {theme.direction === "rtl" ? <ChevronsRightIcon /> : <ChevronsLeftIcon />}
      </IconButton>
      <IconButton onClick={(e) => onPageChange(e, page - 1)} disabled={page === 0} aria-label="previous page">
        {theme.direction === "rtl" ? <ChevronRightIcon /> : <ChevronLeftIcon />}
      </IconButton>
      <IconButton onClick={(e) => onPageChange(e, page + 1)} disabled={isNextDisabled} aria-label="next page">
        {theme.direction === "rtl" ? <ChevronLeftIcon /> : <ChevronRightIcon />}
      </IconButton>
      <IconButton
        onClick={(e) => onPageChange(e, Math.max(0, Math.ceil(count / rowsPerPage) - 1))}
        disabled={isLastDisabled}
        aria-label="last page"
      >
        {theme.direction === "rtl" ? <ChevronsLeftIcon /> : <ChevronsRightIcon />}
      </IconButton>
    </Box>
  );
}
