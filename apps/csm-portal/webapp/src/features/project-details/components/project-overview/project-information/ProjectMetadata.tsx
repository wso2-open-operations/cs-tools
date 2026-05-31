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

import { Box, Grid } from "@wso2/oxygen-ui";
import type { JSX } from "react";
import ProjectMetadataPrimaryRow from "@features/project-details/components/project-overview/project-information/ProjectMetadataPrimaryRow";
import ProjectMetadataSecondaryRow from "@features/project-details/components/project-overview/project-information/ProjectMetadataSecondaryRow";
import type { ProjectMetadataProps } from "@features/project-details/types/projectDetailsComponents";

/**
 * Subscription and status metadata grid for the project overview card.
 *
 * @param props - Dates, chips, and loading flags.
 * @returns {JSX.Element} Bordered metadata section.
 */
const ProjectMetadata = ({
  createdDate,
  type,
  supportTier,
  slaStatus,
  goLivePlanDate,
  onboardingStatus,
  hideOnboardingStatus,
  isLoading,
  isError,
}: ProjectMetadataProps): JSX.Element => {
  return (
    <Box sx={{ pt: 3, borderTop: 1, borderColor: "divider" }}>
      <Grid container spacing={2} sx={{ alignItems: "center" }}>
        <ProjectMetadataPrimaryRow
          createdDate={createdDate}
          type={type}
          supportTier={supportTier}
          isLoading={isLoading}
          isError={isError}
        />
        <ProjectMetadataSecondaryRow
          slaStatus={slaStatus}
          goLivePlanDate={goLivePlanDate}
          onboardingStatus={onboardingStatus}
          hideOnboardingStatus={hideOnboardingStatus}
          isLoading={isLoading}
          isError={isError}
        />
      </Grid>
    </Box>
  );
};

export default ProjectMetadata;
