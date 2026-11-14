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

import AssigneeName from "@components/AssigneeName";
import { Chip, Form, Stack, Typography } from "@wso2/oxygen-ui";
import { User } from "@wso2/oxygen-ui-icons-react";
import type { JSX, ReactNode } from "react";

interface QuickNavEntityCardProps {
  /** Kind icon (incident/change-request/problem) — see `kindMeta.tsx`. */
  icon: ReactNode;
  /** Human id, e.g. the incident/CR/problem number. */
  idLabel?: string | null;
  subject: string;
  /** Raw state string — these entity kinds each have their own state model
   * (upper-snake for incidents/problems, lower-snake for CRs), so this
   * renders as a plain outlined chip rather than reusing the case-specific
   * `StateChip`, which assumes case lifecycle-state semantics/colors. */
  state?: string | null;
  assigneeName?: string;
  active: boolean;
  onMouseEnter: () => void;
  onClick: () => void;
}

/** Best-effort "Upper Case" formatting for a raw, differently-cased state string. */
function formatState(state: string): string {
  return state
    .toLowerCase()
    .split(/[_\s]+/)
    .filter(Boolean)
    .map((w) => w[0].toUpperCase() + w.slice(1))
    .join(" ");
}

/**
 * Result card for an incident/change-request/problem hit in the quick-nav
 * palette. A smaller, kind-agnostic sibling of `QuickNavCaseCard` — these
 * entities don't share the case severity/work-state model, so this only
 * renders what's common: an id, a subject, a raw state, and (when known) the
 * assignee.
 */
export default function QuickNavEntityCard({
  icon,
  idLabel,
  subject,
  state,
  assigneeName,
  active,
  onMouseEnter,
  onClick,
}: QuickNavEntityCardProps): JSX.Element {
  return (
    <Form.CardButton
      selected={active}
      onMouseEnter={onMouseEnter}
      onClick={onClick}
      sx={{
        display: "flex",
        flexDirection: "column",
        alignItems: "stretch",
        gap: 0.75,
        p: 1.5,
        width: "100%",
        minWidth: 0,
      }}
    >
      <Stack direction="row" spacing={1} alignItems="center" sx={{ flexWrap: "wrap" }}>
        {icon}
        {idLabel && (
          <Typography variant="body2" fontWeight={500} color="text.secondary">
            {idLabel}
          </Typography>
        )}
        {state && (
          <Chip
            size="small"
            variant="outlined"
            label={formatState(state)}
            sx={{ height: 20, fontSize: "0.75rem" }}
          />
        )}
      </Stack>

      <Typography
        variant="body2"
        fontWeight={500}
        color="text.primary"
        noWrap
        title={subject}
      >
        {subject}
      </Typography>

      {assigneeName && (
        <Stack direction="row" spacing={0.5} alignItems="center">
          <User size={13} />
          <Typography variant="caption" color="text.secondary">
            Assigned to <AssigneeName name={assigneeName} />
          </Typography>
        </Stack>
      )}
    </Form.CardButton>
  );
}
