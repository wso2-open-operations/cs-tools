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

import { Box, Link, Typography } from "@wso2/oxygen-ui";
import type { JSX } from "react";
import { Link as RouterLink } from "react-router";
import { formatBackendTimestampForDisplay } from "@utils/dateTime";
import { changeRequestStateLabel } from "@features/csm-operations/utils/changeRequests";
import { cabDecisionText, cabHistoryOf } from "@features/csm-operations/utils/cabHistory";
import type { BeStandardChangeCabChangeRequest, BeStandardChangeCabHistory } from "@api/backend/types";

function formatDate(value?: string | null): string {
  return formatBackendTimestampForDisplay(value, { dateStyle: "medium" }) ?? "—";
}

function changeRequestPath(id: string): string {
  return `/operations/change-requests/${encodeURIComponent(id)}`;
}

/**
 * A link to a change request: a router link, or (when the page needs to pass
 * its own navigation state) a button calling onOpen.
 */
function ChangeRequestLink({ cr, onOpen }: { cr: BeStandardChangeCabChangeRequest; onOpen?: (id: string) => void }): JSX.Element {
  const label = cr.number || cr.id;
  if (onOpen) {
    return (
      <Link component="button" variant="body2" onClick={() => onOpen(cr.id)} sx={{ verticalAlign: "baseline" }}>
        {label}
      </Link>
    );
  }
  return (
    <Link component={RouterLink} to={changeRequestPath(cr.id)} underline="hover">
      {label}
    </Link>
  );
}

/**
 * The CAB's rejections and comments (entity-service, "The CAB's pushback"): each
 * rejected change request with who rejected it, when and why, then every comment
 * a CAB approver wrote. A look-alike (a change request the CAB rejected that
 * looks like this one, not one of its own) is marked so. The comments are plain
 * text from the backend and are rendered as text.
 */
export default function StandardChangeCabPushback({
  history,
  onOpen,
  memberWord = "member",
}: {
  history?: Partial<BeStandardChangeCabHistory> | null;
  onOpen?: (id: string) => void;
  /** What a change request of its own is called ("member", or "evidence" on a proposal). */
  memberWord?: string;
}): JSX.Element {
  const { rejections, rejectionCount, comments, commentCount } = cabHistoryOf(history);
  const lookalikeNote = (member: boolean): string => (member ? "" : " · a look-alike, not a " + memberWord);
  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 1 }}>
      {rejectionCount > 0 && (
        <Box>
          <Typography variant="body2" sx={{ fontWeight: 600 }}>
            {`CAB rejections (${rejectionCount})`}
          </Typography>
          <Box component="ul" aria-label="CAB rejections" sx={{ m: 0, pl: 2.5 }}>
            {rejections.map((r) => (
              <li key={`${r.changeRequest.id}|${r.rejectedOn}`}>
                <Typography variant="body2" component="span">
                  <ChangeRequestLink cr={r.changeRequest} onOpen={onOpen} />
                  {` · ${r.changeRequest.subject} · ${changeRequestStateLabel(r.changeRequest.state).toLowerCase()}`}
                  {` · rejected ${formatDate(r.rejectedOn)}${r.rejectedBy ? ` by ${r.rejectedBy}` : ""}`}
                  {lookalikeNote(r.member)}
                </Typography>
                {r.comment && (
                  <Typography variant="body2" color="text.secondary" sx={{ whiteSpace: "pre-wrap", wordBreak: "break-word" }}>
                    {`"${r.comment}"`}
                  </Typography>
                )}
              </li>
            ))}
          </Box>
          {rejectionCount > rejections.length && (
            <Typography variant="caption" color="text.secondary">{`The newest ${rejections.length} are listed.`}</Typography>
          )}
        </Box>
      )}
      {commentCount > 0 && (
        <Box>
          <Typography variant="body2" sx={{ fontWeight: 600 }}>
            {`CAB comments (${commentCount})`}
          </Typography>
          <Box component="ul" aria-label="CAB comments" sx={{ m: 0, pl: 2.5 }}>
            {comments.map((c, i) => (
              <li key={`${c.changeRequest.id}|${c.decidedOn}|${i}`}>
                <Typography variant="body2" component="span">
                  <ChangeRequestLink cr={c.changeRequest} onOpen={onOpen} />
                  {` · ${c.approverName || "A CAB member"} ${cabDecisionText(c.state)} · ${formatDate(c.decidedOn)}`}
                  {lookalikeNote(c.member)}
                </Typography>
                <Typography variant="body2" color="text.secondary" sx={{ whiteSpace: "pre-wrap", wordBreak: "break-word" }}>
                  {`"${c.comment}"`}
                </Typography>
              </li>
            ))}
          </Box>
          {commentCount > comments.length && (
            <Typography variant="caption" color="text.secondary">{`The newest ${comments.length} are listed.`}</Typography>
          )}
        </Box>
      )}
    </Box>
  );
}
