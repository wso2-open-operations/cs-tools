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
  Box,
  Button,
  Card,
  Chip,
  Menu,
  MenuItem,
  Tooltip,
  Typography,
} from "@wso2/oxygen-ui";
import {
  AlertTriangle,
  Bell,
  CheckCircle,
  ChevronDown,
  Clock,
  Copy,
  ExternalLink,
  Eye,
  EyeOff,
  Inbox,
  Link as LinkIcon,
  Play,
  RotateCcw,
  Send,
  TriangleAlert,
  User,
  Users,
} from "@wso2/oxygen-ui-icons-react";
import { useState, type JSX } from "react";
import type {
  CaseLifecycleAction,
  CsmCaseDetail,
} from "@features/csm-cases/types/csmCases";
import type { CaseState } from "@features/csm-dashboard/types/abtDashboard";

type PrimaryAction = {
  action: CaseLifecycleAction;
  label: string;
  color: "primary" | "success" | "warning" | "error";
  icon: JSX.Element;
};

const PRIMARY_BY_STATE: Record<CaseState, PrimaryAction[]> = {
  open: [
    {
      action: "start_work",
      label: "Start work",
      color: "primary",
      icon: <Play size={16} />,
    },
    {
      action: "assign_to_me",
      label: "Assign to me",
      color: "primary",
      icon: <User size={16} />,
    },
  ],
  work_in_progress: [
    {
      action: "propose_solution",
      label: "Propose solution",
      color: "success",
      icon: <Send size={16} />,
    },
    {
      action: "request_info",
      label: "Request info",
      color: "warning",
      icon: <Inbox size={16} />,
    },
    {
      action: "wait_on_wso2",
      label: "Wait on WSO2",
      color: "warning",
      icon: <Clock size={16} />,
    },
  ],
  solution_proposed: [
    {
      action: "close",
      label: "Mark resolved",
      color: "success",
      icon: <CheckCircle size={16} />,
    },
    {
      action: "resume_work",
      label: "Resume work",
      color: "primary",
      icon: <RotateCcw size={16} />,
    },
  ],
  awaiting_info: [
    {
      action: "resume_work",
      label: "Resume work",
      color: "primary",
      icon: <Play size={16} />,
    },
    {
      action: "close_no_response",
      label: "Close (no response)",
      color: "warning",
      icon: <CheckCircle size={16} />,
    },
  ],
  waiting_on_wso2: [
    {
      action: "resume_work",
      label: "Resume work",
      color: "primary",
      icon: <Play size={16} />,
    },
  ],
  reopen: [
    {
      action: "start_work",
      label: "Start work",
      color: "primary",
      icon: <Play size={16} />,
    },
  ],
  closed: [
    {
      action: "reopen",
      label: "Reopen",
      color: "warning",
      icon: <RotateCcw size={16} />,
    },
  ],
};

interface SecondaryItem {
  key: string;
  label: string;
  icon: JSX.Element;
  /** If true, render with a divider below in the menu. */
  divider?: boolean;
}

function buildSecondaryItems(c: CsmCaseDetail): SecondaryItem[] {
  return [
    { key: "reassign_engineer", label: "Reassign engineer…", icon: <User size={16} /> },
    { key: "reassign_group", label: "Reassign to group…", icon: <Users size={16} />, divider: true },
    {
      key: "escalate",
      label: "Escalate to lead…",
      icon: <TriangleAlert size={16} />,
    },
    {
      key: "create_incident",
      label: "Create incident from case…",
      icon: <AlertTriangle size={16} />,
      divider: true,
    },
    { key: "link_case", label: "Link related case…", icon: <LinkIcon size={16} /> },
    {
      key: "link_incident",
      label: "Link to incident…",
      icon: <LinkIcon size={16} />,
      divider: true,
    },
    { key: "log_time", label: "Log time…", icon: <Clock size={16} /> },
    {
      key: "watch",
      label: c.isWatching ? "Stop watching" : "Watch case",
      icon: c.isWatching ? <EyeOff size={16} /> : <Eye size={16} />,
      divider: true,
    },
    { key: "copy_link", label: "Copy case link", icon: <Copy size={16} /> },
    { key: "open_in_sn", label: "Open in ServiceNow", icon: <ExternalLink size={16} /> },
  ];
}

interface CaseActionBarProps {
  caseDetail: CsmCaseDetail;
  onAction: (
    action: CaseLifecycleAction | { secondary: string },
  ) => void | Promise<unknown>;
}

/**
 * Lifecycle + overflow action bar for the case detail page. Primary buttons
 * are state-driven (only valid transitions show). Secondary actions live in
 * an overflow menu and are state-independent.
 */
export default function CaseActionBar({
  caseDetail,
  onAction,
}: CaseActionBarProps): JSX.Element {
  const [menuAnchor, setMenuAnchor] = useState<HTMLElement | null>(null);
  const primary = PRIMARY_BY_STATE[caseDetail.state] ?? [];
  const secondary = buildSecondaryItems(caseDetail);

  return (
    <Card
      variant="outlined"
      sx={{
        p: 1.25,
        display: "flex",
        alignItems: "center",
        gap: 1,
        flexWrap: "wrap",
        backgroundColor: "background.default",
      }}
    >
      <Typography
        variant="caption"
        color="text.secondary"
        sx={{ mr: 0.5, pl: 0.5, alignSelf: "center" }}
      >
        Lifecycle:
      </Typography>
      {primary.map((p, idx) => (
        <Button
          key={p.action}
          size="small"
          variant={idx === 0 ? "contained" : "outlined"}
          color={p.color}
          startIcon={p.icon}
          onClick={() => void onAction(p.action)}
        >
          {p.label}
        </Button>
      ))}

      <Box sx={{ flex: 1 }} />

      <Tooltip title={caseDetail.isWatching ? "Watching" : "Not watching"}>
        <Chip
          size="small"
          variant={caseDetail.isWatching ? "filled" : "outlined"}
          color={caseDetail.isWatching ? "primary" : "default"}
          icon={<Bell size={14} />}
          label={`${caseDetail.watchers.length} watching`}
          onClick={() => void onAction({ secondary: "watch" })}
        />
      </Tooltip>

      <Button
        size="small"
        variant="outlined"
        color="primary"
        endIcon={<ChevronDown size={16} />}
        onClick={(e) => setMenuAnchor(e.currentTarget)}
      >
        More actions
      </Button>
      <Menu
        anchorEl={menuAnchor}
        open={!!menuAnchor}
        onClose={() => setMenuAnchor(null)}
      >
        {secondary.map((item) => [
          <MenuItem
            key={item.key}
            onClick={() => {
              setMenuAnchor(null);
              void onAction({ secondary: item.key });
            }}
            sx={{ gap: 1.25, minHeight: 36 }}
          >
            {item.icon}
            {item.label}
          </MenuItem>,
          item.divider ? (
            <Box
              key={`${item.key}-divider`}
              sx={{ borderTop: 1, borderColor: "divider", my: 0.25 }}
            />
          ) : null,
        ])}
      </Menu>
    </Card>
  );
}
