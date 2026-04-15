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
  TablePagination,
} from "@wso2/oxygen-ui";
import { type JSX, type ChangeEvent } from "react";
import { getVulnerabilitySeverityColor } from "@features/security/utils/vulnerabilities";
import ErrorIndicator from "@components/error-indicator/ErrorIndicator";
import ProductVulnerabilitiesTableSkeleton from "@features/security/components/ProductVulnerabilitiesTableSkeleton";
import type { ProductVulnerability } from "@features/security/types/security";

export interface ProductVulnerabilitiesListData {
  vulnerabilities: ProductVulnerability[];
  totalRecords: number;
}

interface ProductVulnerabilitiesListProps {
  isLoading: boolean;
  isError?: boolean;
  data: ProductVulnerabilitiesListData | undefined;
  page: number;
  rowsPerPage: number;
  onPageChange: (event: unknown, newPage: number) => void;
  onRowsPerPageChange: (event: ChangeEvent<HTMLInputElement>) => void;
  onVulnerabilityClick?: (vulnerability: ProductVulnerability) => void;
}

const COLUMN_COUNT = 6;

/**
 * Renders the Product Vulnerabilities table list.
 * @returns {JSX.Element}
 */
const ProductVulnerabilitiesList = ({
  isLoading,
  isError,
  data,
  page,
  rowsPerPage,
  onPageChange,
  onRowsPerPageChange,
  onVulnerabilityClick,
}: ProductVulnerabilitiesListProps): JSX.Element => {
  return (
    <>
      <TableContainer component={Paper}>
        <Table sx={{ minWidth: 650 }}>
          <TableHead>
            <TableRow>
              <TableCell>CVE ID</TableCell>
              <TableCell>Vulnerability ID</TableCell>
              <TableCell>Severity</TableCell>
              <TableCell>Component Name</TableCell>
              <TableCell>Version</TableCell>
              <TableCell>Type</TableCell>
            </TableRow>
          </TableHead>
          <TableBody>
            {isLoading ? (
              <ProductVulnerabilitiesTableSkeleton rowsPerPage={rowsPerPage} />
            ) : isError ? (
              <TableRow>
                <TableCell colSpan={COLUMN_COUNT} align="center">
                  <Box
                    sx={{
                      display: "flex",
                      alignItems: "center",
                      justifyContent: "center",
                      gap: 1,
                    }}
                  >
                    <ErrorIndicator entityName="vulnerabilities" />
                    <Typography variant="body2" color="error">
                      Failed to fetch product vulnerabilities
                    </Typography>
                  </Box>
                </TableCell>
              </TableRow>
            ) : !data || !data.vulnerabilities?.length ? (
              <TableRow>
                <TableCell colSpan={COLUMN_COUNT} align="center">
                  No vulnerabilities found.
                </TableCell>
              </TableRow>
            ) : (
              data?.vulnerabilities.map((row) => {
                const severityLabel = row.severity?.label ?? "--";
                return (
                  <TableRow
                    key={row.id}
                    hover
                    onClick={() => onVulnerabilityClick?.(row)}
                    sx={{
                      cursor: onVulnerabilityClick ? "pointer" : undefined,
                    }}
                  >
                    <TableCell>
                      <Typography variant="body2" fontWeight="medium">
                        {row.cveId}
                      </Typography>
                    </TableCell>
                    <TableCell>
                      <Typography variant="body2" color="text.secondary">
                        {row.vulnerabilityId}
                      </Typography>
                    </TableCell>
                    <TableCell>
                      <Chip
                        label={severityLabel}
                        size="small"
                        variant="outlined"
                        sx={{
                          color: getVulnerabilitySeverityColor(severityLabel),
                          borderColor:
                            getVulnerabilitySeverityColor(severityLabel),
                          fontWeight: 500,
                        }}
                      />
                    </TableCell>
                    <TableCell sx={{ maxWidth: 180 }}>
                      <Typography
                        variant="body2"
                        noWrap
                        title={row.componentName}
                      >
                        {row.componentName}
                      </Typography>
                    </TableCell>
                    <TableCell>
                      <Typography variant="body2" color="text.secondary">
                        {row.version}
                      </Typography>
                    </TableCell>
                    <TableCell>
                      <Chip label={row.type} variant="outlined" size="small" />
                    </TableCell>
                  </TableRow>
                );
              })
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

export default ProductVulnerabilitiesList;
