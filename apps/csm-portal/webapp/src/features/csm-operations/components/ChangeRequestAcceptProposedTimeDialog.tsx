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
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  Typography,
} from "@wso2/oxygen-ui";
import type { JSX } from "react";
import type { BeChangeRequestCustomerProposal, BeChangeRequestDetail } from "@api/backend/types";
import {
  customerProposalProposer,
  customerProposalProposerLabel,
  customerProposalWording,
  formatCrDateTime,
  formatCrWindow,
  proposedWindowMs,
} from "@features/csm-operations/utils/changeRequests";

interface ChangeRequestAcceptProposedTimeDialogProps {
  cr: BeChangeRequestDetail;
  /** The customer's proposal waiting for WSO2's answer. */
  proposal: BeChangeRequestCustomerProposal;
  /** True while the PATCH is in flight. */
  isSubmitting: boolean;
  /** The backend's refusal for the last attempt, shown verbatim. */
  error?: string | null;
  /**
   * True when the page now holds something other than what this dialog was opened on (the planned
   * window, the proposal or the state moved) and an attempt was refused: the same request would be
   * refused again, so Accept is held back and the dialog says to close it and look at the current state.
   */
  stale?: boolean;
  onClose: () => void;
  /** Sends `{confirmCustomerUpdatedDate: "agree", expectedCustomerUpdatedOn, expectedPlannedStartOn, expectedPlannedEndOn}`. */
  onConfirm: () => void;
}

/**
 * "Accept the proposed time?": the confirmation behind the banner's primary action. Shows the
 * planned window beside the proposed one, so what is about to be scheduled is in front of the
 * engineer, and says what follows: the change goes straight to Scheduled, the customer is not asked
 * again, no CAB approval.
 *
 * It is only opened for a time a registered contact is recorded as having proposed: with nobody recorded
 * (a date WSO2 users write too, or one left over from an earlier round) the banner disables Accept and the
 * backend refuses it, since no staff action stands in for the customer's own answer. There is no
 * confirmation to tick instead. Should it ever be mounted for such a time it states the neutral words
 * (nobody is named) and the server's refusal shows as it does for any other.
 */
export default function ChangeRequestAcceptProposedTimeDialog({
  cr,
  proposal,
  isSubmitting,
  error,
  stale,
  onClose,
  onConfirm,
}: ChangeRequestAcceptProposedTimeDialogProps): JSX.Element {
  const proposer = customerProposalProposer(proposal);
  const wording = customerProposalWording(proposer);
  const proposed = proposedWindowMs(cr, proposal);
  const canConfirm = !stale && !isSubmitting;

  return (
    <Dialog open onClose={onClose} maxWidth="xs" fullWidth aria-labelledby="cr-accept-proposal-title">
      <DialogTitle id="cr-accept-proposal-title">Accept the proposed time?</DialogTitle>
      <DialogContent dividers>
        <Box sx={{ display: "flex", flexDirection: "column", gap: 2, pt: 0.5 }}>
          {error && (
            <Alert severity="error" role="alert">
              {error}
            </Alert>
          )}
          {stale && (
            <Typography variant="caption" color="text.secondary" role="status">
              This change request changed while this dialog was open, so the same request would be refused again. Close
              this dialog to see the current state.
            </Typography>
          )}
          <Typography variant="body2" color="text.secondary">
            {`The change will be scheduled for ${formatCrWindow(proposal.startOn, proposed?.endMs ?? null)}. ` +
              "The customer sees that you accepted it and is not asked again. No CAB approval is needed."}
          </Typography>
          <Box sx={{ display: "flex", flexDirection: "column", gap: 0.25 }}>
            <Typography variant="caption" color="text.secondary" sx={{ textTransform: "uppercase", letterSpacing: 0.4 }}>
              Planned now
            </Typography>
            <Typography variant="body2">{formatCrWindow(cr.plannedStartOn, cr.plannedEndOn)}</Typography>
          </Box>
          <Box sx={{ display: "flex", flexDirection: "column", gap: 0.25 }}>
            <Typography variant="caption" color="text.secondary" sx={{ textTransform: "uppercase", letterSpacing: 0.4 }}>
              {wording.windowLabel}
            </Typography>
            <Typography variant="body2" sx={{ fontWeight: 600 }}>
              {formatCrWindow(proposal.startOn, proposed?.endMs ?? null)}
            </Typography>
          </Box>
          {proposer && (
            <Typography variant="body2" color="text.secondary">
              Proposed by {customerProposalProposerLabel(proposer)}
              {proposer.on ? ` on ${formatCrDateTime(proposer.on)}` : ""}.
            </Typography>
          )}
        </Box>
      </DialogContent>
      <DialogActions>
        <Button color="inherit" onClick={onClose} disabled={isSubmitting}>
          Close
        </Button>
        <Button variant="contained" color="success" onClick={onConfirm} disabled={!canConfirm} loading={isSubmitting}>
          Accept proposed time
        </Button>
      </DialogActions>
    </Dialog>
  );
}
