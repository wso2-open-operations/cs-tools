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

import { Box, IconButton } from "@wso2/oxygen-ui";
import { PencilLine, Trash2 } from "@wso2/oxygen-ui-icons-react";
import type { JSX } from "react";
import type { DeploymentCardToolbarProps } from "@features/project-details/types/projectDetailsComponents";

/**
 * Edit and delete icon actions for a deployment card header.
 *
 * @param props - Click handlers and pending state.
 * @returns {JSX.Element} Icon button group.
 */
export default function DeploymentCardToolbar({
  onEdit,
  onDelete,
  isDeleteDisabled,
}: DeploymentCardToolbarProps): JSX.Element {
  return (
    <Box sx={{ display: "flex", gap: 0.25, alignItems: "center" }}>
      <IconButton
        component="div"
        size="small"
        role="button"
        aria-label="Edit deployment"
        onClick={onEdit}
        sx={{
          color: "text.secondary",
          "&:hover": { color: "primary.main" },
          "&.Mui-focusVisible": { color: "primary.main" },
        }}
      >
        <PencilLine size={16} aria-hidden />
      </IconButton>
      <IconButton
        component="div"
        size="small"
        role="button"
        aria-label="Delete deployment"
        onClick={onDelete}
        disabled={isDeleteDisabled}
        sx={{
          color: "text.secondary",
          "&:hover": { color: "primary.main" },
          "&.Mui-focusVisible": { color: "primary.main" },
        }}
      >
        <Trash2 size={16} aria-hidden />
      </IconButton>
    </Box>
  );
}
