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

import { Box, Typography } from "@wso2/oxygen-ui";
import { useParams } from "react-router";
import type { JSX } from "react";

interface ProjectPageProps {
  title: string;
}

/**
 * ProjectPage component.
 *
 * @param {ProjectPageProps} props - The props for the component.
 * @returns {JSX.Element} The ProjectPage component.
 */
export default function ProjectPage({ title }: ProjectPageProps): JSX.Element {
  const { projectId } = useParams<{ projectId: string }>();

  return (
    <Box>
      {/* project page title */}
      <Typography variant="h4" gutterBottom>
        {title}
      </Typography>
      {/* project page subtitle */}
      <Typography variant="body1" color="text.secondary">
        Displaying content for Project: <strong>{projectId}</strong>
      </Typography>
    </Box>
  );
}
