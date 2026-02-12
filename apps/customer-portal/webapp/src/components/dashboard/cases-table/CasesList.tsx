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
  IconButton,
  Paper,
  Avatar,
  TablePagination,
} from "@wso2/oxygen-ui";
import { ExternalLink, MoreVertical } from "@wso2/oxygen-ui-icons-react";
import { type JSX, type ChangeEvent } from "react";
import type { CaseSearchResponse, CaseListItem } from "@models/responses";
import { getSeverityColor, getStatusColor } from "@utils/casesTable";
import ErrorIndicator from "@components/common/error-indicator/ErrorIndicator";
import CasesTableSkeleton from "@components/dashboard/cases-table/CasesTableSkeleton";

interface CasesListProps {
  isLoading: boolean;
  isError?: boolean;
  data: CaseSearchResponse | undefined;
  page: number;
  rowsPerPage: number;
  onPageChange: (event: unknown, newPage: number) => void;
  onRowsPerPageChange: (event: ChangeEvent<HTMLInputElement>) => void;
  /** When provided, case title is clickable and navigates to case details. */
  onCaseClick?: (caseItem: CaseListItem) => void;
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
}: CasesListProps): JSX.Element => {
  return (
    <>
      <TableContainer component={Paper}>
        <Table sx={{ minWidth: 650 }}>
          <TableHead>
            <TableRow>
              <TableCell>Created</TableCell>
              <TableCell>Case</TableCell>
              <TableCell>Contact</TableCell>
              <TableCell>Assigned to</TableCell>
              <TableCell>Priority</TableCell>
              <TableCell>Status</TableCell>
              <TableCell align="right"></TableCell>
            </TableRow>
          </TableHead>
          <TableBody>
            {isLoading ? (
              <CasesTableSkeleton rowsPerPage={rowsPerPage} />
            ) : isError ? (
              <TableRow>
                <TableCell colSpan={7} align="center">
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
                <TableCell colSpan={7} align="center">
                  No cases found.
                </TableCell>
              </TableRow>
            ) : (
              data?.cases.map((row) => (
                <TableRow key={row.id} hover>
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
                      <Typography
                        component="span"
                        variant="body2"
                        color="text.primary"
                        role={onCaseClick ? "button" : undefined}
                        tabIndex={onCaseClick ? 0 : undefined}
                        onKeyDown={
                          onCaseClick
                            ? (e) => {
                                if (
                                  e.key === "Enter" ||
                                  e.key === " "
                                ) {
                                  e.preventDefault();
                                  onCaseClick(row);
                                }
                              }
                            : undefined
                        }
                        onClick={
                          onCaseClick
                            ? () => onCaseClick(row)
                            : undefined
                        }
                        sx={{
                          display: "inline-flex",
                          alignItems: "center",
                          gap: 0.5,
                          cursor: onCaseClick ? "pointer" : "default",
                          fontWeight: 500,
                          "&:hover": onCaseClick
                            ? { color: "primary.main" }
                            : undefined,
                        }}
                      >
                        {row.title || "--"}
                        <ExternalLink size={12} style={{ opacity: 0.5 }} />
                      </Typography>
                      <Typography variant="caption" color="text.secondary">
                        ID: {row.number}
                      </Typography>
                    </Box>
                  </TableCell>
                  <TableCell>
                    <Typography variant="body2" color="text.secondary">
                      --
                    </Typography>
                  </TableCell>
                  <TableCell>
                    <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
                      <Avatar sx={{ width: 24, height: 24, fontSize: 12 }}>
                        {typeof row.assignedEngineer === "string"
                          ? row.assignedEngineer
                              .split("-")
                              .map((s) => s[0].toUpperCase())
                              .slice(0, 2)
                              .join("")
                          : "?"}
                      </Avatar>
                      <Typography variant="body2" color="text.primary">
                        {typeof row.assignedEngineer === "string"
                          ? row.assignedEngineer
                          : "--"}
                      </Typography>
                    </Box>
                  </TableCell>
                  <TableCell>
                    <Chip
                      label={row.severity?.label || "--"}
                      size="small"
                      variant="outlined"
                      sx={{
                        color: getSeverityColor(row.severity?.label),
                        borderColor: getSeverityColor(row.severity?.label),
                        fontWeight: 500,
                      }}
                    />
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
                  <TableCell align="right">
                    <IconButton size="small">
                      <MoreVertical size={16} />
                    </IconButton>
                  </TableCell>
                </TableRow>
              ))
            )}
          </TableBody>
        </Table>
      </TableContainer>
      <TablePagination
        component="div"
        count={data?.totalRecords || 0}
        page={page}
        onPageChange={onPageChange}
        rowsPerPage={rowsPerPage}
        onRowsPerPageChange={onRowsPerPageChange}
        rowsPerPageOptions={[5, 10, 25, 50]}
      />
    </>
  );
};

export default CasesList;
