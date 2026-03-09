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

import { Box, Button, Pagination, Stack, Typography } from "@wso2/oxygen-ui";
import { Paperclip } from "@wso2/oxygen-ui-icons-react";
import { useEffect, useMemo, useState, type JSX } from "react";
import {
  useGetCaseAttachments,
  flattenCaseAttachments,
} from "@api/useGetCaseAttachments";
import type { CaseAttachment } from "@models/responses";
import UploadAttachmentModal from "@case-details-attachments/UploadAttachmentModal";
import AttachmentListItem from "@case-details-attachments/AttachmentListItem";
import AttachmentsListSkeleton from "@case-details-attachments/AttachmentsListSkeleton";
import EmptyIcon from "@components/common/empty-state/EmptyIcon";

const ITEMS_PER_PAGE = 10;

export interface CaseDetailsAttachmentsPanelProps {
  caseId: string;
  isCaseClosed?: boolean;
}

/**
 * Renders the Attachments tab: upload button, modal, and list from GET /cases/:id/attachments.
 * Uses infinite query with server-side pagination (10 items per page).
 *
 * @param {CaseDetailsAttachmentsPanelProps} props - caseId.
 * @returns {JSX.Element} The attachments panel.
 */
export default function CaseDetailsAttachmentsPanel({
  caseId,
  isCaseClosed = false,
}: CaseDetailsAttachmentsPanelProps): JSX.Element {
  const [uploadOpen, setUploadOpen] = useState(false);
  const [currentPage, setCurrentPage] = useState(1);

  const {
    data,
    isLoading,
    isError,
    isFetchingNextPage,
    hasNextPage,
    fetchNextPage,
    isFetchNextPageError,
  } = useGetCaseAttachments(caseId);

  const allAttachments = useMemo(() => flattenCaseAttachments(data), [data]);

  const totalRecords = data?.pages?.[0]?.totalRecords ?? 0;
  const totalPages = Math.ceil(totalRecords / ITEMS_PER_PAGE);

  // Ensure current page is within bounds
  const boundedPage =
    totalPages > 0 && currentPage > totalPages ? 1 : currentPage;

  const paginatedAttachments = useMemo(() => {
    const startIndex = (boundedPage - 1) * ITEMS_PER_PAGE;
    const endIndex = startIndex + ITEMS_PER_PAGE;
    return allAttachments.slice(startIndex, endIndex);
  }, [allAttachments, boundedPage]);

  const handlePageChange = (
    _event: React.ChangeEvent<unknown>,
    page: number,
  ) => {
    setCurrentPage(page);
  };

  // Auto-fetch next page if we need more data for the current page
  useEffect(() => {
    const neededItemsCount = boundedPage * ITEMS_PER_PAGE;
    if (
      !isLoading &&
      !isFetchingNextPage &&
      !isFetchNextPageError &&
      hasNextPage &&
      allAttachments.length < neededItemsCount &&
      allAttachments.length < totalRecords
    ) {
      fetchNextPage();
    }
  }, [
    boundedPage,
    allAttachments.length,
    totalRecords,
    hasNextPage,
    fetchNextPage,
    isLoading,
    isFetchingNextPage,
    isFetchNextPageError,
  ]);

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

  const uploadButton = (
    <Button
      variant="contained"
      color="primary"
      startIcon={<Paperclip size={16} aria-hidden />}
      onClick={() => setUploadOpen(true)}
      disabled={isCaseClosed}
    >
      Upload Attachment
    </Button>
  );

  return (
    <>
      <Stack spacing={3}>
        {!(allAttachments.length === 0 && !isLoading && !isError) && (
          <Box sx={{ alignSelf: "flex-start" }}>{uploadButton}</Box>
        )}

        {isLoading ? (
          <AttachmentsListSkeleton />
        ) : isError ? (
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
            <Box sx={{ display: "flex", justifyContent: "center", mt: 1 }}>
              {uploadButton}
            </Box>
          </Stack>
        ) : (
          <Stack spacing={2}>
            {isFetchingNextPage && paginatedAttachments.length === 0 ? (
              <AttachmentsListSkeleton />
            ) : (
              paginatedAttachments.map((att) => (
                <AttachmentListItem
                  key={att.id}
                  attachment={att}
                  onDownload={handleDownload}
                />
              ))
            )}
            {totalPages > 1 && (
              <Box sx={{ display: "flex", justifyContent: "center", mt: 2 }}>
                <Pagination
                  count={totalPages}
                  page={boundedPage}
                  onChange={handlePageChange}
                  color="primary"
                  showFirstButton
                  showLastButton
                />
              </Box>
            )}
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
