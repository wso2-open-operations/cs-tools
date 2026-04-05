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
import { MapPin, PhoneCall } from "@wso2/oxygen-ui-icons-react";
import {
  useState,
  useCallback,
  useEffect,
  useMemo,
  type ChangeEvent,
  type JSX,
} from "react";
import type { CallRequest } from "@models/responses";
import {
  useGetCallRequests,
  CALL_REQUESTS_PAGE_SIZE,
} from "@api/useGetCallRequests";
import { usePatchCallRequest } from "@api/usePatchCallRequest";
import useGetUserDetails from "@api/useGetUserDetails";
import useGetProjectFilters from "@api/useGetProjectFilters";
import CallsListSkeleton from "@case-details-calls/CallsListSkeleton";
import CallRequestList from "@case-details-calls/CallRequestList";
import CallsEmptyState from "@case-details-calls/CallsEmptyState";
import CallsErrorState from "@case-details-calls/CallsErrorState";
import RequestCallModal from "@case-details-calls/RequestCallModal";
import DeleteCallRequestModal from "@case-details-calls/DeleteCallRequestModal";
import RejectCallRequestModal from "@case-details-calls/RejectCallRequestModal";
import ApproveCallRequestModal from "@case-details-calls/ApproveCallRequestModal";
import UserProfileModal from "@components/common/header/UserProfileModal";
import ErrorBanner from "@components/common/error-banner/ErrorBanner";
import SuccessBanner from "@components/common/success-banner/SuccessBanner";
import {
  CALL_REQUEST_STATE_CANCELLED,
  CALL_SCHEDULABLE_CASE_STATUSES,
  type CaseStatus,
} from "@constants/supportConstants";
import { ERROR_BANNER_TIMEOUT_MS } from "@constants/errorBannerConstants";

export interface CallsPanelProps {
  projectId: string;
  caseId: string;
  isCaseClosed?: boolean;
  caseStatusLabel?: string;
  /** Case severity id for minimum scheduling offset from filter metadata. */
  caseSeverityId?: string | null;
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
  caseStatusLabel,
  caseSeverityId,
}: CallsPanelProps): JSX.Element {
  const isSchedulingAllowed = useMemo(() => {
    if (!caseStatusLabel) return false;
    return CALL_SCHEDULABLE_CASE_STATUSES.includes(caseStatusLabel as CaseStatus);
  }, [caseStatusLabel]);

  const disableCallActions = isCaseClosed || !isSchedulingAllowed;

  const [isModalOpen, setIsModalOpen] = useState(false);
  const [editCall, setEditCall] = useState<CallRequest | null>(null);
  const [deleteCall, setDeleteCall] = useState<CallRequest | null>(null);
  const [approveCall, setApproveCall] = useState<CallRequest | null>(null);
  const [rejectCall, setRejectCall] = useState<CallRequest | null>(null);
  const [pendingCallAfterTz, setPendingCallAfterTz] = useState<
    | { type: "create" }
    | { type: "edit"; call: CallRequest }
    | null
  >(null);
  const [isProfileModalOpen, setIsProfileModalOpen] = useState(false);
  const [successMessage, setSuccessMessage] = useState<string | null>(null);
  const [errorMessage, setErrorMessage] = useState<string | null>(null);
  const [callRequestsPage, setCallRequestsPage] = useState(1);

  const { data: projectFilters } = useGetProjectFilters(projectId);
  const {
    data: userDetails,
    refetch: refetchUserDetails,
    isPending: isUserDetailsPending,
    isFetching: isUserDetailsFetching,
    isError: isUserDetailsError,
  } = useGetUserDetails();

  const resolvedUserTimeZone =
    userDetails?.timeZone?.trim() ||
    (userDetails as { timezone?: string } | undefined)?.timezone?.trim() ||
    "";

  const userTimeZone = resolvedUserTimeZone || undefined;

  /** True while GET /users/me has not returned usable profile data for this panel. */
  const isUserMeLoading =
    !isUserDetailsError &&
    (isUserDetailsPending ||
      (isUserDetailsFetching && userDetails === undefined));

  const needsTimeZone =
    !isUserMeLoading &&
    !!userDetails &&
    !resolvedUserTimeZone;

  // Derive filtered state keys from project filters (all non-Canceled states for search body)
  const callRequestStateKeys = useMemo<number[] | undefined>(() => {
    if (!projectFilters?.callRequestStates) return undefined;
    return projectFilters.callRequestStates
      .map((s) => Number(s.id))
      .filter((n) => !Number.isNaN(n));
  }, [projectFilters]);

  const severityAllocationMinutes = useMemo(() => {
    const map = projectFilters?.severityBasedAllocationTime;
    const sid = String(caseSeverityId ?? "").trim();
    if (!map || !sid) return undefined;
    const direct = map[sid];
    if (typeof direct === "number" && !Number.isNaN(direct)) return direct;
    const altKey = String(Number(sid));
    if (altKey !== sid) {
      const alt = map[altKey];
      if (typeof alt === "number" && !Number.isNaN(alt)) return alt;
    }
    for (const [key, val] of Object.entries(map)) {
      if (String(key).trim() === sid && typeof val === "number" && !Number.isNaN(val)) {
        return val;
      }
    }
    return undefined;
  }, [projectFilters?.severityBasedAllocationTime, caseSeverityId]);

  // Derive specific state keys from filter labels for Approve / Reject / Cancel
  const approveStateKey = useMemo<number | undefined>(() => {
    const found = projectFilters?.callRequestStates?.find((s) =>
      s.label.toLowerCase().includes("pending on wso2"),
    );
    return found ? Number(found.id) : undefined;
  }, [projectFilters]);

  const rejectStateKey = useMemo<number | undefined>(() => {
    const found = projectFilters?.callRequestStates?.find((s) =>
      s.label.toLowerCase().includes("customer rejected") ||
      s.label.toLowerCase().includes("rejected"),
    );
    return found ? Number(found.id) : undefined;
  }, [projectFilters]);

  // Canceled state key from the raw filter (before filtering out Canceled)
  const cancelStateKey = useMemo<number>(() => {
    // useGetProjectFilters already removes Canceled from callRequestStates,
    // so fall back to the constant which matches the backend value.
    return CALL_REQUEST_STATE_CANCELLED;
  }, []);

  const {
    data,
    isPending,
    isError,
    refetch,
    fetchNextPage,
    hasNextPage,
    isFetchingNextPage,
    isFetchNextPageError,
  } = useGetCallRequests(projectId, caseId, callRequestStateKeys);
  const patchCallRequest = usePatchCallRequest(projectId, caseId);

  const allCallRequests = useMemo(
    () => data?.pages?.flatMap((p) => p.callRequests ?? []) ?? [],
    [data],
  );

  const totalCallRequestRecords =
    data?.pages?.[0]?.totalRecords ?? allCallRequests.length;
  const callRequestTotalPages = Math.ceil(
    totalCallRequestRecords / CALL_REQUESTS_PAGE_SIZE,
  );
  const boundedCallRequestPage =
    callRequestTotalPages > 0 && callRequestsPage > callRequestTotalPages
      ? 1
      : callRequestsPage;

  const paginatedCallRequests = useMemo(() => {
    const start = (boundedCallRequestPage - 1) * CALL_REQUESTS_PAGE_SIZE;
    return allCallRequests.slice(start, start + CALL_REQUESTS_PAGE_SIZE);
  }, [allCallRequests, boundedCallRequestPage]);

  useEffect(() => {
    setCallRequestsPage(1);
  }, [caseId, projectId]);

  useEffect(() => {
    const neededCount = boundedCallRequestPage * CALL_REQUESTS_PAGE_SIZE;
    if (
      !isPending &&
      !isFetchingNextPage &&
      !isFetchNextPageError &&
      hasNextPage &&
      allCallRequests.length < neededCount &&
      allCallRequests.length < totalCallRequestRecords
    ) {
      fetchNextPage();
    }
  }, [
    boundedCallRequestPage,
    allCallRequests.length,
    totalCallRequestRecords,
    hasNextPage,
    fetchNextPage,
    isPending,
    isFetchingNextPage,
    isFetchNextPageError,
  ]);

  const handleCallRequestPageChange = (
    _event: ChangeEvent<unknown>,
    page: number,
  ) => {
    setCallRequestsPage(page);
  };

  const handleOpenModal = () => {
    setSuccessMessage(null);
    setErrorMessage(null);
    if (!userDetails?.timeZone?.trim()) {
      setPendingCallAfterTz({ type: "create" });
      setIsProfileModalOpen(true);
      return;
    }
    setEditCall(null);
    setIsModalOpen(true);
  };
  const handleCloseModal = () => {
    setIsModalOpen(false);
    setEditCall(null);
  };
  const handleEditClick = (call: CallRequest) => {
    if (!userDetails?.timeZone?.trim()) {
      setPendingCallAfterTz({ type: "edit", call });
      setIsProfileModalOpen(true);
      return;
    }
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

  const handleApproveClick = useCallback((call: CallRequest) => {
    setApproveCall(call);
  }, []);
  const handleCloseApproveModal = useCallback(() => {
    setApproveCall(null);
  }, []);

  const handleRejectClick = useCallback((call: CallRequest) => {
    setRejectCall(call);
  }, []);
  const handleCloseRejectModal = useCallback(() => {
    setRejectCall(null);
    setErrorMessage(null);
  }, []);
  const handleConfirmReject = useCallback(
    (reason: string) => {
      if (!rejectCall) return;
      patchCallRequest.mutate(
        {
          callRequestId: rejectCall.id,
          reason: reason.trim(),
          stateKey: rejectStateKey ?? CALL_REQUEST_STATE_CANCELLED,
        },
        {
          onSuccess: () => {
            handleCloseRejectModal();
            setSuccessMessage("Call request rejected.");
            refetch();
          },
          onError: (error) => {
            handleCloseRejectModal();
            setErrorMessage(error.message || "Failed to reject call request.");
          },
        },
      );
    },
    [rejectCall, rejectStateKey, patchCallRequest, handleCloseRejectModal, refetch],
  );

  const handleConfirmDelete = useCallback(
    (reason: string) => {
      if (!deleteCall) return;
      patchCallRequest.mutate(
        {
          callRequestId: deleteCall.id,
          cancellationReason: reason.trim(),
          stateKey: cancelStateKey,
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
    [deleteCall, cancelStateKey, patchCallRequest, handleCloseDeleteModal, refetch],
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
      disabled={disableCallActions}
    >
      Request Call
    </Button>
  );

  const mainCallsContent = (
    <>
      {!(allCallRequests.length === 0 && !isPending && !isError) && (
        <Box sx={{ alignSelf: "flex-start" }}>{requestCallButton}</Box>
      )}

      {isPending ? (
        <CallsListSkeleton />
      ) : isError ? (
        <CallsErrorState />
      ) : allCallRequests.length === 0 ? (
        <CallsEmptyState action={requestCallButton} />
      ) : (
        <Stack spacing={2}>
          <CallRequestList
            requests={paginatedCallRequests}
            userTimeZone={userTimeZone}
            onEditClick={disableCallActions ? undefined : handleEditClick}
            onDeleteClick={disableCallActions ? undefined : handleDeleteClick}
            onApproveClick={disableCallActions ? undefined : handleApproveClick}
            onRejectClick={disableCallActions ? undefined : handleRejectClick}
          />
          {callRequestTotalPages > 1 && (
            <Box sx={{ display: "flex", justifyContent: "center", mt: 1 }}>
              <Pagination
                count={callRequestTotalPages}
                page={boundedCallRequestPage}
                onChange={handleCallRequestPageChange}
                color="primary"
                showFirstButton
                showLastButton
              />
            </Box>
          )}
        </Stack>
      )}
    </>
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

      {isUserMeLoading ? (
        <Box
          role="status"
          aria-busy="true"
          aria-label="Loading your profile for call scheduling"
          sx={{ width: "100%" }}
        >
          <CallsListSkeleton />
        </Box>
      ) : needsTimeZone ? (
        <Box
          sx={{
            flex: 1,
            display: "flex",
            flexDirection: "column",
            alignItems: "center",
            justifyContent: "center",
            minHeight: 280,
            width: "100%",
            py: 4,
            px: 2,
          }}
        >
          <Box
            role="region"
            aria-label="Time zone required"
            sx={{
              maxWidth: 520,
              width: "100%",
              textAlign: "center",
            }}
          >
            <Typography
              variant="body2"
              color="text.secondary"
              sx={{ mb: 2, textAlign: "center" }}
            >
              Set your time zone first to request or reschedule a call. Go to
              your profile to choose your time zone.
            </Typography>
            <Button
              variant="contained"
              color="primary"
              startIcon={<MapPin size={16} />}
              onClick={() => setIsProfileModalOpen(true)}
            >
              Set Time Zone
            </Button>
          </Box>
        </Box>
      ) : (
        mainCallsContent
      )}

      <DeleteCallRequestModal
        open={!!deleteCall}
        call={deleteCall}
        userTimeZone={userTimeZone}
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
        userTimeZone={userTimeZone}
        severityAllocationMinutes={severityAllocationMinutes}
      />

      <ApproveCallRequestModal
        open={!!approveCall}
        call={approveCall}
        projectId={projectId}
        caseId={caseId}
        onClose={handleCloseApproveModal}
        onSuccess={() => {
          handleCloseApproveModal();
          setSuccessMessage("Call request approved successfully.");
          refetch();
        }}
        onError={(message) => setErrorMessage(message)}
        userTimeZone={userTimeZone}
        approveStateKey={approveStateKey}
      />

      <RejectCallRequestModal
        open={!!rejectCall}
        call={rejectCall}
        onClose={handleCloseRejectModal}
        onConfirm={handleConfirmReject}
        isRejecting={patchCallRequest.isPending}
      />

      <UserProfileModal
        open={isProfileModalOpen}
        onClose={() => {
          setIsProfileModalOpen(false);
          void refetchUserDetails()
            .then((result) => {
              const tz = result.data?.timeZone?.trim();
              if (!pendingCallAfterTz || !tz) {
                return;
              }
              if (pendingCallAfterTz.type === "create") {
                setEditCall(null);
                setIsModalOpen(true);
              } else {
                setEditCall(pendingCallAfterTz.call);
                setIsModalOpen(true);
              }
              setPendingCallAfterTz(null);
            })
            .catch(() => {
              setPendingCallAfterTz(null);
            });
        }}
      />
    </Stack>
  );
}
