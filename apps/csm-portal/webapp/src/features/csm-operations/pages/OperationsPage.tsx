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

import { Box, Button, Tab, Tabs, Typography } from "@wso2/oxygen-ui";
import { Plus } from "@wso2/oxygen-ui-icons-react";
import { useState, type JSX } from "react";
import { useNavigate } from "react-router";
import CsmIssuesView from "@features/csm-cases/components/CsmIssuesView";
import IssuesListUnavailable from "@features/csm-operations/components/IssuesListUnavailable";

type OperationsTabId = "service_requests" | "change_requests" | "incidents";

/**
 * Operations landing — the home for the managed-cloud operational entities,
 * split into Service Requests / Change Requests / Incidents tabs. Service
 * requests are case-typed, so they list through the shared issues view; CR and
 * Incidents have no backend search endpoint yet and render an unavailable
 * placeholder.
 */
export default function OperationsPage(): JSX.Element {
  const navigate = useNavigate();
  const [activeTab, setActiveTab] = useState<OperationsTabId>("service_requests");

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
      <Box>
        <Typography variant="h5">Operations</Typography>
        <Typography variant="body2" color="text.secondary">
          Service requests, change requests, and incidents across customers.
        </Typography>
      </Box>

      <Box sx={{ borderBottom: 1, borderColor: "divider" }}>
        <Tabs
          value={activeTab}
          onChange={(_, v) => setActiveTab(v as OperationsTabId)}
        >
          <Tab value="service_requests" label="Service requests" />
          <Tab value="change_requests" label="Change requests" />
          <Tab value="incidents" label="Incidents" />
        </Tabs>
      </Box>

      {activeTab === "service_requests" && (
        <CsmIssuesView
          entityNoun="service requests"
          lockedFilters={{ caseTypes: ["service_request"] }}
          hideTypeFilter
          actions={
            <Button
              variant="contained"
              color="primary"
              size="small"
              startIcon={<Plus size={16} />}
              onClick={() => navigate("/operations/service-requests/new")}
            >
              Create service request
            </Button>
          }
        />
      )}

      {activeTab === "change_requests" && (
        <IssuesListUnavailable
          title="Change requests"
          description="Controlled changes, linked to service requests, with peer / CAB approval."
        />
      )}

      {activeTab === "incidents" && (
        <IssuesListUnavailable
          title="Incidents"
          description="SaaS incidents raised by CRE or automation; may link to a case."
        />
      )}
    </Box>
  );
}
