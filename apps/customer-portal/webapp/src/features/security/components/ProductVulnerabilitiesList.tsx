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
import { type JSX } from "react";
import { getVulnerabilitySeverityColor } from "@features/security/utils/vulnerabilities";
import ErrorIndicator from "@components/error-indicator/ErrorIndicator";
import ProductVulnerabilitiesTableSkeleton from "@features/security/components/ProductVulnerabilitiesTableSkeleton";
import {
  PRODUCT_VULNERABILITIES_EMPTY_MESSAGE,
  PRODUCT_VULNERABILITIES_FETCH_ERROR_MESSAGE,
  PRODUCT_VULNERABILITIES_TABLE_COLUMN_COUNT,
  PRODUCT_VULNERABILITIES_TABLE_PAGINATION_OPTIONS,
} from "@features/security/constants/securityConstants";
import type { ProductVulnerabilitiesListProps } from "@features/security/types/security";

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
      <TableContainer component={Paper} sx={{ overflowX: "auto" }}>
        <Table sx={{ minWidth: 650 }}>
          <TableHead>
            <TableRow>
              <TableCell>CVE</TableCell>
              <TableCell>Vulnerability ID</TableCell>
              <TableCell>Severity</TableCell>
              <TableCell>Product Name</TableCell>
              <TableCell>Product Version</TableCell>
              <TableCell>Component Name</TableCell>
              <TableCell>Component Version</TableCell>
              <TableCell>Component Type</TableCell>
              <TableCell>Update Level</TableCell>
              <TableCell>Use Case</TableCell>
              <TableCell>Justification</TableCell>
              <TableCell>Resolution</TableCell>
            </TableRow>
          </TableHead>
          <TableBody>
            {isLoading ? (
              <ProductVulnerabilitiesTableSkeleton rowsPerPage={rowsPerPage} />
            ) : isError ? (
              <TableRow>
                <TableCell
                  colSpan={PRODUCT_VULNERABILITIES_TABLE_COLUMN_COUNT}
                  align="center"
                >
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
                      {PRODUCT_VULNERABILITIES_FETCH_ERROR_MESSAGE}
                    </Typography>
                  </Box>
                </TableCell>
              </TableRow>
            ) : !data || !data.vulnerabilities?.length ? (
              <TableRow>
                <TableCell
                  colSpan={PRODUCT_VULNERABILITIES_TABLE_COLUMN_COUNT}
                  align="center"
                >
                  {PRODUCT_VULNERABILITIES_EMPTY_MESSAGE}
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
                    <TableCell>
                      <Typography variant="body2" color="text.secondary">
                        {row.productName ?? "--"}
                      </Typography>
                    </TableCell>
                    <TableCell>
                      <Typography variant="body2" color="text.secondary">
                        {row.productVersion ?? "--"}
                      </Typography>
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
                      <Typography variant="body2" color="text.secondary">
                        {row.componentType ?? "--"}
                      </Typography>
                    </TableCell>
                    <TableCell>
                      <Typography variant="body2" color="text.secondary">
                        {row.updateLevel ?? "--"}
                      </Typography>
                    </TableCell>
                    <TableCell sx={{ maxWidth: 160 }}>
                      <Typography
                        variant="body2"
                        color="text.secondary"
                        noWrap
                        title={row.useCase ?? undefined}
                      >
                        {row.useCase ?? "--"}
                      </Typography>
                    </TableCell>
                    <TableCell sx={{ maxWidth: 160 }}>
                      <Typography
                        variant="body2"
                        color="text.secondary"
                        noWrap
                        title={row.justification ?? undefined}
                      >
                        {row.justification ?? "--"}
                      </Typography>
                    </TableCell>
                    <TableCell sx={{ maxWidth: 160 }}>
                      <Typography
                        variant="body2"
                        color="text.secondary"
                        noWrap
                        title={row.resolution ?? undefined}
                      >
                        {row.resolution ?? "--"}
                      </Typography>
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
        rowsPerPageOptions={[...PRODUCT_VULNERABILITIES_TABLE_PAGINATION_OPTIONS]}
      />
    </>
  );
};

export default ProductVulnerabilitiesList;
