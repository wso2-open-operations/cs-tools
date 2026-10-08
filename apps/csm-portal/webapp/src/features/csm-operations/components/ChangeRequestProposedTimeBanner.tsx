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

import { Alert, AlertTitle, Box, Button, Tooltip, Typography } from "@wso2/oxygen-ui";
import { CalendarClock, CheckCircle } from "@wso2/oxygen-ui-icons-react";
import { useId, type JSX } from "react";
import type { BeChangeRequestCustomerProposal, BeChangeRequestDetail } from "@api/backend/types";
import {
  acceptProposedTimeBlockedReason,
  customerProposalProposer,
  customerProposalProposerLabel,
  customerProposalWording,
  formatCrDateTime,
  formatCrWindow,
  formatWindowLength,
  plannedWindowMs,
  proposedWindowMs,
  STORED_TIME_ADVICE,
  storedTimeSentence,
} from "@features/csm-operations/utils/changeRequests";

interface ChangeRequestProposedTimeBannerProps {
  cr: BeChangeRequestDetail;
  /**
   * The time stored on the change while it waits in Customer Approval (`pendingCustomerProposal`): a customer's proposal
   * waiting for WSO2's answer, or, when nobody is recorded as having proposed it, a stored time with no proposal to answer.
   */
  proposal: BeChangeRequestCustomerProposal;
  /** True while a state-changing request for this change is in flight: both answers wait. */
  isPending: boolean;
  /** Opens the confirmation for "Accept proposed time" (the caller sends the PATCH). */
  onAccept: () => void;
  /** Opens the counter dialog ("Propose a different time", or a decline that keeps the current time). */
  onProposeDifferent: () => void;
  /** "Now", for the proposed-time-has-passed test; only tests pass it. */
  nowMs?: number;
}

function WindowBlock({ label, window, note }: { label: string; window: string; note?: string }): JSX.Element {
  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 0.25, minWidth: 0, flex: "1 1 240px" }}>
      <Typography variant="caption" sx={{ textTransform: "uppercase", letterSpacing: 0.4, opacity: 0.8 }}>
        {label}
      </Typography>
      <Typography variant="body2" sx={{ fontWeight: 600 }}>
        {window}
      </Typography>
      {note && (
        <Typography variant="caption" sx={{ opacity: 0.85 }}>
          {note}
        </Typography>
      )}
    </Box>
  );
}

/**
 * "The customer proposed a new time": shown under the lifecycle stepper while a proposal waits for
 * WSO2's answer (the change stays in Customer Approval, the planned window untouched).
 *
 * The two answers are the previous system's own: "Accept proposed time" (Agree: the proposal becomes the
 * planned window and the change goes straight to Scheduled, no CAB, no new customer request) and
 * "Propose a different time" (Disagree: the customer is asked again; it is also how a proposal is
 * declined, keeping the current time). Accept is the one primary action.
 *
 * With nobody recorded as having proposed the time (the date is also written by WSO2 users in the previous
 * system, and one left over from an earlier cycle reads the same) it is not a proposal, and the banner
 * does not say it is: it says a time is stored but nobody is recorded as having proposed it, in an info
 * alert that asks for no answer. Accept is disabled with the reason (no staff action stands in for the
 * customer's own answer, so the backend refuses it too), and "Propose a different time" stays: a plain
 * Re-schedule that asks the customer to approve the time WSO2 names.
 *
 * Accept is disabled, with a focusable reason, for what the page can know (nobody recorded as the proposer,
 * on hold, the proposed time already passed, no planned window to keep the length of) and for the backend's
 * own `canAccept: false`; the backend refuses each of those in words as well and stays the authority.
 */
export default function ChangeRequestProposedTimeBanner({
  cr,
  proposal,
  isPending,
  onAccept,
  onProposeDifferent,
  nowMs,
}: ChangeRequestProposedTimeBannerProps): JSX.Element {
  const titleId = useId();
  const proposer = customerProposalProposer(proposal);
  const wording = customerProposalWording(proposer);
  const planned = plannedWindowMs(cr);
  const proposed = proposedWindowMs(cr, proposal);
  const length = planned ? formatWindowLength(planned.endMs - planned.startMs) : "";
  const acceptBlocked = acceptProposedTimeBlockedReason(cr, proposal, nowMs);
  const storedWindow = formatCrWindow(proposal.startOn, proposed?.endMs ?? null);

  const accept = (
    <Button
      size="small"
      variant={proposer ? "contained" : "outlined"}
      color={proposer ? "success" : "primary"}
      startIcon={<CheckCircle size={16} />}
      disabled={isPending || !!acceptBlocked}
      onClick={onAccept}
      sx={{ flexShrink: 0 }}
    >
      Accept proposed time
    </Button>
  );

  return (
    <Alert
      severity={proposer ? "warning" : "info"}
      role="region"
      aria-labelledby={titleId}
      icon={<CalendarClock size={20} />}
      sx={{ alignItems: "flex-start", "& .MuiAlert-message": { width: "100%" } }}
    >
      <AlertTitle id={titleId}>{wording.bannerTitle}</AlertTitle>
      <Box sx={{ display: "flex", flexDirection: "column", gap: 1.5 }}>
        {proposer ? (
          <Typography variant="body2">
            The planned time stays as it is until you answer. Accepting schedules the change for the proposed time with no
            further approval; proposing a different time asks the customer again.
          </Typography>
        ) : (
          <>
            <Typography variant="body2" data-testid="cr-proposal-proposer">
              {storedTimeSentence(storedWindow)}
            </Typography>
            <Typography variant="body2">{STORED_TIME_ADVICE}</Typography>
          </>
        )}
        <Box sx={{ display: "flex", flexWrap: "wrap", gap: 2 }}>
          <WindowBlock label="Planned now" window={formatCrWindow(cr.plannedStartOn, cr.plannedEndOn)} />
          <WindowBlock
            label={wording.windowLabel}
            window={storedWindow}
            note={length && proposed?.endMs != null ? `Same length as the planned window (${length})` : undefined}
          />
        </Box>
        {proposer && (
          <Typography variant="body2" data-testid="cr-proposal-proposer">
            Proposed by {customerProposalProposerLabel(proposer)}
            {proposer.on ? ` on ${formatCrDateTime(proposer.on)}` : ""}.
          </Typography>
        )}
        <Box sx={{ display: "flex", flexWrap: "wrap", gap: 1 }}>
          {acceptBlocked ? (
            <Tooltip title={acceptBlocked}>
              {/* A disabled button is not focusable, so the tooltip alone would be unreachable by keyboard:
                  this focusable, labelled wrapper is what exposes the reason to assistive tech. */}
              <Box
                component="span"
                tabIndex={0}
                aria-label={`Accept proposed time: ${acceptBlocked}`}
                sx={{ flexShrink: 0 }}
              >
                {accept}
              </Box>
            </Tooltip>
          ) : (
            accept
          )}
          <Button
            size="small"
            variant="outlined"
            color="primary"
            startIcon={<CalendarClock size={16} />}
            disabled={isPending}
            onClick={onProposeDifferent}
            sx={{ flexShrink: 0 }}
          >
            Propose a different time
          </Button>
        </Box>
      </Box>
    </Alert>
  );
}
