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

import { Box, Card, CardContent } from "@wso2/oxygen-ui";
import type { JSX } from "react";

import type { ProjectDetails } from "@models/responses";
import { formatProjectDate } from "@utils/projectStats";
import ProjectHeader from "@components/project-details/project-overview/project-information/ProjectHeader";
import ProjectName from "@components/project-details/project-overview/project-information/ProjectName";
import ProjectDescription from "@components/project-details/project-overview/project-information/ProjectDescription";
import ProjectMetadata from "@components/project-details/project-overview/project-information/ProjectMetadata";
import SubscriptionDetails from "@components/project-details/project-overview/project-information/SubscriptionDetails";

interface ProjectInformationCardProps {
  project?: ProjectDetails;
  slaStatus: string;
  isLoading?: boolean;
  isError?: boolean;
}

const ProjectInformationCard = ({
  project,
  slaStatus,
  isLoading,
  isError,
}: ProjectInformationCardProps): JSX.Element => {
  const getKey = () => project?.key || "--";
  const getName = () => project?.name || "--";
  const getDescription = () => project?.description || "--";
  const getCreatedDate = () =>
    project?.createdOn ? formatProjectDate(project.createdOn) : "--";
  const getType = () => project?.type || "--";
  const getSupportTier = () => project?.subscription?.supportTier || "--";
  const getStartDate = () =>
    project?.subscription?.startDate
      ? formatProjectDate(project.subscription.startDate)
      : "--";
  const getEndDate = () =>
    project?.subscription?.endDate
      ? formatProjectDate(project.subscription.endDate)
      : "--";

  return (
    <Card sx={{ height: "100%" }}>
      <CardContent sx={{ p: 3 }}>
        <ProjectHeader />

        <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
          <ProjectName
            name={getName()}
            projectKey={getKey()}
            isLoading={isLoading}
            isError={isError}
          />

          <ProjectDescription
            description={getDescription()}
            isLoading={isLoading}
            isError={isError}
          />

          <ProjectMetadata
            createdDate={getCreatedDate()}
            projectType={getType()}
            supportTier={getSupportTier()}
            slaStatus={slaStatus}
            isLoading={isLoading}
            isError={isError}
          />

          <SubscriptionDetails
            startDate={getStartDate()}
            endDate={getEndDate()}
            isLoading={isLoading}
            isError={isError}
          />
        </Box>
      </CardContent>
    </Card>
  );
};

export default ProjectInformationCard;
