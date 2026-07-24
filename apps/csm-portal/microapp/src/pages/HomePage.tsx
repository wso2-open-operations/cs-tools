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

import { Stack } from "@wso2/oxygen-ui";
import { AssignedToMeSection } from "@components/home/AssignedToMeSection";
import { CaseCompositionSection } from "@components/home/CaseCompositionSection";

// Mirrors the webapp's Dashboard (apps/csm-portal/webapp/src/features/csm-dashboard/pages/CsmDashboardPage.tsx):
// "Assigned to me" and the case composition donuts, its two widgets with real data. The
// severity×state matrix table is deliberately left out — it costs 25 extra fan-out requests on
// top of the donuts' 11, with no aggregation endpoint to make it cheap, and doesn't fit a mobile
// screen well anyway. The ABT-scope header is skipped too since it's feature-flagged off
// (CSM_DASHBOARD_API_IMPLEMENTED = false) and only ever shows a static fallback in the webapp.
export default function HomePage() {
  return (
    <Stack gap={3}>
      <AssignedToMeSection />
      <CaseCompositionSection />
    </Stack>
  );
}
