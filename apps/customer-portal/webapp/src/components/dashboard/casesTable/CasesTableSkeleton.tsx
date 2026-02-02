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

import { Box, TableCell, TableRow, Skeleton } from "@wso2/oxygen-ui";
import { type JSX } from "react";

interface CasesTableSkeletonProps {
  rowsPerPage: number;
}

const CasesTableSkeleton = ({
  rowsPerPage,
}: CasesTableSkeletonProps): JSX.Element => {
  return (
    <>
      {Array.from({ length: rowsPerPage }).map((_, index) => (
        <TableRow key={`skeleton-${index}`}>
          <TableCell>
            <Box>
              <Skeleton variant="text" width={80} height={20} />
              <Skeleton variant="text" width={50} height={16} />
            </Box>
          </TableCell>
          <TableCell>
            <Box>
              <Skeleton variant="text" width={200} height={20} />
              <Skeleton variant="text" width={80} height={16} />
            </Box>
          </TableCell>
          <TableCell>
            <Skeleton variant="text" width={150} height={20} />
          </TableCell>
          <TableCell>
            <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
              <Skeleton variant="circular" width={24} height={24} />
              <Skeleton variant="text" width={100} height={20} />
            </Box>
          </TableCell>
          <TableCell>
            <Skeleton variant="rounded" width={60} height={24} />
          </TableCell>
          <TableCell>
            <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
              <Skeleton variant="circular" width={8} height={8} />
              <Skeleton variant="text" width={60} height={20} />
            </Box>
          </TableCell>
          <TableCell align="right">
            <Skeleton variant="circular" width={24} height={24} />
          </TableCell>
        </TableRow>
      ))}
    </>
  );
};

export default CasesTableSkeleton;
