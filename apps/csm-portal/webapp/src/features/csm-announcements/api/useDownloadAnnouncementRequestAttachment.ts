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

import { useCallback } from "react";
import { useBackendApi } from "@api/backend/client";
import { saveBlob } from "@utils/saveBlob";
import type { AnnouncementRequestAttachment } from "@features/csm-announcements/types/announcementRequests";

/**
 * Returns a function that downloads one of a request's attached files
 * (`GET /announcement-requests/{id}/attachments/{attachmentId}/content`). The
 * content is fetched as an authenticated blob (a plain link would miss the auth
 * headers) and saved under the file's own name. The route needs the permission
 * the case attachment download asks for, so a caller without it gets the
 * backend's 403 as a rejection, which the caller reports.
 */
export function useDownloadAnnouncementRequestAttachment(): (
  requestId: string,
  attachment: AnnouncementRequestAttachment,
) => Promise<void> {
  const api = useBackendApi();
  return useCallback(
    async (requestId: string, attachment: AnnouncementRequestAttachment): Promise<void> => {
      const blob = await api.getBlob(
        `/announcement-requests/${encodeURIComponent(requestId)}/attachments/${encodeURIComponent(attachment.id)}/content`,
      );
      saveBlob(blob, attachment.name);
    },
    [api],
  );
}
