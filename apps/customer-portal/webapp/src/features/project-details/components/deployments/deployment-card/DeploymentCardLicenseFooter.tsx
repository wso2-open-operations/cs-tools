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

import { Box, Button } from "@wso2/oxygen-ui";
import { Calendar, Key } from "@wso2/oxygen-ui-icons-react";
import type { JSX } from "react";
import type { DeploymentCardLicenseFooterProps } from "@features/project-details/types/projectDetailsComponents";

/**
 * Created/updated timestamps and license download for a deployment card.
 *
 * @param props - Formatted date strings and license action.
 * @returns {JSX.Element} Footer row.
 */
export default function DeploymentCardLicenseFooter({
  createdAtLabel,
  updatedAtLabel,
  onDownloadLicense,
  isDownloading,
}: DeploymentCardLicenseFooterProps): JSX.Element {
  return (
    <Box
      sx={{
        display: "flex",
        alignItems: "center",
        justifyContent: "space-between",
        gap: 1,
        color: "text.secondary",
        fontSize: "0.75rem",
      }}
    >
      <Box
        sx={{
          display: "inline-flex",
          alignItems: "center",
          gap: 0.5,
          flexShrink: 0,
        }}
      >
        <Calendar
          size={14}
          style={{
            verticalAlign: "middle",
            display: "inline-block",
            marginTop: "-2px",
          }}
        />
        <span style={{ verticalAlign: "middle", whiteSpace: "nowrap" }}>
          Created on {createdAtLabel} • Updated on {updatedAtLabel}
        </span>
      </Box>
      <Button
        startIcon={<Key size={16} />}
        onClick={onDownloadLicense}
        loading={isDownloading}
        loadingPosition="start"
        variant="outlined"
        size="small"
        sx={{
          textTransform: "none",
        }}
      >
        Download License
      </Button>
    </Box>
  );
}
