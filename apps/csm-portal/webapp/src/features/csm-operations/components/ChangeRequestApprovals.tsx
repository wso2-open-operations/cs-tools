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

import {
  Accordion,
  AccordionDetails,
  AccordionSummary,
  Box,
  Button,
  Card,
  Chip,
  Divider,
  Skeleton,
  Tooltip,
  Typography,
} from "@wso2/oxygen-ui";
import { Check, ChevronDown, ChevronRight, X } from "@wso2/oxygen-ui-icons-react";
import { useState, type JSX } from "react";
import QueryErrorState from "@components/QueryErrorState";
import { formatBackendTimestampForDisplay } from "@utils/dateTime";
import { useCurrentUser } from "@context/current-user/CurrentUserContext";
import { useErrorBanner } from "@context/error-banner/ErrorBannerContext";
import { useGetChangeRequestApprovals } from "@features/csm-operations/api/useGetChangeRequestApprovals";
import { useDecideChangeRequestApproval } from "@features/csm-operations/api/useDecideChangeRequestApproval";
import {
  approvalStatusColor,
  approvalStatusLabel,
} from "@features/csm-operations/utils/changeRequests";
import type {
  BeChangeRequestApproval,
  BeChangeRequestApprovalDecision,
  BeChangeRequestApprover,
} from "@api/backend/types";

function formatDateTime(value?: string | null): string {
  return (
    formatBackendTimestampForDisplay(value, {
      dateStyle: "medium",
      timeStyle: "short",
    }) ?? "—"
  );
}

/** "Devops Approval" (STATIC_GROUP) or a named customer contact (DYNAMIC_CONTACT). */
function approverDisplayName(approval: BeChangeRequestApproval): string {
  return approval.approverName || (approval.approverType === "DYNAMIC_CONTACT" ? "Customer contact" : "Approval group");
}

function isNotRequired(status: string): boolean {
  return status.trim().toUpperCase() === "NOT_REQUIRED";
}

/** Whether this approver row is the current user's own pending ("REQUESTED") approval. */
function isMyPendingApproval(approver: BeChangeRequestApprover, currentUserId?: string): boolean {
  return (
    !!currentUserId &&
    approver.id === currentUserId &&
    approver.status.trim().toUpperCase() === "REQUESTED"
  );
}

interface DecideHandlers {
  onDecide: (decision: BeChangeRequestApprovalDecision) => void;
  isDeciding: boolean;
}

function ApproverRow({
  approver,
  currentUserId,
  decide,
}: {
  approver: BeChangeRequestApprover;
  currentUserId?: string;
  decide?: DecideHandlers;
}): JSX.Element {
  const name = approver.name?.trim();
  const canDecide = !!decide && isMyPendingApproval(approver, currentUserId);

  return (
    <Box
      sx={{
        display: "flex",
        alignItems: "center",
        gap: 1.5,
        flexWrap: "wrap",
        py: 0.5,
      }}
    >
      {name ? (
        <Typography variant="body2" sx={{ minWidth: 200 }}>
          {name}
        </Typography>
      ) : (
        <Tooltip title={`ID: ${approver.id}`} placement="top">
          <Typography variant="body2" color="text.secondary" sx={{ minWidth: 200 }}>
            Unnamed approver
          </Typography>
        </Tooltip>
      )}
      <Chip
        size="small"
        variant="outlined"
        color={approvalStatusColor(approver.status)}
        label={approvalStatusLabel(approver.status)}
      />
      {canDecide ? (
        <Box sx={{ display: "flex", gap: 1, ml: "auto" }}>
          <Button
            size="small"
            variant="outlined"
            color="success"
            startIcon={<Check size={14} />}
            disabled={decide.isDeciding}
            onClick={() => decide.onDecide("approved")}
          >
            Approve
          </Button>
          <Button
            size="small"
            variant="outlined"
            color="error"
            startIcon={<X size={14} />}
            disabled={decide.isDeciding}
            onClick={() => decide.onDecide("rejected")}
          >
            Reject
          </Button>
        </Box>
      ) : (
        <Typography variant="caption" color="text.secondary" sx={{ ml: "auto" }}>
          {approver.respondedOn ? formatDateTime(approver.respondedOn) : "—"}
        </Typography>
      )}
    </Box>
  );
}

function ApprovalStage({
  approval,
  displayStage,
  currentUserId,
  decide,
}: {
  approval: BeChangeRequestApproval;
  displayStage: string;
  currentUserId?: string;
  decide?: DecideHandlers;
}): JSX.Element {
  const [notRequiredExpanded, setNotRequiredExpanded] = useState(false);
  const notableApprovers = approval.approvers.filter((approver) => !isNotRequired(approver.status));
  const notRequiredApprovers = approval.approvers.filter((approver) => isNotRequired(approver.status));

  return (
    <Accordion disableGutters sx={{ "&:before": { display: "none" } }}>
      <AccordionSummary expandIcon={<ChevronDown size={16} />}>
        <Box sx={{ display: "flex", alignItems: "center", gap: 1.5, flexWrap: "wrap", minWidth: 0 }}>
          <Typography variant="body2" fontWeight={600} sx={{ minWidth: 100 }}>
            {displayStage}
          </Typography>
          <Typography variant="body2" color="text.secondary">
            {approverDisplayName(approval)}
          </Typography>
          <Chip
            size="small"
            color={approvalStatusColor(approval.status)}
            label={approvalStatusLabel(approval.status)}
            sx={{ ml: "auto" }}
          />
        </Box>
      </AccordionSummary>
      <AccordionDetails>
        {approval.approvers.length === 0 ? (
          <Typography variant="body2" color="text.secondary">
            No individual approvers listed for this stage.
          </Typography>
        ) : (
          <Box sx={{ display: "flex", flexDirection: "column", gap: 1 }}>
            {notableApprovers.map((approver) => (
              <ApproverRow
                key={approver.id}
                approver={approver}
                currentUserId={currentUserId}
                decide={decide}
              />
            ))}
            {notRequiredApprovers.length > 0 && (
              <Box>
                <Button
                  size="small"
                  variant="text"
                  color="inherit"
                  onClick={() => setNotRequiredExpanded((prev) => !prev)}
                  aria-expanded={notRequiredExpanded}
                  startIcon={
                    notRequiredExpanded ? <ChevronDown size={14} /> : <ChevronRight size={14} />
                  }
                  sx={{ color: "text.secondary", textTransform: "none" }}
                >
                  {notRequiredApprovers.length} not required
                </Button>
                {notRequiredExpanded && (
                  <Box sx={{ display: "flex", flexDirection: "column", gap: 1, mt: 0.5 }}>
                    {notRequiredApprovers.map((approver) => (
                      <ApproverRow
                        key={approver.id}
                        approver={approver}
                        currentUserId={currentUserId}
                        decide={decide}
                      />
                    ))}
                  </Box>
                )}
              </Box>
            )}
          </Box>
        )}
      </AccordionDetails>
    </Accordion>
  );
}

/**
 * Appends an "(N of M)" suffix, in encounter order, to any stage label that
 * repeats (case-insensitively) across the approvals list — so a genuine
 * duplicate stage (e.g. a data issue upstream) reads as an explained repeat
 * instead of a visually-identical, unexplained duplicate. Single-occurrence
 * stages are left unchanged.
 */
function computeDisplayStages(approvals: BeChangeRequestApproval[]): string[] {
  const totalByKey = new Map<string, number>();
  for (const approval of approvals) {
    const key = approval.stage.trim().toLowerCase();
    totalByKey.set(key, (totalByKey.get(key) ?? 0) + 1);
  }
  const seenByKey = new Map<string, number>();
  return approvals.map((approval) => {
    const key = approval.stage.trim().toLowerCase();
    const total = totalByKey.get(key) ?? 1;
    if (total <= 1) return approval.stage;
    const seen = (seenByKey.get(key) ?? 0) + 1;
    seenByKey.set(key, seen);
    return `${approval.stage} (${seen} of ${total})`;
  });
}

/**
 * Approval-stage records for a change request (`GET /change-requests/{id}/approvals`):
 * who specifically needs to approve at each stage (Assess/Authorize/Customer
 * Approval, however many currently exist) and each approver's individual
 * status. Distinct from the flat `hasCustomerApproved`/`hasCustomerReviewed`
 * toggle shown in the Approval card above, which is a different, already-built
 * concept.
 */
export default function ChangeRequestApprovals({ id }: { id: string | undefined }): JSX.Element | null {
  const { data, isLoading, isError, error } = useGetChangeRequestApprovals(id);
  const { user } = useCurrentUser();
  const { showError } = useErrorBanner();
  const decideApproval = useDecideChangeRequestApproval();

  const decide: DecideHandlers | undefined = id
    ? {
        isDeciding: decideApproval.isPending,
        onDecide: (decision) =>
          decideApproval.mutate(
            { id, decision },
            {
              onError: (err) =>
                showError(
                  decision === "approved"
                    ? "Could not approve the change request."
                    : "Could not reject the change request.",
                  err,
                ),
            },
          ),
      }
    : undefined;

  if (isLoading) {
    return (
      <Card sx={{ p: 2.5, display: "flex", flexDirection: "column", gap: 2 }}>
        <Typography variant="subtitle2">Approvals</Typography>
        <Skeleton variant="rounded" height={48} />
        <Skeleton variant="rounded" height={48} />
      </Card>
    );
  }

  if (isError) {
    return (
      <Card sx={{ p: 2.5, display: "flex", flexDirection: "column", gap: 2 }}>
        <Typography variant="subtitle2">Approvals</Typography>
        <QueryErrorState message="Could not load the approval stages for this change request." error={error} />
      </Card>
    );
  }

  const approvals = data?.approvals ?? [];

  if (approvals.length === 0) {
    return (
      <Card sx={{ p: 2.5, display: "flex", flexDirection: "column", gap: 2 }}>
        <Typography variant="subtitle2">Approvals</Typography>
        <Typography variant="body2" color="text.secondary">
          No approval stages recorded for this change request.
        </Typography>
      </Card>
    );
  }

  const displayStages = computeDisplayStages(approvals);

  return (
    <Card sx={{ p: 2.5, display: "flex", flexDirection: "column", gap: 1.5 }}>
      <Typography variant="subtitle2">Approvals</Typography>
      <Box sx={{ display: "flex", flexDirection: "column" }}>
        {approvals.map((approval, index) => (
          <Box key={`${approval.stage}-${index}`}>
            {index > 0 && <Divider />}
            <ApprovalStage
              approval={approval}
              displayStage={displayStages[index]}
              currentUserId={user?.id}
              decide={decide}
            />
          </Box>
        ))}
      </Box>
    </Card>
  );
}
