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

import { useState } from "react";
import { Box, Button, Card, Collapse, Divider, Stack, Typography } from "@wso2/oxygen-ui";
import { ChevronDown } from "@wso2/oxygen-ui-icons-react";
import type { CsmTimeCard, CsmTimeSheet } from "@src/types";
import { cardDateLabel, formatMinutes, weekLabel } from "@utils/timecard";
import { TimeCardStateChip, TimeSheetStateChip } from "./TimeCardStateChip";

interface TimeSheetCardProps {
  sheet: CsmTimeSheet;
  /** Show the engineer's name in the header — used in the All / Approvals views
   * where sheets belong to other people. */
  showEngineer?: boolean;
  /** When provided, renders Approve/Reject on each `submitted` card. */
  onDecide?: (card: CsmTimeCard, state: "approved" | "rejected") => void;
  /** Id of the card whose decision is currently in flight (disables its buttons). */
  decidingCardId?: string | null;
  /** Whether the week starts expanded (the list opens the newest week by default). */
  defaultExpanded?: boolean;
}

// A weekly sheet as a tappable accordion: a one-line summary header (week, total,
// status) that expands to reveal the individual cards. Keeps a long list of weeks
// scannable on a phone.
export function TimeSheetCard({
  sheet,
  showEngineer = false,
  onDecide,
  decidingCardId,
  defaultExpanded = false,
}: TimeSheetCardProps) {
  const [expanded, setExpanded] = useState(defaultExpanded);
  const toggle = () => setExpanded((v) => !v);

  return (
    <Card variant="outlined" sx={{ overflow: "hidden" }}>
      <Stack
        direction="row"
        alignItems="center"
        gap={1}
        role="button"
        tabIndex={0}
        aria-expanded={expanded}
        onClick={toggle}
        onKeyDown={(e) => {
          if (e.key === "Enter" || e.key === " ") {
            e.preventDefault();
            toggle();
          }
        }}
        sx={{ p: 2, cursor: "pointer", userSelect: "none" }}
      >
        <ChevronDown
          size={18}
          style={{
            flexShrink: 0,
            transition: "transform 150ms ease",
            transform: expanded ? "rotate(0deg)" : "rotate(-90deg)",
          }}
        />
        <Box sx={{ flexGrow: 1, minWidth: 0 }}>
          {showEngineer && (
            <Typography variant="subtitle2" noWrap>
              {sheet.userName}
            </Typography>
          )}
          <Typography
            variant={showEngineer ? "body2" : "subtitle2"}
            color={showEngineer ? "text.secondary" : undefined}
            noWrap
          >
            {weekLabel(sheet.weekStart)}
          </Typography>
          <Typography variant="caption" color="text.secondary">
            {sheet.cards.length} {sheet.cards.length === 1 ? "card" : "cards"} · {formatMinutes(sheet.totalMinutes)}
          </Typography>
        </Box>
        <TimeSheetStateChip state={sheet.state} />
      </Stack>

      <Collapse in={expanded} unmountOnExit>
        <Divider />
        <Stack gap={1.5} sx={{ p: 2 }}>
          {sheet.cards.map((card) => (
            <TimeCardRow key={card.id} card={card} onDecide={onDecide} deciding={decidingCardId === card.id} />
          ))}
        </Stack>
      </Collapse>
    </Card>
  );
}

function TimeCardRow({
  card,
  onDecide,
  deciding,
}: {
  card: CsmTimeCard;
  onDecide?: (card: CsmTimeCard, state: "approved" | "rejected") => void;
  deciding?: boolean;
}) {
  const canDecide = onDecide && card.state === "submitted";
  return (
    <Stack gap={1}>
      <Stack direction="row" alignItems="center" justifyContent="space-between" gap={1}>
        <Box sx={{ minWidth: 0 }}>
          <Typography variant="body2" noWrap>
            {card.caseNumber}
          </Typography>
          <Typography variant="caption" color="text.secondary" noWrap display="block">
            {card.projectName}
          </Typography>
        </Box>
        <Stack alignItems="flex-end" gap={0.5} flexShrink={0}>
          <TimeCardStateChip state={card.state} />
          <Typography variant="caption" color="text.secondary">
            {cardDateLabel(card.createdOn)} · {formatMinutes(card.totalMinutes)}
            {card.billable ? "" : " · Non-billable"}
          </Typography>
        </Stack>
      </Stack>

      {canDecide && (
        <Stack direction="row" gap={1}>
          <Button
            size="small"
            variant="outlined"
            color="error"
            disabled={deciding}
            onClick={() => onDecide!(card, "rejected")}
          >
            Reject
          </Button>
          <Button
            size="small"
            variant="contained"
            color="success"
            disabled={deciding}
            onClick={() => onDecide!(card, "approved")}
          >
            Approve
          </Button>
        </Stack>
      )}
    </Stack>
  );
}
