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

import { useCallback, useMemo, useState } from "react";
import type { BeReferenceType } from "@api/backend/types";
import { useGetCsmDeletedCaseAttachments } from "@features/csm-cases/api/useCsmCaseAttachments";
import type { DeletedAttachmentsTab } from "@features/csm-cases/components/CaseDetailWidgets";

/**
 * What `AttachmentsWidget` needs for its admin-only "Deleted" tab: `undefined`
 * for anyone else (no tab), otherwise the deleted attachments of the work item.
 * They are fetched on their own, and only once the admin first opens the tab,
 * so the ordinary attachment list and a non-admin's page never pay for them.
 */
export function useDeletedAttachmentsTab(
  workItemId: string | undefined,
  referenceType: BeReferenceType,
  isAdmin: boolean,
): DeletedAttachmentsTab | undefined {
  const [opened, setOpened] = useState(false);
  const { data, isLoading, isError, refetch } = useGetCsmDeletedCaseAttachments(
    workItemId,
    referenceType,
    isAdmin && opened,
  );
  const onOpen = useCallback(() => setOpened(true), []);
  const onRetry = useCallback(() => void refetch(), [refetch]);

  return useMemo(
    () =>
      isAdmin
        ? { attachments: data, loading: opened && isLoading, error: isError, onRetry, onOpen }
        : undefined,
    [isAdmin, data, opened, isLoading, isError, onRetry, onOpen],
  );
}
