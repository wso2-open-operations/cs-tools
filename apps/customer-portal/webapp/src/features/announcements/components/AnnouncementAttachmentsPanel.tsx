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
import { Box, Button, Paper, Stack, Typography } from "@wso2/oxygen-ui";
import type { JSX } from "react";
import AttachmentListItem from "@case-details-attachments/AttachmentListItem";
import {
  flattenCaseAttachments,
  useGetCaseAttachments,
} from "@features/support/api/useGetCaseAttachments";
import { useGetAttachmentContent } from "@api/useGetAttachmentContent";
import { useErrorBanner } from "@context/error-banner/ErrorBannerContext";
import type { CaseAttachment } from "@features/support/types/cases";
import {
  ANNOUNCEMENT_DETAILS_ATTACHMENTS_ERROR,
  ANNOUNCEMENT_DETAILS_ATTACHMENTS_HEADING,
} from "@features/announcements/constants/announcementsConstants";

interface AnnouncementAttachmentsPanelProps {
  caseId: string;
}

/**
 * The files sent with an announcement (a PDF above all), listed on its details page
 * and downloadable by the customers who can see the announcement. They are ordinary
 * case attachments, so this reads the same endpoint as any case's Attachments tab
 * and offers nothing to change them: an announcement's files are set by WSO2 when it
 * is sent, and customers cannot upload to or delete from one.
 *
 * Renders nothing while loading and for an announcement with no files, so the many
 * announcements without any look exactly as they did. A failed load is shown rather
 * than hidden, since an announcement that does carry a document must not read as if
 * it did not.
 *
 * @param props - The announcement's case id.
 * @returns The attachments section, or null when there is nothing to show.
 */
export default function AnnouncementAttachmentsPanel({
  caseId,
}: AnnouncementAttachmentsPanelProps): JSX.Element | null {
  const { showError } = useErrorBanner();
  const { downloadAttachment, isDownloading, downloadingId } =
    useGetAttachmentContent();
  const {
    data,
    isLoading,
    isError,
    refetch,
    hasNextPage,
    fetchNextPage,
    isFetchingNextPage,
  } = useGetCaseAttachments(caseId);

  const attachments = flattenCaseAttachments(data);

  const handleDownload = async (attachment: CaseAttachment): Promise<void> => {
    try {
      await downloadAttachment({
        id: attachment.id,
        name: attachment.name,
        type: attachment.type,
        downloadUrl: attachment.downloadUrl,
      });
    } catch (error) {
      showError(error instanceof Error ? error.message : "Download failed");
    }
  };

  if (isLoading) return null;

  if (isError) {
    return (
      <Paper
        variant="outlined"
        elevation={0}
        sx={{ p: 4, display: "flex", flexDirection: "column", gap: 2 }}
      >
        <Typography variant="h6" color="text.primary">
          {ANNOUNCEMENT_DETAILS_ATTACHMENTS_HEADING}
        </Typography>
        <Box sx={{ display: "flex", alignItems: "center", gap: 2, flexWrap: "wrap" }}>
          <Typography variant="body2" color="error">
            {ANNOUNCEMENT_DETAILS_ATTACHMENTS_ERROR}
          </Typography>
          <Button size="small" variant="outlined" onClick={() => void refetch()}>
            Retry
          </Button>
        </Box>
      </Paper>
    );
  }

  if (attachments.length === 0) return null;

  return (
    <Paper
      variant="outlined"
      elevation={0}
      sx={{ p: 4, display: "flex", flexDirection: "column", gap: 2 }}
    >
      <Typography variant="h6" color="text.primary">
        {ANNOUNCEMENT_DETAILS_ATTACHMENTS_HEADING}
      </Typography>
      <Stack spacing={2}>
        {attachments.map((attachment) => (
          <AttachmentListItem
            key={attachment.id}
            attachment={attachment}
            onDownload={handleDownload}
            hideDescription
            isDownloadLoading={isDownloading && downloadingId === attachment.id}
          />
        ))}
      </Stack>
      {hasNextPage && (
        <Box sx={{ alignSelf: "flex-start" }}>
          <Button
            size="small"
            variant="text"
            disabled={isFetchingNextPage}
            onClick={() => void fetchNextPage()}
          >
            Show more
          </Button>
        </Box>
      )}
    </Paper>
  );
}
