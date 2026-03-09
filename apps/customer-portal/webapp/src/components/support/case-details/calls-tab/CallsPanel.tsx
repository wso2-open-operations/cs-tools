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

import { Box, Button, Stack } from "@wso2/oxygen-ui";
import { PhoneCall } from "@wso2/oxygen-ui-icons-react";
import { useState, useCallback, useEffect, type JSX } from "react";
import type { CallRequest } from "@models/responses";
import { useGetCallRequests } from "@api/useGetCallRequests";
import { usePatchCallRequest } from "@api/usePatchCallRequest";
import CallsListSkeleton from "@case-details-calls/CallsListSkeleton";
import CallRequestList from "@case-details-calls/CallRequestList";
import CallsEmptyState from "@case-details-calls/CallsEmptyState";
import CallsErrorState from "@case-details-calls/CallsErrorState";
import RequestCallModal from "@case-details-calls/RequestCallModal";
import DeleteCallRequestModal from "@case-details-calls/DeleteCallRequestModal";
import ErrorBanner from "@components/common/error-banner/ErrorBanner";
import SuccessBanner from "@components/common/success-banner/SuccessBanner";
import { CALL_REQUEST_STATE_CANCELLED } from "@constants/supportConstants";
import { ERROR_BANNER_TIMEOUT_MS } from "@constants/errorBannerConstants";

export interface CallsPanelProps {
  projectId: string;
  caseId: string;
  isCaseClosed?: boolean;
}

/**
 * CallsPanel displays call requests for a specific case.
 *
 * @param {CallsPanelProps} props - The project and case identifiers.
 * @returns {JSX.Element} The rendered calls panel.
 */
export default function CallsPanel({
  projectId,
  caseId,
  isCaseClosed = false,
}: CallsPanelProps): JSX.Element {
  const [isModalOpen, setIsModalOpen] = useState(false);
  const [editCall, setEditCall] = useState<CallRequest | null>(null);
  const [deleteCall, setDeleteCall] = useState<CallRequest | null>(null);
  const [successMessage, setSuccessMessage] = useState<string | null>(null);
  const [errorMessage, setErrorMessage] = useState<string | null>(null);

  const {
    data,
    isPending,
    isError,
    refetch,
    fetchNextPage,
    hasNextPage,
    isFetchingNextPage,
  } = useGetCallRequests(projectId, caseId);
  const patchCallRequest = usePatchCallRequest(projectId, caseId);

  const callRequests = data?.pages?.flatMap((p) => p.callRequests ?? []) ?? [];

  const handleOpenModal = () => {
    setEditCall(null);
    setSuccessMessage(null);
    setErrorMessage(null);
    setIsModalOpen(true);
  };
  const handleCloseModal = () => {
    setIsModalOpen(false);
    setEditCall(null);
  };
  const handleEditClick = (call: CallRequest) => {
    setEditCall(call);
    setIsModalOpen(true);
  };
  const handleDeleteClick = useCallback((call: CallRequest) => {
    setDeleteCall(call);
  }, []);
  const handleCloseDeleteModal = useCallback(() => {
    setDeleteCall(null);
    setErrorMessage(null);
  }, []);
  const handleConfirmDelete = useCallback(
    (reason: string) => {
      if (!deleteCall) return;
      patchCallRequest.mutate(
        {
          callRequestId: deleteCall.id,
          reason: reason.trim(),
          stateKey: CALL_REQUEST_STATE_CANCELLED,
        },
      {
        onSuccess: () => {
          handleCloseDeleteModal();
          setSuccessMessage("Call request cancelled successfully.");
          refetch();
        },
        onError: (error) => {
          handleCloseDeleteModal();
          setErrorMessage(error.message || "Failed to cancel call request.");
        },
      },
    );
    },
    [deleteCall, patchCallRequest, handleCloseDeleteModal, refetch],
  );
  const handleSuccess = useCallback(() => {
    setSuccessMessage("Call request submitted successfully.");
    refetch();
  }, [refetch]);
  const handleError = useCallback((message: string) => {
    setErrorMessage(message);
  }, []);

  useEffect(() => {
    if (!successMessage) return;
    const t = setTimeout(() => setSuccessMessage(null), ERROR_BANNER_TIMEOUT_MS);
    return () => clearTimeout(t);
  }, [successMessage]);

  useEffect(() => {
    if (!errorMessage) return;
    const t = setTimeout(() => setErrorMessage(null), ERROR_BANNER_TIMEOUT_MS);
    return () => clearTimeout(t);
  }, [errorMessage]);

  const requestCallButton = (
    <Button
      variant="contained"
      color="primary"
      startIcon={<PhoneCall size={16} />}
      onClick={handleOpenModal}
      disabled={isCaseClosed}
    >
      Request Call
    </Button>
  );

  return (
    <Stack spacing={3}>
      {successMessage && (
        <SuccessBanner
          message={successMessage}
          onClose={() => setSuccessMessage(null)}
        />
      )}
      {errorMessage && (
        <ErrorBanner
          message={errorMessage}
          onClose={() => setErrorMessage(null)}
        />
      )}

      {!(callRequests.length === 0 && !isPending && !isError) && (
        <Box sx={{ alignSelf: "flex-start" }}>{requestCallButton}</Box>
      )}

      {isPending ? (
        <CallsListSkeleton />
      ) : isError ? (
        <CallsErrorState />
      ) : callRequests.length === 0 ? (
        <CallsEmptyState action={requestCallButton} />
      ) : (
        <>
          <CallRequestList
            requests={callRequests}
            onEditClick={handleEditClick}
            onDeleteClick={handleDeleteClick}
          />
          {hasNextPage && (
            <Button
              variant="outlined"
              onClick={() => fetchNextPage()}
              disabled={isFetchingNextPage}
              sx={{ alignSelf: "flex-start" }}
            >
              {isFetchingNextPage ? "Loading..." : "Load more"}
            </Button>
          )}
        </>
      )}

      <DeleteCallRequestModal
        open={!!deleteCall}
        call={deleteCall}
        onClose={handleCloseDeleteModal}
        onConfirm={handleConfirmDelete}
        isDeleting={patchCallRequest.isPending}
      />

      <RequestCallModal
        open={isModalOpen}
        projectId={projectId}
        caseId={caseId}
        onClose={handleCloseModal}
        onSuccess={handleSuccess}
        onError={handleError}
        editCall={editCall ?? undefined}
      />
    </Stack>
  );
}
