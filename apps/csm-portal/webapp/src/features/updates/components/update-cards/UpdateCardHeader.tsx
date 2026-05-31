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

import { Box, Paper, Typography, alpha, useTheme } from "@wso2/oxygen-ui";
import type { JSX } from "react";
import type { UpdateCardHeaderProps } from "@features/updates/types/updates";

/**
 * Header section of the update product card.
 *
 * @param {UpdateCardHeaderProps} props - Component props.
 * @returns {JSX.Element} The rendered component.
 */
export function UpdateCardHeader({
  productName,
  productBaseVersion,
  percentage,
  statusColor,
}: UpdateCardHeaderProps): JSX.Element {
  const theme = useTheme();

  return (
    <Box
      sx={{
        display: "flex",
        justifyContent: "space-between",
        alignItems: "flex-start",
        mb: 2,
      }}
    >
      <Box>
        <Typography variant="body1" fontWeight="medium">
          {productName}
        </Typography>
        <Typography variant="caption" color="text.secondary">
          Version {productBaseVersion}
        </Typography>
      </Box>
      <Paper
        sx={{
          display: "flex",
          alignItems: "center",
          justifyContent: "center",
          px: 1,
          height: 20,
          bgcolor: alpha(theme.palette[statusColor].light, 0.1),
          color: theme.palette[statusColor].light,
          flexShrink: 0,
          borderRadius: 1,
        }}
      >
        <Typography variant="caption">
          {Math.round(percentage)}% Updated
        </Typography>
      </Paper>
    </Box>
  );
}
