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

// Direct visits to the SLA report route. The project button itself does not
// navigate here; it stays disabled.
import { Box, Button } from "@wso2/oxygen-ui";
import { ArrowLeft } from "@wso2/oxygen-ui-icons-react";
import type { JSX } from "react";
import { useParams } from "react-router";
import CsmComingSoonPage from "@features/csm-coming-soon/pages/CsmComingSoonPage";
import { useNavTransition } from "@hooks/useNavTransition";

export default function SlaReportUpcomingPage(): JSX.Element {
  const { projectId } = useParams<{ projectId: string }>();
  const navigate = useNavTransition();

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 2.5 }}>
      <Button
        variant="text"
        size="small"
        startIcon={<ArrowLeft size={16} />}
        onClick={() => navigate(projectId ? `/customers/projects/${projectId}` : "/customers/projects")}
        sx={{ alignSelf: "flex-start" }}
      >
        Back
      </Button>
      <CsmComingSoonPage
        title="SLA report"
        description="The SLA report isn't available in the CSM portal yet."
      />
    </Box>
  );
}
