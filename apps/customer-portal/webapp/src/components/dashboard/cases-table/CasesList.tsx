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
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  Typography,
  Chip,
  Paper,
  Avatar,
  TablePagination,
  alpha,
} from "@wso2/oxygen-ui";
import { type JSX, type ChangeEvent } from "react";
import type { CaseSearchResponse, CaseListItem } from "@/types/cases";
import {
  formatValue,
  getInitials,
  getStatusColor,
  mapSeverityToDisplay,
} from "@utils/support";
import { getSeverityLegendColor } from "@constants/dashboardConstants";
import ErrorIndicator from "@components/common/error-indicator/ErrorIndicator";
import CasesTableSkeleton from "@components/dashboard/cases-table/CasesTableSkeleton";
import EmptyIcon from "@components/common/empty-state/EmptyIcon";
import SearchNoResultsIcon from "@components/common/empty-state/SearchNoResultsIcon";

interface CasesListProps {
  isLoading: boolean;
  isError?: boolean;
  data: CaseSearchResponse | undefined;
  page: number;
  rowsPerPage: number;
  onPageChange: (event: unknown, newPage: number) => void;
  onRowsPerPageChange: (event: ChangeEvent<HTMLInputElement>) => void;
  onCaseClick?: (caseItem: CaseListItem) => void;
  showPagination?: boolean;
  hasListRefinement?: boolean;
}

const CasesList = ({
  isLoading,
  isError,
  data,
  page,
  rowsPerPage,
  onPageChange,
  onRowsPerPageChange,
  onCaseClick,
  showPagination = true,
  hasListRefinement = false,
}: CasesListProps): JSX.Element => {
  return (
    <>
      <TableContainer component={Paper}>
        <Table sx={{ minWidth: 650 }}>
          <TableHead>
            <TableRow>
              <TableCell>Created</TableCell>
              <TableCell>Details</TableCell>
              <TableCell>Severity</TableCell>
              <TableCell>Assigned to</TableCell>
              <TableCell>Status</TableCell>
            </TableRow>
          </TableHead>
          <TableBody>
            {isLoading ? (
              <CasesTableSkeleton rowsPerPage={rowsPerPage} />
            ) : isError ? (
              <TableRow>
                <TableCell colSpan={5} align="center">
                  <Box
                    sx={{
                      display: "flex",
                      alignItems: "center",
                      justifyContent: "center",
                      gap: 1,
                    }}
                  >
                    <ErrorIndicator entityName="cases" />
                    <Typography variant="body2" color="error">
                      Failed to fetch outstanding cases
                    </Typography>
                  </Box>
                </TableCell>
              </TableRow>
            ) : data?.cases.length === 0 ? (
              <TableRow>
                <TableCell colSpan={5} align="center">
                  <Box
                    sx={{
                      display: "flex",
                      flexDirection: "column",
                      alignItems: "center",
                      py: 4,
                    }}
                  >
                    {hasListRefinement ? (
                      <>
                        <SearchNoResultsIcon
                          style={{
                            width: 160,
                            maxWidth: "100%",
                            height: "auto",
                            marginBottom: 12,
                          }}
                        />
                        <Typography variant="body2" color="text.secondary">
                          No outstanding cases. Try adjusting your filters.
                        </Typography>
                      </>
                    ) : (
                      <>
                        <EmptyIcon
                          style={{
                            width: 160,
                            maxWidth: "100%",
                            height: "auto",
                            marginBottom: 12,
                          }}
                        />
                        <Typography variant="body2" color="text.secondary">
                          No outstanding cases.
                        </Typography>
                      </>
                    )}
                  </Box>
                </TableCell>
              </TableRow>
            ) : (
              data?.cases.map((row) => (
                <TableRow
                  key={row.id}
                  hover
                  onClick={onCaseClick ? () => onCaseClick(row) : undefined}
                  sx={{
                    cursor: onCaseClick ? "pointer" : "default",
                  }}
                >
                  <TableCell>
                    <Box>
                      <Typography variant="body2" color="text.primary">
                        {row.createdOn ? row.createdOn.split(" ")[0] : "--"}
                      </Typography>
                      <Typography variant="caption" color="text.secondary">
                        {row.createdOn ? row.createdOn.split(" ")[1] || "" : ""}
                      </Typography>
                    </Box>
                  </TableCell>
                  <TableCell>
                    <Box
                      sx={{
                        display: "flex",
                        flexDirection: "column",
                        gap: 0.25,
                      }}
                    >
                      <Typography variant="body2" color="text.primary">
                        {row.title || "--"}
                      </Typography>
                      <Typography variant="caption" color="text.secondary">
                        ID: {row.number}
                      </Typography>
                    </Box>
                  </TableCell>
                  <TableCell>
                    {(() => {
                      const severityColor = getSeverityLegendColor(
                        row.severity?.label,
                      );
                      return (
                        <Chip
                          label={mapSeverityToDisplay(row.severity?.label)}
                          size="small"
                          variant="outlined"
                          sx={{
                            bgcolor: alpha(severityColor, 0.1),
                            color: severityColor,
                            borderColor: alpha(severityColor, 0.3),
                            fontWeight: 500,
                            px: 0,
                            height: 20,
                            fontSize: "0.75rem",
                            "& .MuiChip-label": {
                              pl: "6px",
                              pr: "6px",
                            },
                          }}
                        />
                      );
                    })()}
                  </TableCell>
                  <TableCell>
                    <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
                      <Avatar sx={{ width: 24, height: 24, fontSize: 12 }}>
                        {getInitials(row.assignedEngineer)}
                      </Avatar>
                      <Typography variant="body2" color="text.primary">
                        {formatValue(row.assignedEngineer)}
                      </Typography>
                    </Box>
                  </TableCell>
                  <TableCell>
                    <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
                      <Box
                        sx={{
                          width: 8,
                          height: 8,
                          borderRadius: "50%",
                          backgroundColor: getStatusColor(
                            row.status?.label || "",
                          ),
                        }}
                      />
                      <Typography variant="body2" color="text.primary">
                        {row.status?.label || "--"}
                      </Typography>
                    </Box>
                  </TableCell>
                </TableRow>
              ))
            )}
          </TableBody>
        </Table>
      </TableContainer>
      {showPagination && (
        <TablePagination
          component="div"
          count={data?.totalRecords || 0}
          page={page}
          onPageChange={onPageChange}
          rowsPerPage={rowsPerPage}
          onRowsPerPageChange={onRowsPerPageChange}
          rowsPerPageOptions={[5, 10, 25, 50]}
        />
      )}
    </>
  );
};

export default CasesList;
