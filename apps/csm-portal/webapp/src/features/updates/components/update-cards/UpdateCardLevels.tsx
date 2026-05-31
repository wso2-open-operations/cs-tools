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

import { Box, Grid, Paper, Typography } from "@wso2/oxygen-ui";
import type { JSX } from "react";
import type { UpdateCardLevelsProps } from "@features/updates/types/updates";

/**
 * Component to display level highlights (Current, Latest, Pending).
 *
 * @param {UpdateCardLevelsProps} props - Component props.
 * @returns {JSX.Element} The rendered component.
 */
export function UpdateCardLevels({
  currentUpdateLevel,
  recommendedUpdateLevel,
  pendingLevels,
}: UpdateCardLevelsProps): JSX.Element {
  return (
    <Paper sx={{ bgcolor: "action.hover", p: 1.5, mb: 2 }}>
      <Grid container justifyContent="space-between" alignItems="center">
        <Box>
          <Typography variant="caption" color="text.secondary" display="block">
            Current Level
          </Typography>
          <Typography variant="h6" fontWeight="bold">
            U{currentUpdateLevel}
          </Typography>
        </Box>
        <Box sx={{ textAlign: "center" }}>
          <Typography variant="caption" color="text.secondary" display="block">
            Latest Level
          </Typography>
          <Typography variant="h6" color="text.primary" fontWeight="bold">
            U{recommendedUpdateLevel}
          </Typography>
        </Box>
        <Box sx={{ textAlign: "right" }}>
          <Typography variant="caption" color="text.secondary" display="block">
            Pending Levels
          </Typography>
          <Typography variant="h6" fontWeight="bold">
            {pendingLevels}
          </Typography>
        </Box>
      </Grid>
    </Paper>
  );
}
