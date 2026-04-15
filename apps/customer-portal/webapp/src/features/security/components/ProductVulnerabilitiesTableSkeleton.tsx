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

import { TableCell, TableRow, Skeleton } from "@wso2/oxygen-ui";
import { type JSX } from "react";

interface ProductVulnerabilitiesTableSkeletonProps {
  rowsPerPage: number;
}

/**
 * Renders skeleton rows for the Product Vulnerabilities table.
 * @param {ProductVulnerabilitiesTableSkeletonProps} props
 * @returns {JSX.Element}
 */
const ProductVulnerabilitiesTableSkeleton = ({
  rowsPerPage,
}: ProductVulnerabilitiesTableSkeletonProps): JSX.Element => {
  return (
    <>
      {Array.from({ length: rowsPerPage }).map((_, index) => (
        <TableRow key={`skeleton-${index}`}>
          <TableCell>
            <Skeleton variant="text" width={120} height={20} />
          </TableCell>
          <TableCell>
            <Skeleton variant="text" width={100} height={20} />
          </TableCell>
          <TableCell>
            <Skeleton variant="rounded" width={70} height={24} />
          </TableCell>
          <TableCell sx={{ maxWidth: 180 }}>
            <Skeleton variant="text" width={180} height={20} />
          </TableCell>
          <TableCell>
            <Skeleton variant="text" width={80} height={20} />
          </TableCell>
          <TableCell>
            <Skeleton variant="rounded" width={60} height={24} />
          </TableCell>
        </TableRow>
      ))}
    </>
  );
};

export default ProductVulnerabilitiesTableSkeleton;
