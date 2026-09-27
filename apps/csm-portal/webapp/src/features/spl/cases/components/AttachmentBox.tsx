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

// Ported from apps/support-portal-lite/webapp's own
// features/spl/cases/components/AttachmentBox.tsx — rewritten against
// useGetSplCaseAttachments (React Query) instead of useSplApi's useGetApi.
import { Box, Stack, Tooltip, Typography } from "@wso2/oxygen-ui";
import { CircleAlertIcon, ClockIcon } from "@wso2/oxygen-ui-icons-react";
import { useSplPermissions } from "@features/spl/api/splPermissionsContext";
import { useAttachmentDownload } from "../api/useAttachmentDownload";
import { useGetSplCaseAttachments } from "../api/useSplCases";
import type { AttachmentDetails } from "../api/splCaseTypes";
import { LinearLoadingPanel } from "./StatePanels";

export function AttachmentBox({ caseId }: { caseId: string | undefined }) {
  const { data, isLoading, error } = useGetSplCaseAttachments(caseId ?? "");
  const { canDownloadAttachments } = useSplPermissions();
  const { downloadAttachment } = useAttachmentDownload();

  if (isLoading) return <LinearLoadingPanel />;
  if (error) {
    return (
      <Stack direction="row" alignItems="center" spacing={1} sx={{ color: "error.main" }}>
        <CircleAlertIcon size={20} />
        <Typography color="error" variant="body1">
          error while loading attachments!
        </Typography>
      </Stack>
    );
  }
  const attachments = (data ?? []) as AttachmentDetails[];
  if (attachments.length === 0) {
    return (
      <Typography variant="body2" color="text.secondary">
        No attachments
      </Typography>
    );
  }

  return (
    <Stack spacing={1}>
      {attachments.map((item, index) =>
        canDownloadAttachments ? (
          <Box
            key={index}
            sx={{ cursor: "pointer", p: 1, borderRadius: 1, "&:hover": { backgroundColor: "action.hover" } }}
            onClick={() => downloadAttachment(item.sysId, item.fileName)}
          >
            <Typography variant="body2" fontWeight={600}>
              {item.fileName}
            </Typography>
            <Stack direction="row" spacing={0.5} alignItems="center" sx={{ color: "text.secondary" }}>
              <ClockIcon size={14} />
              <Typography variant="caption">{new Date(item.createdOn + "Z").toLocaleString()}</Typography>
            </Stack>
          </Box>
        ) : (
          <Tooltip key={index} title="You don't have permission to download this file">
            <Box sx={{ p: 1, opacity: 0.6 }}>
              <Typography variant="body2">{item.fileName}</Typography>
              <Stack direction="row" spacing={0.5} alignItems="center" sx={{ color: "text.secondary" }}>
                <ClockIcon size={14} />
                <Typography variant="caption">{new Date(item.createdOn + "Z").toLocaleString()}</Typography>
              </Stack>
            </Box>
          </Tooltip>
        ),
      )}
    </Stack>
  );
}
