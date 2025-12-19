// Copyright (c) 2025 WSO2 LLC. (https://www.wso2.com).
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

import { Box } from "@mui/material";
import React, { useEffect } from "react";
import { useParams } from "react-router-dom";
import {
  ChangeRequestCard,
  ServiceRequestCard,
  SupportHeader,
  SupportStatsRow,
  OngoingCasesList,
  NewChatCTA,
} from "@/components/Support";
import { Endpoints } from "@/services/endpoints";
import { useGet } from "@/services/useApi";
import { useProject } from "@/context/ProjectContext";
import type { CaseResponse } from "@/types/support.types";
import type { ProjectMetadataResponse } from "@/types/project-metadata.types";
import PreLoader from "@/components/PreLoader/PreLoader";

const SupportPage: React.FC = () => {
  const { sysId } = useParams<{ sysId: string }>();
  const { currentProject } = useProject();

  const projectName = currentProject?.name || "";

  const { data, isLoading, error } = useGet<CaseResponse>(
    ["getAllCases", sysId],
    Endpoints.getAllCases(sysId || "", 0, 5),
    {
      enabled: !!sysId,
    }
  );

  const { data: projectMetadata } = useGet<ProjectMetadataResponse>(
    ["project-metadata", sysId],
    Endpoints.getProjectMetaData(sysId || ""),
    {
      enabled: !!sysId,
    }
  );

  const statsData = projectMetadata
    ? {
        totalCasesCount: projectMetadata.projectStatistics.totalCasesCount,
        ongoingCasesCount: projectMetadata.projectStatistics.openCasesCount,
        awaitingCasesCount:
          projectMetadata.projectStatistics.activeCaseCount.awaitingCount,
        inProgressCasesCount:
          projectMetadata.projectStatistics.inProgressCasesCount,
        resolvedCasesCount:
          projectMetadata.projectStatistics.resolvedCasesCount,
      }
    : undefined;

  useEffect(() => {
    window.scrollTo(0, 0);
  }, []);

  if (isLoading) {
    return <PreLoader isLoading={isLoading} />;
  }

  if (error) {
    return "error";
  }

  const ongoingCases = data?.cases;

  return (
    <>
      <SupportHeader projectName={projectName} />

      <Box sx={{ px: 4, py: 3 }}>
        {statsData && <SupportStatsRow stats={statsData} />}

        <NewChatCTA />

        <Box
          sx={{
            display: "grid",
            gridTemplateColumns: { xs: "1fr", lg: "1fr 1fr" },
            gap: 3,
            mb: 3,
          }}
        >
          {ongoingCases && <OngoingCasesList cases={ongoingCases} />}
        </Box>

        <Box
          sx={{
            display: "grid",
            gridTemplateColumns: { xs: "1fr", lg: "1fr 1fr" },
            gap: 3,
          }}
        >
          <ServiceRequestCard />
          <ChangeRequestCard />
        </Box>
      </Box>
    </>
  );
};

export default SupportPage;
