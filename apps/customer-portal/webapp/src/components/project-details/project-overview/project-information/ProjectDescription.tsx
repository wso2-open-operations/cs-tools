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

import { Box, Typography, Skeleton } from "@wso2/oxygen-ui";
import type { JSX } from "react";
import ErrorIndicator from "@components/common/error-indicator/ErrorIndicator";
import { stripHtmlTags } from "@utils/projectCard";

interface ProjectDescriptionProps {
  description: string;
  isLoading?: boolean;
  isError?: boolean;
}

const ProjectDescription = ({
  description,
  isLoading,
  isError,
}: ProjectDescriptionProps): JSX.Element => {
  return (
    <Box>
      <Typography
        variant="body2"
        fontWeight="medium"
        sx={{ display: "block", mb: 0.5 }}
      >
        Description
      </Typography>
      {isLoading ? (
        <Skeleton variant="text" width="100%" height={60} />
      ) : isError ? (
        <ErrorIndicator entityName="project description" />
      ) : (
        <Typography variant="body2" color="text.secondary">
          {stripHtmlTags(description)}
        </Typography>
      )}
    </Box>
  );
};

export default ProjectDescription;
