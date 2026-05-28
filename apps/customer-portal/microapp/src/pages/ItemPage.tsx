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
import {
  AnnouncementItemView,
  CaseItemView,
  ChangeRequestItemView,
  ChatItemView,
  EngagementItemView,
  SecurityReportAnalysisItemView,
  ServiceRequestItemView,
} from "@features/detail/components";

import { CASE_TYPES } from "@shared/constants";
import type { CaseType } from "@shared/types";

export default function ItemPage({ type }: { type: CaseType }) {
  switch (type) {
    case CASE_TYPES.DEFAULT:
      return <CaseItemView />;
    case CASE_TYPES.CHAT:
      return <ChatItemView />;
    case CASE_TYPES.SERVICE_REQUEST:
      return <ServiceRequestItemView />;
    case CASE_TYPES.CHANGE_REQUEST:
      return <ChangeRequestItemView />;
    case CASE_TYPES.SECURITY_REPORT_ANALYSIS:
      return <SecurityReportAnalysisItemView />;
    case CASE_TYPES.ENGAGEMENT:
      return <EngagementItemView />;
    case CASE_TYPES.ANNOUNCEMENT:
      return <AnnouncementItemView />;
    default:
      return null;
  }
}
