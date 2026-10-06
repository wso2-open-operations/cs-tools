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

// `error` is whatever React Query's useQuery reports (an Error, usually a
// BackendApiError with `.status` — see @api/backend/client.ts).
import type { ChangeEvent, MouseEvent } from "react";
import {
  Alert,
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
  useTheme,
} from "@wso2/oxygen-ui";
import {
  ChevronsLeftIcon,
  ChevronsRightIcon,
  ChevronLeftIcon,
  ChevronRightIcon,
} from "@wso2/oxygen-ui-icons-react";
import LinearLoading from "./LinearLoading";
import NoDataAvailable from "./NoDataAvailable";

// eslint-disable-next-line @typescript-eslint/no-explicit-any -- row shape is caller-defined per table instance
export interface TableDataProps<T = any> {
  data: T[] | undefined;
  loading: boolean;
  error?: Error | null;
  page: number;
  setPage: (page: number) => void;
  rowsPerPage: number;
  setRowsPerPage: (rowsPerPage: number) => void;
  colNameArray: string[];
  colAttributeArray: string[];
  handleRowClick?: (rowData: T) => void;
}

export default function DefaultTable<T extends Record<string, unknown>>(props: TableDataProps<T>) {
  const handleChangePage = (_event: MouseEvent<HTMLButtonElement> | null, newPage: number) => {
    props.setPage(newPage);
  };

  const handleChangeRowsPerPage = (event: ChangeEvent<HTMLInputElement | HTMLTextAreaElement>) => {
    props.setRowsPerPage(parseInt(event.target.value, 10));
    props.setPage(0);
  };

  if (props.loading) return <LinearLoading />;
  if (props.error) return <Alert severity="error">{props.error.message}</Alert>;
  if (!props.data) return null;

  return (
    <PopulateTable
      data={props.data}
      rowsPerPage={props.rowsPerPage}
      page={props.page}
      handleChangePage={handleChangePage}
      handleChangeRowsPerPage={handleChangeRowsPerPage}
      colNameArray={props.colNameArray}
      colAttributeArray={props.colAttributeArray}
      handleRowClick={props.handleRowClick}
    />
  );
}

function PopulateTable<T extends Record<string, unknown>>({
  data,
  rowsPerPage,
  page,
  handleChangePage,
  handleChangeRowsPerPage,
  colNameArray,
  colAttributeArray,
  handleRowClick,
}: {
  data: T[];
  rowsPerPage: number;
  page: number;
  handleChangePage: (event: MouseEvent<HTMLButtonElement> | null, newPage: number) => void;
  handleChangeRowsPerPage: (event: ChangeEvent<HTMLInputElement | HTMLTextAreaElement>) => void;
  colNameArray: string[];
  colAttributeArray: string[];
  handleRowClick?: (rowData: T) => void;
}) {
  const theme = useTheme();

  if (data.length === 0) {
    return (
      <NoDataAvailable
        message="No data available"
        description="There are no items to display. Try adjusting your filters or check back later."
      />
    );
  }

  return (
    <Paper
      variant="outlined"
      sx={{
        width: "100%",
        borderRadius: "8px",
        overflow: "hidden",
      }}
    >
      <TableContainer sx={{ maxHeight: "70vh", overflow: "auto" }}>
        <Table stickyHeader aria-label="sticky table">
          <TableHead>
            <TableRow>
              {colNameArray.map((value, index) => (
                <TableCell
                  key={index}
                  sx={{
                    backgroundColor: theme.palette.background.paper,
                    fontWeight: "bold",
                  }}
                >
                  {value}
                </TableCell>
              ))}
            </TableRow>
          </TableHead>
          <TableBody>
            {data.map((dataRow, index) => (
              <TableRow
                key={index}
                hover
                onClick={() => handleRowClick?.(dataRow)}
                sx={handleRowClick ? { cursor: "pointer" } : undefined}
              >
                {colAttributeArray.map((attributeName, i) => (
                  <TableCell key={i}>{String(dataRow[attributeName] ?? "")}</TableCell>
                ))}
              </TableRow>
            ))}
          </TableBody>
          <TableFooter>
            <TableRow>
              <TablePagination
                rowsPerPageOptions={[5, 10, 25]}
                count={-1}
                rowsPerPage={rowsPerPage}
                page={page}
                slotProps={{ select: { inputProps: { "aria-label": "rows per page" }, native: false } }}
                onPageChange={handleChangePage}
                onRowsPerPageChange={handleChangeRowsPerPage}
                ActionsComponent={({ page: p, onPageChange }) => (
                  <PaginationActions page={p} rowsPerPage={rowsPerPage} onPageChange={onPageChange} currentDataLength={data.length} />
                )}
              />
            </TableRow>
          </TableFooter>
        </Table>
      </TableContainer>
    </Paper>
  );
}

function PaginationActions({
  page,
  rowsPerPage,
  onPageChange,
  currentDataLength,
}: {
  page: number;
  rowsPerPage: number;
  onPageChange: (event: MouseEvent<HTMLButtonElement>, newPage: number) => void;
  currentDataLength: number;
}) {
  const theme = useTheme();
  // No "last page" control: these list endpoints report no total count.
  const isNextDisabled = currentDataLength < rowsPerPage;
  const isRtl = theme.direction === "rtl";

  return (
    <Box sx={{ flexShrink: 0, ml: 2.5, display: "flex" }}>
      <IconButton onClick={(e) => onPageChange(e, 0)} disabled={page === 0} aria-label="first page">
        {isRtl ? <ChevronsRightIcon size={18} /> : <ChevronsLeftIcon size={18} />}
      </IconButton>
      <IconButton onClick={(e) => onPageChange(e, page - 1)} disabled={page === 0} aria-label="previous page">
        {isRtl ? <ChevronRightIcon size={18} /> : <ChevronLeftIcon size={18} />}
      </IconButton>
      <IconButton onClick={(e) => onPageChange(e, page + 1)} disabled={isNextDisabled} aria-label="next page">
        {isRtl ? <ChevronLeftIcon size={18} /> : <ChevronRightIcon size={18} />}
      </IconButton>
    </Box>
  );
}
