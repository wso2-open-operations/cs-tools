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
  Card,
  Chip,
  Divider,
  Skeleton,
  Typography,
} from "@wso2/oxygen-ui";
import { ChevronDown } from "@wso2/oxygen-ui-icons-react";
import type { JSX } from "react";
import QueryErrorState from "@components/QueryErrorState";
import { formatBackendTimestampForDisplay } from "@utils/dateTime";
import { useGetChangeRequestApprovals } from "@features/csm-operations/api/useGetChangeRequestApprovals";
import {
  approvalStatusColor,
  approvalStatusLabel,
} from "@features/csm-operations/utils/changeRequests";
import type { BeChangeRequestApproval } from "@api/backend/types";

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

function ApprovalStage({ approval }: { approval: BeChangeRequestApproval }): JSX.Element {
  return (
    <Accordion disableGutters sx={{ "&:before": { display: "none" } }}>
      <AccordionSummary expandIcon={<ChevronDown size={16} />}>
        <Box sx={{ display: "flex", alignItems: "center", gap: 1.5, flexWrap: "wrap", minWidth: 0 }}>
          <Typography variant="body2" fontWeight={600} sx={{ minWidth: 100 }}>
            {approval.stage}
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
            {approval.approvers.map((approver) => (
              <Box
                key={approver.id}
                sx={{
                  display: "flex",
                  alignItems: "center",
                  gap: 1.5,
                  flexWrap: "wrap",
                  py: 0.5,
                }}
              >
                <Typography variant="body2" sx={{ minWidth: 200 }}>
                  {approver.name || "Unknown approver"}
                </Typography>
                <Chip
                  size="small"
                  variant="outlined"
                  color={approvalStatusColor(approver.status)}
                  label={approvalStatusLabel(approver.status)}
                />
                <Typography variant="caption" color="text.secondary" sx={{ ml: "auto" }}>
                  {approver.respondedOn ? formatDateTime(approver.respondedOn) : "—"}
                </Typography>
              </Box>
            ))}
          </Box>
        )}
      </AccordionDetails>
    </Accordion>
  );
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

  return (
    <Card sx={{ p: 2.5, display: "flex", flexDirection: "column", gap: 1.5 }}>
      <Typography variant="subtitle2">Approvals</Typography>
      <Box sx={{ display: "flex", flexDirection: "column" }}>
        {approvals.map((approval, index) => (
          <Box key={`${approval.stage}-${index}`}>
            {index > 0 && <Divider />}
            <ApprovalStage approval={approval} />
          </Box>
        ))}
      </Box>
    </Card>
  );
}
