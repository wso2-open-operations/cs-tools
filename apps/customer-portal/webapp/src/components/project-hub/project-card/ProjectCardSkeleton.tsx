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
  Card,
  Divider,
  Form,
  Skeleton,
  Stack,
  Typography,
} from "@wso2/oxygen-ui";
import { type JSX } from "react";

/**
 * ProjectCardSkeleton component to display a loading placeholder for ProjectCard.
 *
 * @returns {JSX.Element} The rendered Project Card Skeleton.
 */
const ProjectCardSkeleton = (): JSX.Element => {
  return (
    <Card
      sx={{
        alignItems: "stretch",
        display: "flex",
        flexDirection: "column",
        height: "100%",
        width: "100%",
      }}
    >
      {/* project card badges skeleton */}
      <Form.CardContent sx={{ width: "100%", pt: 2, pb: 0 }}>
        <Box display="flex" justifyContent="space-between" alignItems="center">
          <Skeleton variant="rounded" width={80} height={24} />
          <Skeleton variant="rounded" width={60} height={24} />
        </Box>
      </Form.CardContent>

      {/* project card info skeleton */}
      <Form.CardHeader
        sx={{ pt: 1.5 }}
        title={
          <Typography variant="h6" sx={{ mb: 1 }}>
            <Skeleton variant="text" width="80%" height={32} />
          </Typography>
        }
        subheader={
          <Typography
            variant="body2"
            sx={{
              minHeight: "5rem",
              display: "block",
            }}
          >
            <Skeleton variant="text" width="100%" />
            <Skeleton variant="text" width="90%" />
            <Skeleton variant="text" width="40%" />
          </Typography>
        }
      />

      {/* project card stats skeleton */}
      <Form.CardContent sx={{ width: "100%", pb: 0 }}>
        <Box
          sx={{
            display: "flex",
            flexDirection: "column",
            gap: 1.5,
          }}
        >
          {/* Open Cases */}
          <Box
            display="flex"
            justifyContent="space-between"
            alignItems="center"
          >
            <Box display="flex" alignItems="center" gap={1}>
              <Skeleton variant="circular" width={16} height={16} />
              <Typography variant="body2">
                <Skeleton variant="text" width={80} />
              </Typography>
            </Box>
            <Typography variant="body2">
              <Skeleton variant="text" width={20} />
            </Typography>
          </Box>

          {/* Active Chats */}
          <Box
            display="flex"
            justifyContent="space-between"
            alignItems="center"
          >
            <Box display="flex" alignItems="center" gap={1}>
              <Skeleton variant="circular" width={16} height={16} />
              <Typography variant="body2">
                <Skeleton variant="text" width={80} />
              </Typography>
            </Box>
            <Typography variant="body2">
              <Skeleton variant="text" width={20} />
            </Typography>
          </Box>

          {/* Date */}
          <Box display="flex" alignItems="center" gap={1}>
            <Skeleton variant="circular" width={16} height={16} />
            <Typography variant="body2">
              <Skeleton variant="text" width={100} />
            </Typography>
          </Box>
          <Divider sx={{ width: "100%" }} />
        </Box>
      </Form.CardContent>

      {/* project card actions skeleton */}
      <Form.CardActions sx={{ width: "100%", mt: "auto", pt: 1.5, pb: 2 }}>
        <Stack spacing={2} sx={{ width: "100%" }}>
          <Skeleton variant="rounded" width="100%" height={40} />
        </Stack>
      </Form.CardActions>
    </Card>
  );
};

export default ProjectCardSkeleton;
