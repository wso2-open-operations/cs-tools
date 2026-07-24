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
import { AccountsTab } from "@components/customers/AccountsTab";
import { ProjectsTab } from "@components/customers/ProjectsTab";

type CustomersTabId = "accounts" | "projects";

const TABS: { id: CustomersTabId; label: string }[] = [
  { id: "accounts", label: "Accounts" },
  { id: "projects", label: "Projects" },
];

// Mirrors the webapp's CsmCustomersLayout: a thin tab shell (Accounts | Projects) over two
// pre-existing, fully-built search pages — read-only browse only for this pass, no
// create/edit/deactivate (that's several more dialogs with cascading pickers, deliberately
// deferred, same call the webapp's own Operations "Create SR/CR" and this app's Engagements made).
export default function CustomersPage() {
  const [activeTab, setActiveTab] = useState<CustomersTabId>("accounts");

  return (
    <Stack gap={2}>
      <Tabs variant="scrollable" value={activeTab} onChange={(_, value: CustomersTabId) => setActiveTab(value)}>
        {TABS.map((tab) => (
          <Tab key={tab.id} label={tab.label} value={tab.id} disableRipple />
        ))}
      </Tabs>

      {activeTab === "accounts" && <AccountsTab />}
      {activeTab === "projects" && <ProjectsTab />}
    </Stack>
  );
}
