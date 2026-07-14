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

import { useState } from "react";
import { Stack, Tab, Tabs } from "@wso2/oxygen-ui";
import { ComingSoonPage } from "@components/common/ComingSoonPage";
import { ServiceRequestsTab } from "@components/operations/ServiceRequestsTab";
import { ChangeRequestsTab } from "@components/operations/ChangeRequestsTab";

type OperationsTabId = "service_requests" | "change_requests" | "incidents";

const TABS: { id: OperationsTabId; label: string }[] = [
  { id: "service_requests", label: "Service Requests" },
  { id: "change_requests", label: "Change Requests" },
  { id: "incidents", label: "Incidents" },
];

// Mirrors the webapp's OperationsPage (apps/csm-portal/webapp/src/features/csm-operations/pages/OperationsPage.tsx):
// three tabs — Service requests (cases with type=service_request, reusing the Support page's own
// list/pagination/filter infra), Change requests (its own list/detail/edit), and Incidents (no
// backend endpoint exists yet, in this repo or the webapp's — kept as a placeholder like the
// webapp's own IssuesListUnavailable). "Create service request"/"Create change request" are
// deliberately out of scope for this pass — each is its own large form (cascading
// project/deployment/catalog selects with dynamic variables, or a 15+ field change-request form).
export default function OperationsPage() {
  const [activeTab, setActiveTab] = useState<OperationsTabId>("service_requests");

  return (
    <Stack gap={2}>
      <Tabs variant="scrollable" value={activeTab} onChange={(_, value: OperationsTabId) => setActiveTab(value)}>
        {TABS.map((tab) => (
          <Tab key={tab.id} label={tab.label} value={tab.id} disableRipple />
        ))}
      </Tabs>

      {activeTab === "service_requests" && <ServiceRequestsTab />}
      {activeTab === "change_requests" && <ChangeRequestsTab />}
      {activeTab === "incidents" && (
        <ComingSoonPage
          title="Incidents"
          description="Incident tracking is still under construction."
          showTitle={false}
        />
      )}
    </Stack>
  );
}
