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

import { Alert, Box, Button, Stack, Typography } from "@wso2/oxygen-ui";
import { Paperclip } from "@wso2/oxygen-ui-icons-react";
import { useMemo, useState, type JSX } from "react";
import useGetCaseAttachments from "@api/useGetCaseAttachments";
import type { CaseAttachment } from "@models/responses";
import { CASE_ATTACHMENTS_INITIAL_LIMIT } from "@constants/supportConstants";
import UploadAttachmentModal from "@case-details-attachments/UploadAttachmentModal";
import AttachmentListItem from "@case-details-attachments/AttachmentListItem";
import AttachmentsListSkeleton from "@case-details-attachments/AttachmentsListSkeleton";
import EmptyIcon from "@components/common/empty-state/EmptyIcon";

export interface CaseDetailsAttachmentsPanelProps {
  caseId: string;
}

/**
 * Renders the Attachments tab: upload button, modal, and list from GET /cases/:id/attachments.
 * Fetches first page, then remaining when totalRecords > limit; shows skeleton until all loaded.
 *
 * @param {CaseDetailsAttachmentsPanelProps} props - caseId.
 * @returns {JSX.Element} The attachments panel.
 */
export default function CaseDetailsAttachmentsPanel({
  caseId,
}: CaseDetailsAttachmentsPanelProps): JSX.Element {
  const [uploadOpen, setUploadOpen] = useState(false);

  const first = useGetCaseAttachments(caseId, {
    limit: CASE_ATTACHMENTS_INITIAL_LIMIT,
    offset: 0,
  });
  const totalRecords = first.data?.totalRecords ?? 0;
  const needMore = totalRecords > CASE_ATTACHMENTS_INITIAL_LIMIT;
  const secondEnabled =
    !!caseId &&
    needMore &&
    !!first.data &&
    totalRecords > CASE_ATTACHMENTS_INITIAL_LIMIT;
  const second = useGetCaseAttachments(caseId, {
    limit: Math.max(0, totalRecords - CASE_ATTACHMENTS_INITIAL_LIMIT),
    offset: CASE_ATTACHMENTS_INITIAL_LIMIT,
    enabled: secondEnabled,
  });

  const allAttachments = useMemo(() => {
    if (!first.data) return [];
    if (!needMore) return first.data.attachments;
    if (!second.data) return first.data.attachments;
    return [...first.data.attachments, ...second.data.attachments];
  }, [first.data, needMore, second.data]);

  const combinedLength =
    (first.data?.attachments?.length ?? 0) +
    (second.data?.attachments?.length ?? 0);

  const hasPartialError =
    secondEnabled &&
    (second.isError ||
      (first.data != null &&
        totalRecords > 0 &&
        first.data.totalRecords !== combinedLength));

  const isLoading =
    first.isLoading ||
    (secondEnabled && !second.isError && (second.isLoading || !second.data));

  const handleDownload = (att: CaseAttachment) => {
    if (att.downloadUrl) {
      window.open(att.downloadUrl, "_blank", "noopener,noreferrer");
    }
  };

  if (!caseId) {
    return (
      <Typography variant="body2" color="text.secondary">
        No case selected.
      </Typography>
    );
  }

  return (
    <>
      <Stack spacing={3}>
        <Button
          variant="contained"
          color="primary"
          startIcon={<Paperclip size={16} aria-hidden />}
          sx={{ alignSelf: "flex-start" }}
          onClick={() => setUploadOpen(true)}
        >
          Upload Attachment
        </Button>

        {isLoading ? (
          <AttachmentsListSkeleton />
        ) : first.isError ? (
          <Typography variant="body2" color="error">
            Failed to load attachments.
          </Typography>
        ) : allAttachments.length === 0 ? (
          <Stack
            spacing={2}
            alignItems="center"
            justifyContent="center"
            sx={{ py: 4 }}
          >
            <Box
              sx={{
                width: 160,
                maxWidth: "100%",
                "& svg": { width: "100%", height: "auto" },
              }}
              aria-hidden
            >
              <EmptyIcon />
            </Box>
            <Typography variant="body2" color="text.secondary">
              No attachments found.
            </Typography>
          </Stack>
        ) : (
          <Stack spacing={2}>
            {hasPartialError && (
              <Alert severity="warning" sx={{ mb: 1 }}>
                Some attachments may not be shown. The list may be incomplete
                due to a load error or API limits.
              </Alert>
            )}
            {allAttachments.map((att) => (
              <AttachmentListItem
                key={att.id}
                attachment={att}
                onDownload={handleDownload}
              />
            ))}
          </Stack>
        )}
      </Stack>

      <UploadAttachmentModal
        open={uploadOpen}
        caseId={caseId}
        onClose={() => setUploadOpen(false)}
      />
    </>
  );
}
