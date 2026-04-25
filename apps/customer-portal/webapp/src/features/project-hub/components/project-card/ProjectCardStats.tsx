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
  Divider,
  Form,
  Skeleton,
  Typography,
  colors,
} from "@wso2/oxygen-ui";
import { AlertTriangle, CircleAlert, MessageSquare } from "@wso2/oxygen-ui-icons-react";
import { type JSX } from "react";
import ErrorIndicator from "@components/error-indicator/ErrorIndicator";
import {
  PROJECT_CARD_ERROR_ENTITY_ACTIVE_CHATS,
  PROJECT_CARD_ERROR_ENTITY_OUTSTANDING_CASES,
  PROJECT_CARD_STATS_ACTIVE_CHATS_LABEL,
  PROJECT_CARD_STATS_NULL_PLACEHOLDER,
  PROJECT_CARD_STATS_OUTSTANDING_CASES_LABEL,
} from "@features/project-hub/constants/projectHubConstants";
import type { ProjectCardStatsProps } from "@features/project-hub/types/projectHub";

/**
 * Component to render the stats section (cases, chats, date) for the Project Card.
 *
 * @param {ProjectCardStatsProps} props - The props for the component.
 * @returns {JSX.Element} The rendered stats section.
 */
export default function ProjectCardStats({
  activeCasesCount,
  activeChatsCount,
  actionRequiredCount,
  isError,
  isLoading,
}: ProjectCardStatsProps): JSX.Element {
  return (
    <Form.CardContent sx={{ width: "100%", pb: 0 }}>
      <Box
        sx={{
          display: "flex",
          flexDirection: "column",
          gap: 1.5,
        }}
      >
        {/* Action Required cases */}
        <Box display="flex" justifyContent="space-between" alignItems="center">
          <Box
            display="flex"
            alignItems="center"
            gap={1}
            color="text.secondary"
          >
            <AlertTriangle size={16} />
            <Typography variant="body2" color="inherit">
              Action Required
            </Typography>
          </Box>
          {isLoading ? (
            <Skeleton variant="text" width={20} />
          ) : isError ? (
            <ErrorIndicator entityName="action required" />
          ) : (
            <Typography variant="body2" color="warning.main">
              {actionRequiredCount ?? PROJECT_CARD_STATS_NULL_PLACEHOLDER}
            </Typography>
          )}
        </Box>

        {/* Outstanding support cases */}
        <Box display="flex" justifyContent="space-between" alignItems="center">
          <Box
            display="flex"
            alignItems="center"
            gap={1}
            color="text.secondary"
          >
            <CircleAlert size={16} />
            <Typography variant="body2" color="inherit">
              {PROJECT_CARD_STATS_OUTSTANDING_CASES_LABEL}
            </Typography>
          </Box>
          {isLoading ? (
            <Skeleton variant="text" width={20} />
          ) : isError ? (
            <ErrorIndicator
              entityName={PROJECT_CARD_ERROR_ENTITY_OUTSTANDING_CASES}
            />
          ) : (
            <Typography variant="body2" color="primary">
              {activeCasesCount ?? PROJECT_CARD_STATS_NULL_PLACEHOLDER}
            </Typography>
          )}
        </Box>

        {/* Active Chats */}
        <Box display="flex" justifyContent="space-between" alignItems="center">
          <Box
            display="flex"
            alignItems="center"
            gap={1}
            color="text.secondary"
          >
            <MessageSquare size={16} />
            <Typography variant="body2" color="inherit">
              {PROJECT_CARD_STATS_ACTIVE_CHATS_LABEL}
            </Typography>
          </Box>
          {isLoading ? (
            <Skeleton variant="text" width={20} />
          ) : isError ? (
            <ErrorIndicator
              entityName={PROJECT_CARD_ERROR_ENTITY_ACTIVE_CHATS}
            />
          ) : (
            <Typography variant="body2" color={colors.blue[500]}>
              {activeChatsCount ?? PROJECT_CARD_STATS_NULL_PLACEHOLDER}
            </Typography>
          )}
        </Box>

        <Divider sx={{ width: "100%" }} />
      </Box>
    </Form.CardContent>
  );
}
