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

import { Box, Button, Card, Tooltip, Typography } from "@wso2/oxygen-ui";
import { AlertTriangle, Link as LinkIcon } from "@wso2/oxygen-ui-icons-react";
import type { JSX } from "react";
import { Link as RouterLink } from "react-router";
import { parentRecordPath } from "@features/csm-cases/utils/parentRecordRoute";

interface LinkedIncidentRef {
  id: string;
  caseNumber?: string;
  type?: "case" | "incident" | "change_request" | "problem" | null;
}

interface LinkedIncidentWidgetProps {
  /** UUID of the case whose linked incident (its `parentCase`, when that
   * parent is an incident) is shown here. Only used to build the "back"
   * state for the incident's own detail page. */
  caseId: string;
  /** The case's `parentCase` ref, straight off the case detail response —
   * this widget only renders it when its `type` is `"incident"`; any other
   * parent (a parent case, change request, or problem) is out of scope here
   * and stays on `CaseMetaBand`'s generic parent-record chip instead. */
  parentCase: LinkedIncidentRef | undefined | null;
  onLinkIncident: () => void;
  /** Disables "Link to incident…" once the case is closed — matches the
   * read-only rule the rest of the case detail page applies (comment
   * composer, attachment upload, "Link to another case"). */
  linkDisabled?: boolean;
}

/**
 * The incident this case is linked to as its parent, when any — a single
 * reference (`c.parentCase`), not a list, unlike the sibling
 * `ChildCasesWidget`/`LinkedServiceRequestsWidget`/`LinkedChangeRequestsWidget`
 * cards. Reads straight off the already-loaded case detail, so it has no
 * fetch or refresh of its own.
 */
export function LinkedIncidentWidget({
  caseId,
  parentCase,
  onLinkIncident,
  linkDisabled = false,
}: LinkedIncidentWidgetProps): JSX.Element {
  const incident = parentCase?.type === "incident" ? parentCase : null;
  const incidentPath = incident ? parentRecordPath(incident) : null;
  // So Back on the incident's own page returns here instead of falling
  // through to its hardcoded list route — same pattern as the sibling
  // widgets' `backPath`.
  const backPath = `/cases/${encodeURIComponent(caseId)}`;

  return (
    <Card sx={{ p: 2.5, display: "flex", flexDirection: "column", gap: 1.5 }}>
      <Box sx={{ display: "flex", alignItems: "center", gap: 0.75 }}>
        <AlertTriangle size={16} />
        <Typography variant="subtitle2">Linked incident</Typography>
      </Box>

      {incident && incidentPath ? (
        <Typography
          component={RouterLink}
          to={incidentPath}
          state={{ from: backPath }}
          variant="body2"
          noWrap
          title={incident.caseNumber ?? incident.id}
          sx={{ color: "inherit", textDecoration: "none", display: "block" }}
        >
          {incident.caseNumber ?? incident.id}
        </Typography>
      ) : (
        <>
          <Typography variant="body2" color="text.secondary">
            No incident linked to this case.
          </Typography>
          <Box>
            <Tooltip title={linkDisabled ? "This case is closed — it's read-only." : ""}>
              <Box component="span">
                <Button
                  size="small"
                  variant="outlined"
                  startIcon={<LinkIcon size={14} />}
                  onClick={onLinkIncident}
                  disabled={linkDisabled}
                >
                  Link to incident…
                </Button>
              </Box>
            </Tooltip>
          </Box>
        </>
      )}
    </Card>
  );
}

export default LinkedIncidentWidget;
