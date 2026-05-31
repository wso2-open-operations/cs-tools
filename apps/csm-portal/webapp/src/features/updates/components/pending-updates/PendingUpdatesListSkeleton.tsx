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
  Paper,
  Skeleton,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
} from "@wso2/oxygen-ui";
import type { JSX } from "react";

/**
 * Skeleton component for the pending updates list.
 *
 * @returns {JSX.Element} The rendered skeleton.
 */
export default function PendingUpdatesListSkeleton(): JSX.Element {
  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
      <Skeleton variant="text" width="60%" height={24} />
      <TableContainer component={Paper} variant="outlined">
        <Table>
          <TableHead>
            <TableRow>
              <TableCell>
                <Skeleton variant="text" width={80} height={20} />
              </TableCell>
              <TableCell>
                <Skeleton variant="text" width={80} height={20} />
              </TableCell>
              <TableCell>
                <Skeleton variant="text" width={60} height={20} />
              </TableCell>
            </TableRow>
          </TableHead>
          <TableBody>
            {[1, 2, 3, 4, 5].map((i) => (
              <TableRow key={i}>
                <TableCell>
                  <Skeleton variant="text" width={40} height={20} />
                </TableCell>
                <TableCell>
                  <Skeleton variant="rounded" width={60} height={24} />
                </TableCell>
                <TableCell>
                  <Skeleton variant="text" width={40} height={20} />
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </TableContainer>
    </Box>
  );
}
