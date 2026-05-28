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
import { useDeclareLayout } from "@root/src/context/layout";
import { Stack } from "@wso2/oxygen-ui";

import {
  AnnouncementItemsList,
  CaseItemsList,
  ChangeRequestItemsList,
  ChatItemsList,
  EngagementItemsList,
  ItemCardSkeleton,
  SecurityReportAnalysisItemsList,
  ServiceRequestItemsList,
} from "@features/items/components";
import { useFilters, useItems } from "@features/items/hooks";

import { EmptyState } from "@shared/components/common";

import { CASE_TYPES } from "@shared/constants";

export function FilterContent() {
  const { state, filters } = useFilters();
  const { total, isResolving } = useItems();

  useDeclareLayout(
    {
      title: state?.title,
    },
    { enabled: Boolean(state?.title) },
  );

  if (!isResolving && total === 0) return <EmptyState />;

  return (
    <Stack gap={2}>
      {filters.types.map((type) => {
        switch (type) {
          case CASE_TYPES.DEFAULT:
            return <CaseItemsList key={type} />;
          case CASE_TYPES.CHAT:
            return <ChatItemsList key={type} />;
          case CASE_TYPES.SERVICE_REQUEST:
            return <ServiceRequestItemsList key={type} />;
          case CASE_TYPES.CHANGE_REQUEST:
            return <ChangeRequestItemsList key={type} />;
          case CASE_TYPES.SECURITY_REPORT_ANALYSIS:
            return <SecurityReportAnalysisItemsList key={type} />;
          case CASE_TYPES.ENGAGEMENT:
            return <EngagementItemsList key={type} />;
          case CASE_TYPES.ANNOUNCEMENT:
            return <AnnouncementItemsList key={type} />;
          default:
            return null;
        }
      })}
    </Stack>
  );
}

export function FilterContentSkeleton() {
  return Array.from({ length: 5 }).map((_, index) => <ItemCardSkeleton key={index} />);
}
