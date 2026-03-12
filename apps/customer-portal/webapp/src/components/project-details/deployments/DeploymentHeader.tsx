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

import { Box, Button, Skeleton, Typography } from "@wso2/oxygen-ui";
import { Plus } from "@wso2/oxygen-ui-icons-react";
import type { JSX } from "react";

export interface DeploymentHeaderProps {
  count?: number;
  onAddClick?: () => void;
  isLoading?: boolean;
}

/**
 * Header for the deployments section with count and Add Deployment button.
 *
 * @param {DeploymentHeaderProps} props - Props including deployment count and add click handler.
 * @returns {JSX.Element} The deployment header.
 */
export default function DeploymentHeader({
  count = 0,
  onAddClick,
  isLoading = false,
}: DeploymentHeaderProps): JSX.Element {
  return (
    <Box
      sx={{
        display: "flex",
        alignItems: "center",
        justifyContent: "space-between",
        mb: 2,
      }}
    >
      {isLoading ? (
        <Skeleton
          variant="text"
          width={120}
          height={24}
          data-testid="deployment-header-skeleton"
        />
      ) : (
        <Typography variant="body2" color="text.secondary">
          {count} deployment environment{count !== 1 ? "s" : ""}
        </Typography>
      )}
      <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
        <Button
          variant="outlined"
          color="primary"
          startIcon={<Plus />}
          size="small"
          onClick={onAddClick}
        >
          Add Deployment
        </Button>
      </Box>
    </Box>
  );
}
