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

import { useState, type JSX } from "react";
import { Box, Button, Menu, MenuItem, Tooltip } from "@wso2/oxygen-ui";
import {
  AlertTriangle,
  CalendarClock,
  ChevronDown,
  Clock,
  Gauge,
  GitBranch,
  Link as LinkIcon,
  ListChecks,
  PauseCircle,
  Pencil,
  Play,
  User,
} from "@wso2/oxygen-ui-icons-react";
import type { CaseDetail } from "@src/types";
import { ACTION_BAR_CONTROL_HEIGHT } from "./CaseActionBar";

interface SecondaryItem {
  key: string;
  label: string;
  icon: JSX.Element;
  /** If true, render with a divider below in the menu. */
  divider?: boolean;
  disabled?: boolean;
  /** Hover hint — explains why an item is disabled. */
  tooltip?: string;
  onClick?: () => void;
}

/**
 * The overflow menu of state-independent case actions, mirroring the webapp's
 * CaseActionBar "More" menu (see that file's buildSecondaryItems). "Copy case
 * link" and "Hold auto-closure" are intentionally left out for now. Everything
 * else appears in the same order/labels/icons as the webapp, but only "Change
 * severity" and "Log time" are wired to real actions here — the rest have no
 * microapp implementation yet and render disabled with a "not available yet"
 * tooltip so they're ready to be wired up one at a time.
 */
function buildItems(
  caseDetail: CaseDetail,
  currentUserId: string | null | undefined,
  onChangeSeverity: () => void,
  onLogTime: () => void,
): SecondaryItem[] {
  const items: SecondaryItem[] = [];
  const caseClosed = caseDetail.state === "closed";
  const NOT_AVAILABLE = "Not available yet.";

  // Guard on `currentUserId` being present first: `assignedEngineer?.id` is `undefined` for an
  // unassigned case, which would otherwise equal an `undefined` currentUserId (an unidentified
  // caller) and wrongly show the toggle for a case nobody — including the viewer — is assigned to.
  if (currentUserId && caseDetail.assignedEngineer?.id === currentUserId && caseDetail.state === "work_in_progress") {
    const ongoing = caseDetail.workState === "ongoing";
    items.push({
      key: "toggle_work_state",
      label: ongoing ? "Pause work" : "Resume work",
      icon: ongoing ? <PauseCircle size={16} /> : <Play size={16} />,
      divider: true,
      disabled: true,
      tooltip: NOT_AVAILABLE,
    });
  }

  items.push(
    {
      key: "raise_git_issue",
      label: "Raise internal Git issue…",
      icon: <GitBranch size={16} />,
      divider: true,
      disabled: true,
      tooltip: NOT_AVAILABLE,
    },
    {
      key: "reassign_engineer",
      label: "Assign / reassign engineer…",
      icon: <User size={16} />,
      divider: true,
      disabled: true,
      tooltip: NOT_AVAILABLE,
    },
  );

  // Only case-type items carry a severity — mirrors the caseDetail.severity
  // check elsewhere on this page (service requests/security reports/etc. have
  // no severity to change).
  if (caseDetail.severity) {
    items.push({
      key: "change_severity",
      label: "Change severity…",
      icon: <Gauge size={16} />,
      disabled: caseClosed,
      tooltip: caseClosed ? "This case is closed — it's read-only." : undefined,
      onClick: onChangeSeverity,
    });
  }

  items.push(
    {
      key: "edit_case_details",
      label: "Edit case details…",
      icon: <Pencil size={16} />,
      divider: true,
      disabled: true,
      tooltip: NOT_AVAILABLE,
    },
    {
      key: "create_incident",
      label: "Create incident from case…",
      icon: <AlertTriangle size={16} />,
      disabled: true,
      tooltip: NOT_AVAILABLE,
    },
    {
      key: "link_incident",
      label: "Link to incident…",
      icon: <LinkIcon size={16} />,
      divider: true,
      disabled: true,
      tooltip: NOT_AVAILABLE,
    },
    {
      key: "create_task",
      label: "Create task…",
      icon: <ListChecks size={16} />,
      disabled: true,
      tooltip: NOT_AVAILABLE,
    },
    {
      key: "set_fix_eta",
      label: "Set fix ETA…",
      icon: <CalendarClock size={16} />,
      divider: true,
      disabled: true,
      tooltip: NOT_AVAILABLE,
    },
    {
      key: "log_time",
      label: "Log time…",
      icon: <Clock size={16} />,
      onClick: onLogTime,
    },
  );

  return items;
}

interface CaseMoreMenuProps {
  caseDetail: CaseDetail;
  currentUserId: string | null | undefined;
  onChangeSeverity: () => void;
  onLogTime: () => void;
}

export function CaseMoreMenu({ caseDetail, currentUserId, onChangeSeverity, onLogTime }: CaseMoreMenuProps) {
  const [menuAnchor, setMenuAnchor] = useState<HTMLElement | null>(null);
  const items = buildItems(caseDetail, currentUserId, onChangeSeverity, onLogTime);

  return (
    <>
      <Button
        size="small"
        variant="outlined"
        color="primary"
        endIcon={<ChevronDown size={16} />}
        onClick={(e) => setMenuAnchor(e.currentTarget)}
        sx={{ borderRadius: 999, flexShrink: 0, height: ACTION_BAR_CONTROL_HEIGHT }}
      >
        More
      </Button>
      <Menu anchorEl={menuAnchor} open={!!menuAnchor} onClose={() => setMenuAnchor(null)}>
        {items.map((item) => {
          const menuItem = (
            <MenuItem
              key={item.key}
              disabled={item.disabled}
              onClick={() => {
                if (item.disabled) return;
                setMenuAnchor(null);
                item.onClick?.();
              }}
              sx={{ gap: 1.25, minHeight: 36 }}
            >
              {item.icon}
              {item.label}
            </MenuItem>
          );
          return [
            // A disabled MenuItem has `pointer-events: none`, so a Tooltip only
            // fires when it wraps a non-disabled span — wrap just the item here.
            item.tooltip ? (
              <Tooltip key={item.key} title={item.tooltip}>
                <Box component="span" sx={{ display: "block" }}>
                  {menuItem}
                </Box>
              </Tooltip>
            ) : (
              menuItem
            ),
            item.divider ? (
              <Box key={`${item.key}-divider`} sx={{ borderTop: 1, borderColor: "divider", my: 0.25 }} />
            ) : null,
          ];
        })}
      </Menu>
    </>
  );
}
