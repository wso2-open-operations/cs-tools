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
import {
  Bell,
  CheckCircle,
  Eye,
  EyeOff,
  Pause,
  Play,
  RotateCcw,
  Send,
  X,
} from "@wso2/oxygen-ui-icons-react";
import type { JSX } from "react";
import type {
  CsmEngagementDetail,
  CsmEngagementLifecycleAction,
  CsmEngagementState,
} from "@features/csm-engagements/types/csmEngagements";

interface ActionDef {
  action: CsmEngagementLifecycleAction;
  label: string;
  color: "primary" | "success" | "warning" | "error";
  icon: JSX.Element;
}

const PRIMARY_BY_STATE: Record<CsmEngagementState, ActionDef[]> = {
  new: [
    { action: "approve_request", label: "Approve & start", color: "primary", icon: <Play size={16} /> },
    { action: "cancel_engagement", label: "Cancel", color: "error", icon: <X size={16} /> },
  ],
  requested: [
    { action: "approve_request", label: "Approve & start", color: "primary", icon: <Play size={16} /> },
    { action: "cancel_engagement", label: "Cancel", color: "error", icon: <X size={16} /> },
  ],
  in_progress: [
    { action: "complete_engagement", label: "Mark complete", color: "success", icon: <CheckCircle size={16} /> },
    { action: "put_on_hold", label: "Put on hold", color: "warning", icon: <Pause size={16} /> },
  ],
  on_hold: [
    { action: "resume_work", label: "Resume work", color: "primary", icon: <Play size={16} /> },
    { action: "cancel_engagement", label: "Cancel", color: "error", icon: <X size={16} /> },
  ],
  completed: [
    { action: "reopen", label: "Reopen", color: "warning", icon: <RotateCcw size={16} /> },
  ],
  cancelled: [
    { action: "reopen", label: "Reopen", color: "warning", icon: <RotateCcw size={16} /> },
  ],
};

interface EngagementActionBarProps {
  engagement: CsmEngagementDetail;
  onLifecycle: (action: CsmEngagementLifecycleAction) => void;
  onToggleWatch: () => void;
  onPostStatusUpdate: () => void;
  isMutating?: boolean;
}

export default function EngagementActionBar({
  engagement,
  onLifecycle,
  onToggleWatch,
  onPostStatusUpdate,
  isMutating = false,
}: EngagementActionBarProps): JSX.Element {
  const primary = PRIMARY_BY_STATE[engagement.state] ?? [];
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
        sx={{ mr: 0.5, pl: 0.5 }}
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
          disabled={isMutating}
          onClick={() => onLifecycle(p.action)}
        >
          {p.label}
        </Button>
      ))}

      <Box sx={{ flex: 1 }} />

      <Button
        size="small"
        variant="outlined"
        color="primary"
        startIcon={<Send size={16} />}
        onClick={onPostStatusUpdate}
      >
        Post status update
      </Button>

      <Tooltip title={engagement.isWatching ? "Stop watching" : "Watch engagement"}>
        <Button
          size="small"
          variant={engagement.isWatching ? "contained" : "outlined"}
          color={engagement.isWatching ? "primary" : "inherit"}
          startIcon={engagement.isWatching ? <EyeOff size={16} /> : <Eye size={16} />}
          onClick={onToggleWatch}
        >
          <Box component="span" sx={{ display: "flex", alignItems: "center", gap: 0.5 }}>
            <Bell size={14} />
            {engagement.watchers.length}
          </Box>
        </Button>
      </Tooltip>
    </Card>
  );
}
