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

import { Box, Card, CardContent, Grid, Paper, Skeleton } from "@wso2/oxygen-ui";
import type { JSX } from "react";

/**
 * Skeleton loader for the UpdateProductCard component.
 *
 * @returns {JSX.Element} The rendered skeleton.
 */
export function UpdateProductCardSkeleton(): JSX.Element {
  return (
    <Card>
      <CardContent sx={{ p: 2 }}>
        {/* Header Section */}
        <Box
          sx={{
            display: "flex",
            justifyContent: "space-between",
            alignItems: "flex-start",
            mb: 2,
          }}
        >
          <Box>
            <Skeleton variant="text" width={140} height={24} sx={{ mb: 0.5 }} />
            <Skeleton variant="text" width={80} height={16} />
          </Box>
          <Skeleton
            variant="rectangular"
            width={80}
            height={20}
            sx={{ borderRadius: 1 }}
          />
        </Box>

        {/* Level Highlights */}
        <Paper sx={{ bgcolor: "action.hover", p: 1.5, mb: 2 }}>
          <Grid container justifyContent="space-between" alignItems="center">
            <Box>
              <Skeleton
                variant="text"
                width={60}
                height={16}
                sx={{ mb: 0.5 }}
              />
              <Skeleton variant="text" width={40} height={32} />
            </Box>
            <Box sx={{ textAlign: "center" }}>
              <Skeleton
                variant="text"
                width={60}
                height={16}
                sx={{ mb: 0.5 }}
              />
              <Skeleton variant="text" width={40} height={32} />
            </Box>
            <Box sx={{ textAlign: "right" }}>
              <Skeleton
                variant="text"
                width={60}
                height={16}
                sx={{ mb: 0.5 }}
              />
              <Skeleton variant="text" width={40} height={32} />
            </Box>
          </Grid>
        </Paper>

        {/* Breakdown Stats */}
        <Grid container spacing={1} sx={{ mb: 2 }}>
          <Grid size={{ xs: 6 }}>
            <Paper sx={{ p: 1, textAlign: "center" }}>
              <Skeleton
                variant="text"
                width={50}
                height={16}
                sx={{ mx: "auto", mb: 0.5 }}
              />
              <Skeleton
                variant="text"
                width={30}
                height={24}
                sx={{ mx: "auto", mb: 0.5 }}
              />
              <Skeleton
                variant="text"
                width={60}
                height={16}
                sx={{ mx: "auto" }}
              />
            </Paper>
          </Grid>
          <Grid size={{ xs: 6 }}>
            <Paper sx={{ p: 1, textAlign: "center" }}>
              <Skeleton
                variant="text"
                width={50}
                height={16}
                sx={{ mx: "auto", mb: 0.5 }}
              />
              <Skeleton
                variant="text"
                width={30}
                height={24}
                sx={{ mx: "auto", mb: 0.5 }}
              />
              <Skeleton
                variant="text"
                width={60}
                height={16}
                sx={{ mx: "auto" }}
              />
            </Paper>
          </Grid>
        </Grid>

        {/* Progress Bar */}
        <Box sx={{ mb: 2 }}>
          <Skeleton
            variant="rectangular"
            height={6}
            sx={{ borderRadius: 1.5 }}
          />
        </Box>

        {/* Action Button */}
        <Skeleton variant="rectangular" height={36} sx={{ borderRadius: 1 }} />
      </CardContent>
    </Card>
  );
}
