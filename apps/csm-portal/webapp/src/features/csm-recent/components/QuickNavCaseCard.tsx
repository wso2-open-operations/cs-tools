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

import { Chip, Form, Stack, Typography } from "@wso2/oxygen-ui";
import { Calendar, User } from "@wso2/oxygen-ui-icons-react";
import type { JSX } from "react";
import SeverityChip from "@components/SeverityChip";
import StateChip from "@components/StateChip";
import WorkStateChip from "@components/WorkStateChip";
import RelativeTime from "@components/RelativeTime";
import { CASE_TYPE_LABEL } from "@features/csm-cases/utils/caseType";
import { caseIdLabel } from "@features/csm-cases/utils/caseIdentity";
import type { QuickCaseHit } from "@features/csm-cases/api/useQuickCaseSearch";

interface QuickNavCaseCardProps {
  hit: QuickCaseHit;
  active: boolean;
  onMouseEnter: () => void;
  onClick: () => void;
}

/**
 * Result card for a case hit in the quick-nav palette — mirrors the case
 * list's severity/state chips so a search hit reads the same as it would in
 * the cases table, just condensed for the palette's row height.
 */
export default function QuickNavCaseCard({
  hit,
  active,
  onMouseEnter,
  onClick,
}: QuickNavCaseCardProps): JSX.Element {
  const idLabel = caseIdLabel(hit);

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
      <Stack
        direction="row"
        spacing={1}
        alignItems="center"
        sx={{ flexWrap: "wrap" }}
      >
        {idLabel && (
          <Typography variant="body2" fontWeight={500} color="text.secondary">
            {idLabel}
          </Typography>
        )}
        <SeverityChip severity={hit.severity} />
        <StateChip state={hit.state} variant="outlined" />
        {hit.state === "work_in_progress" && hit.workState && (
          <WorkStateChip workState={hit.workState} />
        )}
        {hit.caseType && CASE_TYPE_LABEL[hit.caseType] && (
          <Chip
            size="small"
            variant="outlined"
            label={CASE_TYPE_LABEL[hit.caseType]}
            sx={{ height: 20, fontSize: "0.75rem" }}
          />
        )}
      </Stack>

      <Typography
        variant="body2"
        fontWeight={500}
        color="text.primary"
        noWrap
        title={hit.subject}
      >
        {hit.subject}
      </Typography>

      <Stack direction="row" spacing={2} sx={{ flexWrap: "wrap" }}>
        <Stack direction="row" spacing={0.5} alignItems="center">
          <Calendar size={13} />
          <Typography variant="caption" color="text.secondary">
            Updated <RelativeTime iso={hit.updatedOn ?? hit.createdOn} />
          </Typography>
        </Stack>
        {hit.assigneeName && (
          <Stack direction="row" spacing={0.5} alignItems="center">
            <User size={13} />
            <Typography variant="caption" color="text.secondary">
              Assigned to {hit.assigneeName}
            </Typography>
          </Stack>
        )}
      </Stack>
    </Form.CardButton>
  );
}
