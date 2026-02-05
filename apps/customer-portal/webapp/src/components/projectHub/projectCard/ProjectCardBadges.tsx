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

import { Box, Chip, Form, Skeleton } from "@wso2/oxygen-ui";
import { type JSX } from "react";
import { getStatusColor } from "@/utils/projectCard";
import ErrorIndicator from "@/components/common/errorIndicator/ErrorIndicator";

interface ProjectCardBadgesProps {
  projectKey: string;
  status: string;
  isError?: boolean;
  isLoading?: boolean;
}

/**
 * Component to render the top badges for the Project Card.
 *
 * @param {ProjectCardBadgesProps} props - The props for the component.
 * @returns {JSX.Element} The rendered badges.
 */
export default function ProjectCardBadges({
  projectKey,
  status,
  isError,
  isLoading,
}: ProjectCardBadgesProps): JSX.Element {
  return (
    <Form.CardContent sx={{ width: "100%", pt: 2, pb: 0 }}>
      <Box display="flex" justifyContent="space-between" alignItems="center">
        <Chip label={projectKey} variant="outlined" size="small" />
        {isLoading ? (
          <Skeleton
            variant="rectangular"
            width={80}
            height={24}
            sx={{ borderRadius: 1 }}
          />
        ) : isError ? (
          <ErrorIndicator entityName="Status" />
        ) : (
          <Chip
            label={status}
            variant="outlined"
            size="small"
            color={getStatusColor(status)}
          />
        )}
      </Box>
    </Form.CardContent>
  );
}
