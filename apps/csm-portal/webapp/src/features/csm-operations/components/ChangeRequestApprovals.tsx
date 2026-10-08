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
  Alert,
  Box,
  Button,
  Card,
  Chip,
  Link,
  Skeleton,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  Tooltip,
  Typography,
} from "@wso2/oxygen-ui";
import { Check, X } from "@wso2/oxygen-ui-icons-react";
import { useState, type JSX } from "react";
import QueryErrorState from "@components/QueryErrorState";
import { formatBackendTimestampForDisplay } from "@utils/dateTime";
import { BackendApiError } from "@api/backend/client";
import { useCurrentUser } from "@context/current-user/CurrentUserContext";
import { useErrorBanner } from "@context/error-banner/ErrorBannerContext";
import ApprovalGroupDialog, {
  type ApprovalGroupPerson,
  type ApprovalGroupTarget,
} from "@features/csm-operations/components/ApprovalGroupDialog";
import { useGetChangeRequestApprovals } from "@features/csm-operations/api/useGetChangeRequestApprovals";
import { useDecideChangeRequestApproval } from "@features/csm-operations/api/useDecideChangeRequestApproval";
import {
  approvalStageLabel,
  approvalStatusColor,
  approvalStatusLabel,
} from "@features/csm-operations/utils/changeRequests";
import type {
  BeChangeRequestApproval,
  BeChangeRequestApprovalDecision,
  BeChangeRequestApprover,
  BeCustomerContact,
} from "@api/backend/types";

function formatDateTime(value?: string | null): string {
  return (
    formatBackendTimestampForDisplay(value, {
      dateStyle: "medium",
      timeStyle: "short",
    }) ?? "—"
  );
}

/** The stages the backend provisions for the CR's customer group (the project's registered contacts). */
function isCustomerStage(stageName: string): boolean {
  return stageName === "Customer Approval" || stageName === "Customer Review";
}

/** "Devops Approval" (STATIC_GROUP), a named customer contact (DYNAMIC_CONTACT)
 * or, for a Customer Approval / Customer Review stage, the customer group (the
 * project's registered contacts) who are the approvers. */
function approverGroupName(approval: BeChangeRequestApproval): string {
  if (approval.approverName) return approval.approverName;
  if (approval.approverType === "DYNAMIC_CONTACT") return "Customer contact";
  return isCustomerStage(approvalStageLabel(approval.stage)) ? "Customer group" : "Approval group";
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

/** One flattened row: an individual approver plus the assignment group of the
 * approval stage they belong to. Real ServiceNow's own Approvers list (the
 * reference this table matches) has no separate "stage" grouping at all —
 * every approver record for the change request appears in one flat table,
 * distinguished only by their own state and assignment group, so nesting
 * approvers under a collapsible per-stage card (as this component used to)
 * was needless structure a real approver never asked for: reported live as
 * confusing — an approver looking for their own pending decision does not
 * benefit from first finding "their" stage card and expanding it. */
interface ApproverTableRow {
  key: string;
  approver: BeChangeRequestApprover;
  groupName: string;
  /** What clicking the Assignment group opens: the group page, the customer
   * group's contacts, or `null` (plain text) when the stage has neither. */
  groupTarget: ApprovalGroupTarget | null;
  /** "Peer Approval" / "CAB Approval" / backend's own name ("ECAB Approval" on an older Emergency change). */
  stageName: string;
}

/**
 * What the Assignment group of `approval`'s rows opens. An internal stage that
 * carries its group (`assignmentGroup.id`) opens that group's page. A Customer
 * Approval / Customer Review stage has no group row -- its approvers are the
 * project's registered contacts -- so it opens the contacts already on the page
 * (the change request's `customerContacts`, else the stage's own approvers).
 * Any other stage (ServiceNow source, legacy rows) has nothing to open.
 */
function groupTargetFor(
  approval: BeChangeRequestApproval,
  groupName: string,
  customerContacts: BeCustomerContact[] | undefined,
): ApprovalGroupTarget | null {
  const group = approval.assignmentGroup;
  if (group?.id) return { kind: "group", id: group.id, name: group.name || groupName };
  if (isCustomerStage(approvalStageLabel(approval.stage))) {
    const contacts: ApprovalGroupPerson[] =
      customerContacts && customerContacts.length > 0
        ? customerContacts.map((c) => ({ id: c.id, name: c.name, email: c.email }))
        : approval.approvers
            .filter((a, i, all) => all.findIndex((b) => b.id === a.id) === i)
            .map((a) => ({ id: a.id, name: a.name?.trim() ?? "" }));
    return { kind: "customer", name: "Customer Group", contacts };
  }
  return null;
}

function flattenApprovals(
  approvals: BeChangeRequestApproval[],
  customerContacts: BeCustomerContact[] | undefined,
): ApproverTableRow[] {
  const rows: ApproverTableRow[] = [];
  approvals.forEach((approval, approvalIndex) => {
    const groupName = approverGroupName(approval);
    const groupTarget = groupTargetFor(approval, groupName, customerContacts);
    approval.approvers.forEach((approver, approverIndex) => {
      rows.push({
        key: `${approvalIndex}-${approverIndex}-${approver.id}`,
        approver,
        groupName,
        groupTarget,
        stageName: approvalStageLabel(approval.stage),
      });
    });
  });
  return rows;
}

const CREATOR_CANNOT_DECIDE =
  "You created this change request, so you can't approve or reject it. Another approver has to decide. You can still cancel it.";
const CANNOT_DECIDE = "You aren't able to approve or reject this stage.";

function ApproverActionsCell({
  approver,
  currentUserId,
  decide,
  isCreator,
  canDecide,
}: {
  approver: BeChangeRequestApprover;
  currentUserId?: string;
  decide?: DecideHandlers;
  isCreator: boolean;
  /** Backend-supplied, optional: `false` forbids deciding. */
  canDecide: boolean;
}): JSX.Element {
  if (!decide || !isMyPendingApproval(approver, currentUserId)) {
    return <>—</>;
  }
  // The creator can never approve or reject -- Peer and CAB alike. Show
  // the controls disabled with the reason, rather than silently hiding them,
  // so it's clear why this pending row can't be decided by them.
  if (isCreator || !canDecide) {
    const reason = isCreator ? CREATOR_CANNOT_DECIDE : CANNOT_DECIDE;
    return (
      <Tooltip title={reason}>
        <Box
          component="span"
          tabIndex={0}
          aria-label={`Approve and Reject unavailable: ${reason}`}
          sx={{ display: "flex", gap: 1 }}
        >
          <Button size="small" variant="outlined" color="success" startIcon={<Check size={14} />} disabled>
            Approve
          </Button>
          <Button size="small" variant="outlined" color="error" startIcon={<X size={14} />} disabled>
            Reject
          </Button>
        </Box>
      </Tooltip>
    );
  }
  return (
    <Box sx={{ display: "flex", gap: 1 }}>
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
  );
}

/**
 * Approval-stage records for a change request (`GET /change-requests/{id}/approvals`):
 * who specifically needs to approve, and each approver's individual status,
 * rendered as one flat table — Stage, State, Approver, Assignment group, Comments,
 * Created, Approved on — matching real ServiceNow's own Approvers list
 * layout rather than this app's earlier collapsible-per-stage-card design.
 * Distinct from the flat `hasCustomerApproved`/`hasCustomerReviewed` toggle
 * shown in the Approval card above, which is a different, already-built
 * concept.
 */
export default function ChangeRequestApprovals({
  id,
  isCreator = false,
  customerContacts,
}: {
  id: string | undefined;
  /** True when the signed-in user created/requested this change request -- the
   * backend refuses their approvals, so Approve/Reject render disabled with
   * the reason. See `isChangeRequestCreator`. */
  isCreator?: boolean;
  /** The change request's read-only Customer Group (its project's registered
   * contacts, `BeChangeRequestDetail.customerContacts`): what the Assignment
   * group of a Customer Approval / Customer Review row lists. */
  customerContacts?: BeCustomerContact[];
}): JSX.Element | null {
  const { data, isLoading, isError, error } = useGetChangeRequestApprovals(id);
  const { user } = useCurrentUser();
  const { showError } = useErrorBanner();
  const decideApproval = useDecideChangeRequestApproval();
  // The group whose page is open (clicked from an Assignment group cell).
  const [openGroup, setOpenGroup] = useState<ApprovalGroupTarget | null>(null);

  const decide: DecideHandlers | undefined = id
    ? {
        isDeciding: decideApproval.isPending,
        onDecide: (decision) =>
          decideApproval.mutate(
            { id, decision },
            {
              // The backend refuses with a readable 4xx message (e.g. 403 "the
              // creator of a change request cannot approve it") -- show that
              // rather than a generic failure.
              onError: (err) =>
                showError(
                  err instanceof BackendApiError && err.status < 500 && err.message
                    ? err.message
                    : decision === "approved"
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
  const rows = flattenApprovals(approvals, customerContacts);


  if (rows.length === 0) {
    return (
      <Card sx={{ p: 2.5, display: "flex", flexDirection: "column", gap: 2 }}>
        <Typography variant="subtitle2">Approvals</Typography>
        <Typography variant="body2" color="text.secondary">
          No approval stages recorded for this change request.
        </Typography>
      </Card>
    );
  }

  return (
    <Card sx={{ p: 2.5, display: "flex", flexDirection: "column", gap: 1.5 }}>
      <Typography variant="subtitle2">Approvals</Typography>
      {isCreator && (
        <Alert severity="info">
          You created this change request, so you can&apos;t approve or reject it (Peer or CAB).
          Another approver has to decide. You can still cancel it.
        </Alert>
      )}
      <Box sx={{ border: 1, borderColor: "divider", borderRadius: 1, overflow: "hidden" }}>
        <TableContainer>
          <Table size="small" sx={{ "& .MuiTableCell-root": { borderColor: "divider" } }}>
            <TableHead>
              <TableRow sx={{ bgcolor: "action.hover" }}>
                <TableCell>Stage</TableCell>
                <TableCell>State</TableCell>
                <TableCell>Approver</TableCell>
                <TableCell>Assignment group</TableCell>
                <TableCell>Comments</TableCell>
                <TableCell>Created</TableCell>
                <TableCell>Approved on</TableCell>
                <TableCell>Actions</TableCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {rows.map(({ key, approver, groupName, groupTarget, stageName }) => {
                const name = approver.name?.trim();
                return (
                  <TableRow key={key}>
                    <TableCell>{stageName}</TableCell>
                    <TableCell>
                      <Chip
                        size="small"
                        variant="outlined"
                        color={approvalStatusColor(approver.status)}
                        label={approvalStatusLabel(approver.status)}
                      />
                    </TableCell>
                    <TableCell>
                      {name || (
                        <Typography variant="body2" color="text.secondary">
                          Unnamed approver
                        </Typography>
                      )}
                    </TableCell>
                    <TableCell>
                      {groupTarget ? (
                        <Link
                          component="button"
                          type="button"
                          underline="hover"
                          variant="body2"
                          aria-haspopup="dialog"
                          aria-label={`View members of ${groupName}`}
                          onClick={() => setOpenGroup(groupTarget)}
                          sx={{ textAlign: "left", verticalAlign: "baseline" }}
                        >
                          {groupName}
                        </Link>
                      ) : (
                        groupName
                      )}
                    </TableCell>
                    <TableCell>{approver.comments?.trim() || "—"}</TableCell>
                    <TableCell>{formatDateTime(approver.createdOn)}</TableCell>
                    <TableCell>{formatDateTime(approver.respondedOn)}</TableCell>
                    <TableCell>
                      <ApproverActionsCell
                        approver={approver}
                        currentUserId={user?.id}
                        decide={decide}
                        isCreator={isCreator}
                        canDecide={approver.canDecide !== false}
                      />
                    </TableCell>
                  </TableRow>
                );
              })}
            </TableBody>
          </Table>
        </TableContainer>
      </Box>
      {openGroup && <ApprovalGroupDialog target={openGroup} onClose={() => setOpenGroup(null)} />}
    </Card>
  );
}
