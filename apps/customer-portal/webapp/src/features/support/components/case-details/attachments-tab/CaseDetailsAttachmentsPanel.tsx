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

import type { CaseDetailsAttachmentsPanelProps } from "@features/support/types/supportComponents";
import { Box, Button, Stack, Typography } from "@wso2/oxygen-ui";
import ListPagination from "@components/list-view/ListPagination";
import { Paperclip } from "@wso2/oxygen-ui-icons-react";
import { useEffect, useMemo, useRef, useState, type JSX } from "react";
import { useMutationState } from "@tanstack/react-query";
import {
  useGetCaseAttachments,
  flattenCaseAttachments,
} from "@features/support/api/useGetCaseAttachments";
import type { CaseAttachment } from "@features/support/types/cases";
import type { PostAttachmentsVariables } from "@features/support/types/supportApi";
import { useDeleteAttachment } from "@features/support/api/useDeleteAttachment";
import { useGetAttachmentContent } from "@api/useGetAttachmentContent";
import { isEntityAttachmentsEnabled } from "@/config/entityAttachmentsConfig";
import {
  useStoredAttachmentDelete,
  useStoredAttachmentDownload,
} from "@api/useStoredAttachments";
import { isNotFoundError } from "@utils/ApiError";
import { useErrorBanner } from "@context/error-banner/ErrorBannerContext";
import useGetUserDetails from "@features/settings/api/useGetUserDetails";

import UploadAttachmentModal from "@case-details-attachments/UploadAttachmentModal";
import AttachmentListItem from "@case-details-attachments/AttachmentListItem";
import AttachmentsListSkeleton from "@case-details-attachments/AttachmentsListSkeleton";
import UploadingAttachmentPlaceholder from "@case-details-attachments/UploadingAttachmentPlaceholder";
import DeleteAttachmentModal from "@case-details-attachments/DeleteAttachmentModal";
import EditCaseAttachmentModal from "@case-details-attachments/EditCaseAttachmentModal";
import EmptyIcon from "@components/empty-state/EmptyIcon";
import ApiErrorState from "@components/error/ApiErrorState";


const ITEMS_PER_PAGE = 10;
const SKELETON_MIN_DISPLAY_MS = 4000;

/**
 * Renders the Attachments tab: upload button, modal, and list from GET /cases/:id/attachments.
 * Download uses GET /attachments/:id (authenticated) when inline content is absent.
 * Uses infinite query with server-side pagination (10 items per page).
 *
 * @param {CaseDetailsAttachmentsPanelProps} props - caseId.
 * @returns {JSX.Element} The attachments panel.
 */
export default function CaseDetailsAttachmentsPanel({
  caseId,
  isCaseClosed = false,
}: CaseDetailsAttachmentsPanelProps): JSX.Element {
  const { showError } = useErrorBanner();
  const { data: userDetails } = useGetUserDetails();
  const currentUserEmail = userDetails?.email?.trim().toLowerCase() ?? "";
  const { downloadAttachment, isDownloading, downloadingId } =
    useGetAttachmentContent();
  const entityAttachments = isEntityAttachmentsEnabled();
  const storedDownload = useStoredAttachmentDownload();
  const storedDelete = useStoredAttachmentDelete();
  const [uploadOpen, setUploadOpen] = useState(false);
  const [minTimeElapsed, setMinTimeElapsed] = useState(false);
  const minTimeRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(() => {
    minTimeRef.current = setTimeout(
      () => setMinTimeElapsed(true),
      SKELETON_MIN_DISPLAY_MS,
    );
    return () => {
      if (minTimeRef.current) clearTimeout(minTimeRef.current);
    };
  }, []);
  const [currentPage, setCurrentPage] = useState(1);
  const [attachmentToDelete, setAttachmentToDelete] =
    useState<CaseAttachment | null>(null);
  const [attachmentToEdit, setAttachmentToEdit] =
    useState<CaseAttachment | null>(null);

  const {
    data,
    isLoading,
    isError,
    error,
    isFetchingNextPage,
    hasNextPage,
    fetchNextPage,
    isFetchNextPageError,
  } = useGetCaseAttachments(caseId);

  const deleteAttachment = useDeleteAttachment();

  const pendingUploads = useMutationState({
    filters: { mutationKey: ["postCaseAttachment"], status: "pending" },
    select: (mutation) =>
      mutation.state.variables as PostAttachmentsVariables | undefined,
  }).filter(
    (vars): vars is PostAttachmentsVariables => vars?.caseId === caseId,
  );

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

  const handleDownload = async (att: CaseAttachment) => {
    try {
      if (entityAttachments) {
        try {
          await storedDownload.downloadStoredAttachment({
            type: "work_item",
            id: att.id,
            name: att.name,
          });
          return;
        } catch (storedError) {
          // Not held by entity-service (older attachment): use the existing download.
          if (!isNotFoundError(storedError)) throw storedError;
        }
      }
      await downloadAttachment({
        id: att.id,
        name: att.name,
        type: att.type,
        downloadUrl: att.downloadUrl,
      });
    } catch (error) {
      const message =
        error instanceof Error ? error.message : "Download failed";
      showError(message);
    }
  };

  const handleDeleteClick = (att: CaseAttachment) => {
    setAttachmentToDelete(att);
  };

  const handleConfirmDelete = () => {
    if (!attachmentToDelete?.id) return;
    const attachmentId = attachmentToDelete.id;
    const deleteOlder = () =>
      deleteAttachment.mutate(
        { attachmentId, caseId },
        {
          onSuccess: () => setAttachmentToDelete(null),
          onError: (error) => {
            showError(error.message);
          },
        },
      );
    if (!entityAttachments) {
      deleteOlder();
      return;
    }
    storedDelete.mutate(
      { type: "work_item", attachmentId, caseId },
      {
        onSuccess: () => setAttachmentToDelete(null),
        onError: (error) => {
          // Not held by entity-service (older attachment): use the existing delete.
          if (isNotFoundError(error)) {
            deleteOlder();
            return;
          }
          showError(error.message);
        },
      },
    );
  };

  const handleEditClick = (att: CaseAttachment) => {
    setAttachmentToEdit(att);
  };

  const handleEditSuccess = () => {
    setAttachmentToEdit(null);
  };

  const handleEditError = (message: string) => {
    showError(message);
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
        {!(
          allAttachments.length === 0 &&
          pendingUploads.length === 0 &&
          !isLoading &&
          minTimeElapsed &&
          !isError
        ) && <Box sx={{ alignSelf: "flex-start" }}>{uploadButton}</Box>}

        {isLoading || !minTimeElapsed ? (
          <AttachmentsListSkeleton />
        ) : isError ? (
          <ApiErrorState
            error={error}
            fallbackMessage="Failed to load attachments."
          />
        ) : allAttachments.length === 0 && pendingUploads.length === 0 ? (
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
            {pendingUploads.map((vars, index) => (
              <UploadingAttachmentPlaceholder
                key={`pending-upload-${index}`}
                name={vars.body.name}
              />
            ))}
            {isFetchingNextPage && paginatedAttachments.length === 0 ? (
              <AttachmentsListSkeleton />
            ) : (
              paginatedAttachments.map((att) => {
                const isOwner =
                  currentUserEmail &&
                  att.createdBy?.trim().toLowerCase() === currentUserEmail;
                const deleteDisabled = isCaseClosed || !isOwner;
                const deleteTooltip = isCaseClosed
                  ? undefined
                  : !isOwner
                    ? "Only the uploader can delete this attachment"
                    : undefined;
                return (
                  <AttachmentListItem
                    key={att.id}
                    attachment={att}
                    onDownload={handleDownload}
                    // With entity-service attachments on, a closed case offers no delete at all.
                    onDelete={
                      entityAttachments && isCaseClosed
                        ? undefined
                        : handleDeleteClick
                    }
                    deleteDisabled={deleteDisabled}
                    deleteTooltip={deleteTooltip}
                    onEdit={isCaseClosed || !isOwner ? undefined : handleEditClick}
                    hideDescription
                    isDownloadLoading={
                      (isDownloading && downloadingId === att.id) ||
                      (storedDownload.isDownloading &&
                        storedDownload.downloadingId === att.id)
                    }
                  />
                );
              })
            )}
            <ListPagination
              totalRecords={totalRecords}
              page={boundedPage}
              rowsPerPage={ITEMS_PER_PAGE}
              onPageChange={handlePageChange}
              onRowsPerPageChange={() => {}}
              rowsPerPageOptions={[ITEMS_PER_PAGE]}
            />
          </Stack>
        )}
      </Stack>

      <UploadAttachmentModal
        open={uploadOpen}
        caseId={caseId}
        onClose={() => setUploadOpen(false)}
      />

      <DeleteAttachmentModal
        open={!!attachmentToDelete}
        attachmentName={attachmentToDelete?.name ?? null}
        onClose={() => setAttachmentToDelete(null)}
        onConfirm={handleConfirmDelete}
        isDeleting={
          !!attachmentToDelete &&
          (deleteAttachment.isPending || storedDelete.isPending)
        }
      />

      <EditCaseAttachmentModal
        open={!!attachmentToEdit}
        attachment={attachmentToEdit}
        caseId={caseId}
        onClose={() => setAttachmentToEdit(null)}
        onSuccess={handleEditSuccess}
        onError={handleEditError}
      />
    </>
  );
}
