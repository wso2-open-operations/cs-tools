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
  FormControl,
  InputLabel,
  MenuItem,
  Select,
  Typography,
} from "@wso2/oxygen-ui";
import { useState, type JSX } from "react";
import type {
  BeHandoffEscalationTeam,
  BeSpecialistHandoffTeam,
  BeHandoffReasonCode,
  BeIncidentHandoffResult,
} from "@api/backend/types";
import { isHttpUrl } from "@utils/isHttpUrl";

const REASON_OPTIONS: Array<{ value: BeHandoffReasonCode; label: string }> = [
  { value: "no-runbook", label: "Runbook is not available" },
  { value: "runbook-not-working", label: "Runbook doesn't solve the incident" },
];

interface HandoffToSpecialistDialogProps {
  /** The incident's product's Special Ops teams, from
   * `GET /specialist-handoff-teams?serviceId=`: several are a required
   * choice, one is the target with nothing to choose. */
  teamOptions: BeSpecialistHandoffTeam[];
  /** True while `teamOptions` is still loading. */
  isLoadingTeams?: boolean;
  isSubmitting: boolean;
  /** Set once the mutation resolves with a result to show inline
   * (success, with or without a GitHub issue error) — `null` before submit
   * and after a hard failure (those go through the page's own error banner
   * instead, same convention as every other action in this portal). */
  result?: BeIncidentHandoffResult | null;
  onClose: () => void;
  onSubmit: (fields: { reasonCode: BeHandoffReasonCode; escalationTeam?: BeHandoffEscalationTeam }) => void;
}

/**
 * Reproduces the ServiceNow "Escalate to Special Ops Team" modal: one
 * mandatory reason select, then the team. The teams come from the entity
 * service's handoff configuration for the incident's service: a product
 * with several (Choreo) makes the team a required select; a product with
 * one (Asgardeo) names it and asks nothing.
 *
 * Unlike that modal, a `githubIssueError` on an otherwise-successful handoff
 * is shown here explicitly (`result.githubIssueError`) rather than silently
 * folded into a clean "success" — the handoff itself still succeeded (the
 * dialog does not block staying open on it), but the receiving team's GitHub
 * intake was missed and that needs to be visible before the engineer moves
 * on.
 */
export default function HandoffToSpecialistDialog({
  teamOptions,
  isLoadingTeams = false,
  isSubmitting,
  result,
  onClose,
  onSubmit,
}: HandoffToSpecialistDialogProps): JSX.Element {
  const [reasonCode, setReasonCode] = useState<BeHandoffReasonCode | "">("");
  const [escalationTeam, setEscalationTeam] = useState<BeHandoffEscalationTeam | "">("");

  const choosesTeam = teamOptions.length > 1;
  const noTeam = !isLoadingTeams && teamOptions.length === 0;
  const canSubmit =
    !!reasonCode && (!choosesTeam || !!escalationTeam) && !isLoadingTeams && !noTeam && !isSubmitting && !result;

  return (
    <Dialog open onClose={onClose} maxWidth="xs" fullWidth>
      <DialogTitle>Escalate to specialist team</DialogTitle>
      <DialogContent dividers>
        <Box sx={{ display: "flex", flexDirection: "column", gap: 2, pt: 0.5 }}>
          {result ? (
            <>
              <Alert severity="success">
                Handed off to <strong>{result.assignmentGroup.name}</strong>. Runbook task{" "}
                <strong>{result.task.number}</strong> created.
              </Alert>
              {result.githubIssueError ? (
                <Alert severity="warning">
                  The internal GitHub issue could not be created: {result.githubIssueError}. The
                  handoff itself succeeded — the receiving team just won't have a GitHub issue for
                  it yet.
                </Alert>
              ) : (
                result.githubIssue && (
                  <Typography variant="body2" color="text.secondary">
                    GitHub issue{" "}
                    {isHttpUrl(result.githubIssue.url) ? (
                      <a href={result.githubIssue.url} target="_blank" rel="noreferrer">
                        #{result.githubIssue.number}
                      </a>
                    ) : (
                      <>#{result.githubIssue.number}</>
                    )}{" "}
                    opened in {result.githubIssue.repo}.
                  </Typography>
                )
              )}
            </>
          ) : (
            <>
              <FormControl fullWidth size="small" required disabled={isSubmitting}>
                <InputLabel id="handoff-reason-label" shrink>
                  Reason
                </InputLabel>
                <Select
                  labelId="handoff-reason-label"
                  label="Reason"
                  value={reasonCode}
                  displayEmpty
                  onChange={(e) => setReasonCode(e.target.value as BeHandoffReasonCode)}
                >
                  <MenuItem value="">
                    <Typography component="span" color="text.secondary">
                      -- Select --
                    </Typography>
                  </MenuItem>
                  {REASON_OPTIONS.map((o) => (
                    <MenuItem key={o.value} value={o.value}>
                      {o.label}
                    </MenuItem>
                  ))}
                </Select>
              </FormControl>

              {isLoadingTeams ? (
                <Typography variant="body2" color="text.secondary">
                  Loading specialist teams…
                </Typography>
              ) : noTeam ? (
                <Alert severity="info">
                  No specialist team is configured for this incident's service.
                </Alert>
              ) : choosesTeam ? (
                <FormControl fullWidth size="small" required disabled={isSubmitting}>
                  <InputLabel id="handoff-team-label" shrink>
                    Escalation Team
                  </InputLabel>
                  <Select
                    labelId="handoff-team-label"
                    label="Escalation Team"
                    value={escalationTeam}
                    displayEmpty
                    onChange={(e) => setEscalationTeam(e.target.value as BeHandoffEscalationTeam)}
                  >
                    <MenuItem value="">
                      <Typography component="span" color="text.secondary">
                        -- Select --
                      </Typography>
                    </MenuItem>
                    {teamOptions.map((o) => (
                      <MenuItem key={o.key} value={o.key}>
                        {o.label}
                      </MenuItem>
                    ))}
                  </Select>
                </FormControl>
              ) : (
                <Typography variant="body2" color="text.secondary">
                  Escalates to <strong>{teamOptions[0].label}</strong>.
                </Typography>
              )}
            </>
          )}
        </Box>
      </DialogContent>
      <DialogActions>
        {result ? (
          <Button variant="contained" onClick={onClose}>
            Done
          </Button>
        ) : (
          <>
            <Button color="inherit" onClick={onClose} disabled={isSubmitting}>
              Cancel
            </Button>
            <Button
              variant="contained"
              disabled={!canSubmit}
              onClick={() =>
                reasonCode &&
                onSubmit({
                  reasonCode,
                  // A single team is the target already; only a choice is sent.
                  escalationTeam: (choosesTeam && escalationTeam) || undefined,
                })
              }
            >
              {isSubmitting ? "Escalating…" : "Escalate"}
            </Button>
          </>
        )}
      </DialogActions>
    </Dialog>
  );
}
