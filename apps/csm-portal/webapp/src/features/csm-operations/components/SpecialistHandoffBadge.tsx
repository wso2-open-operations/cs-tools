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

import { Box, Chip, Tooltip, Typography } from "@wso2/oxygen-ui";
import { AlertTriangle, ExternalLink, Users } from "@wso2/oxygen-ui-icons-react";
import { type JSX } from "react";
import { formatBackendTimestampForDisplay } from "@utils/dateTime";
import { isHttpUrl } from "@utils/isHttpUrl";
import type { BeSpecialistHandoffSummary } from "@api/backend/types";

const REASON_LABEL: Record<string, string> = {
  "no-runbook": "Runbook is not available",
  "runbook-not-working": "Runbook doesn't solve the incident",
};

interface SpecialistHandoffBadgeProps {
  handoff: BeSpecialistHandoffSummary;
}

/**
 * Summarizes an existing specialist handoff on the incident detail page:
 * which group it went to, why, and links to the runbook task and (if one
 * exists) the internal GitHub issue. Readable regardless of whether the
 * handoff was performed through this portal or the ServiceNow UI button —
 * the read side derives it from the incident's own state either way (see
 * `CHANGES-incident-handoff.md` §3.4), so this renders the same either way.
 *
 * A missing GitHub issue is called out explicitly rather than just omitted:
 * it durably signals the `githubIssueError` case even after the one-time
 * dialog result that reported it is gone.
 */
export default function SpecialistHandoffBadge({
  handoff,
}: SpecialistHandoffBadgeProps): JSX.Element {
  const reasonLabel = handoff.reasonDescription || REASON_LABEL[handoff.reasonCode] || handoff.reasonCode;

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 1 }}>
      <Box sx={{ display: "flex", alignItems: "center", gap: 1, flexWrap: "wrap" }}>
        <Chip size="small" icon={<Users size={14} />} label={handoff.assignmentGroup.name} />
        {handoff.escalationTeam && (
          <Chip size="small" variant="outlined" label={handoff.escalationTeam} />
        )}
        <Typography variant="body2" color="text.secondary">
          {reasonLabel}
        </Typography>
      </Box>
      <Typography variant="caption" color="text.secondary">
        Handed off {formatBackendTimestampForDisplay(handoff.handedOffAt, { dateStyle: "medium", timeStyle: "short" }) ?? "—"}
        {handoff.handedOffBy ? ` by ${handoff.handedOffBy}` : ""}
      </Typography>
      <Box sx={{ display: "flex", alignItems: "center", gap: 1.5, flexWrap: "wrap" }}>
        {handoff.task.number && (
          <Typography variant="caption">
            Runbook task: <strong>{handoff.task.number}</strong>
            {handoff.task.stateLabel ? ` (${handoff.task.stateLabel})` : ""}
          </Typography>
        )}
        {handoff.githubIssueUrl && isHttpUrl(handoff.githubIssueUrl) ? (
          <Typography variant="caption">
            <a href={handoff.githubIssueUrl} target="_blank" rel="noreferrer" style={{ display: "inline-flex", alignItems: "center", gap: 4 }}>
              GitHub issue <ExternalLink size={12} />
            </a>
          </Typography>
        ) : (
          <Tooltip title="No internal GitHub issue is linked to this handoff — it may have failed to create, or none was requested.">
            <Box sx={{ display: "inline-flex", alignItems: "center", gap: 0.5, color: "warning.main" }}>
              <AlertTriangle size={12} />
              <Typography variant="caption">No GitHub issue</Typography>
            </Box>
          </Tooltip>
        )}
      </Box>
    </Box>
  );
}
