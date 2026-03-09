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

import { Box } from "@wso2/oxygen-ui";
import { type JSX } from "react";
import SupportStatGrid from "@components/common/stat-grid/SupportStatGrid";
import {
  SERVICE_REQUEST_STAT_CONFIGS,
  type ServiceRequestStatKey,
} from "@constants/supportConstants";

export interface ServiceRequestStats {
  pending: number;
  inProgress: number;
  completed: number;
  rejected: number;
}

export interface ServiceRequestsStatCardsProps {
  isLoading: boolean;
  isError?: boolean;
  stats: Partial<Record<ServiceRequestStatKey, number>> | undefined;
}

/**
 * ServiceRequestsStatCards component to display 4 stat cards for service request statistics.
 *
 * @param {ServiceRequestsStatCardsProps} props - Loading state and stats data.
 * @returns {JSX.Element} The rendered stat cards grid.
 */
export default function ServiceRequestsStatCards({
  isLoading,
  isError,
  stats,
}: ServiceRequestsStatCardsProps): JSX.Element {
  return (
    <Box sx={{ mb: 3 }}>
      <SupportStatGrid
        isLoading={isLoading}
        isError={isError}
        entityName="service request"
        configs={SERVICE_REQUEST_STAT_CONFIGS}
        stats={stats}
      />
    </Box>
  );
}
