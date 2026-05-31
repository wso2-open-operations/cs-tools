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
  Badge,
  Box,
  Button,
  IconButton,
  Paper,
  Tooltip,
  Typography,
  useTheme,
} from "@wso2/oxygen-ui";
import {
  AlertTriangle,
  Bell,
  CircleAlert,
  MessageSquare,
  User,
} from "@wso2/oxygen-ui-icons-react";
import {
  useCallback,
  useEffect,
  useLayoutEffect,
  useRef,
  useState,
  type JSX,
} from "react";
import { createPortal } from "react-dom";
import { useNavigate } from "react-router";
import {
  useGetCsmNotifications,
  useMarkAllNotificationsRead,
  useMarkNotificationRead,
} from "@features/csm-notifications/api/useCsmNotifications";
import RelativeTime from "@components/RelativeTime";
import type {
  CsmNotification,
  CsmNotificationKind,
} from "@features/csm-notifications/types/csmNotifications";

const PANEL_WIDTH = 380;

function kindIcon(kind: CsmNotificationKind): JSX.Element {
  switch (kind) {
    case "sla_breach":
      return <CircleAlert size={16} />;
    case "sla_at_risk":
      return <AlertTriangle size={16} />;
    case "new_comment":
      return <MessageSquare size={16} />;
    case "case_assigned":
      return <User size={16} />;
    case "escalation_opened":
      return <AlertTriangle size={16} />;
  }
}

function kindColor(kind: CsmNotificationKind): string {
  switch (kind) {
    case "sla_breach":
    case "escalation_opened":
      return "error.main";
    case "sla_at_risk":
      return "warning.main";
    case "new_comment":
      return "info.main";
    case "case_assigned":
      return "primary.main";
  }
}

export default function NotificationsBell(): JSX.Element {
  const navigate = useNavigate();
  const theme = useTheme();
  const [open, setOpen] = useState(false);
  const anchorRef = useRef<HTMLButtonElement | null>(null);
  const panelRef = useRef<HTMLDivElement | null>(null);
  const [anchorRect, setAnchorRect] = useState<DOMRect | null>(null);

  const { data, isLoading } = useGetCsmNotifications();
  const markRead = useMarkNotificationRead();
  const markAllRead = useMarkAllNotificationsRead();

  const notifications = data ?? [];
  const unreadCount = notifications.filter((n) => !n.isRead).length;

  const updateAnchorRect = useCallback(() => {
    if (anchorRef.current) {
      setAnchorRect(anchorRef.current.getBoundingClientRect());
    }
  }, []);

  useLayoutEffect(() => {
    if (open) updateAnchorRect();
  }, [open, updateAnchorRect]);

  useEffect(() => {
    if (!open) return;
    const handle = () => updateAnchorRect();
    window.addEventListener("resize", handle);
    window.addEventListener("scroll", handle, true);
    return () => {
      window.removeEventListener("resize", handle);
      window.removeEventListener("scroll", handle, true);
    };
  }, [open, updateAnchorRect]);

  // Click-outside to close.
  useEffect(() => {
    if (!open) return;
    const onDocClick = (e: MouseEvent) => {
      const target = e.target as Node;
      if (
        panelRef.current?.contains(target) ||
        anchorRef.current?.contains(target)
      ) {
        return;
      }
      setOpen(false);
    };
    document.addEventListener("mousedown", onDocClick);
    return () => document.removeEventListener("mousedown", onDocClick);
  }, [open]);

  const handleSelect = (n: CsmNotification) => {
    if (!n.isRead) markRead.mutate(n.id);
    setOpen(false);
    navigate(n.href);
  };

  const panel = open && anchorRect && (
    <Paper
      ref={panelRef}
      elevation={6}
      sx={{
        position: "fixed",
        top: anchorRect.bottom + 6,
        right: window.innerWidth - anchorRect.right,
        width: PANEL_WIDTH,
        maxHeight: 480,
        overflow: "hidden",
        display: "flex",
        flexDirection: "column",
        zIndex: theme.zIndex.modal + 1,
        border: 1,
        borderColor: "divider",
      }}
    >
      <Box
        sx={{
          display: "flex",
          alignItems: "center",
          justifyContent: "space-between",
          gap: 1,
          px: 1.5,
          py: 1,
          borderBottom: 1,
          borderColor: "divider",
          bgcolor: "background.default",
        }}
      >
        <Typography variant="subtitle2">Notifications</Typography>
        {unreadCount > 0 && (
          <Button
            size="small"
            variant="text"
            onClick={() => markAllRead.mutate()}
            disabled={markAllRead.isPending}
          >
            Mark all read
          </Button>
        )}
      </Box>

      <Box sx={{ overflowY: "auto", flex: 1 }}>
        {isLoading && (
          <Box sx={{ px: 1.5, py: 1.5 }}>
            <Typography variant="caption" color="text.secondary">
              Loading…
            </Typography>
          </Box>
        )}
        {!isLoading && notifications.length === 0 && (
          <Box sx={{ px: 1.5, py: 2.5, textAlign: "center" }}>
            <Typography variant="body2" color="text.secondary">
              No notifications.
            </Typography>
          </Box>
        )}
        {notifications.map((n) => (
          <Box
            key={n.id}
            role="button"
            tabIndex={0}
            onClick={() => handleSelect(n)}
            onKeyDown={(e) => {
              if (e.key === "Enter" || e.key === " ") {
                e.preventDefault();
                handleSelect(n);
              }
            }}
            sx={{
              display: "flex",
              alignItems: "flex-start",
              gap: 1,
              px: 1.5,
              py: 1.25,
              borderBottom: 1,
              borderColor: "divider",
              bgcolor: n.isRead ? "transparent" : "action.hover",
              cursor: "pointer",
              "&:hover": { bgcolor: "action.selected" },
              "&:last-of-type": { borderBottom: 0 },
            }}
          >
            <Box sx={{ color: kindColor(n.kind), mt: 0.25 }}>
              {kindIcon(n.kind)}
            </Box>
            <Box sx={{ flex: 1, minWidth: 0 }}>
              <Typography
                variant="body2"
                sx={{ fontWeight: n.isRead ? 400 : 600 }}
                noWrap
              >
                {n.title}
              </Typography>
              <Typography
                variant="caption"
                color="text.secondary"
                sx={{
                  display: "block",
                  whiteSpace: "nowrap",
                  overflow: "hidden",
                  textOverflow: "ellipsis",
                }}
              >
                {n.summary}
              </Typography>
              <Typography variant="caption" color="text.secondary">
                <RelativeTime iso={n.createdAt} href={n.href} />
              </Typography>
            </Box>
            {!n.isRead && (
              <Box
                sx={{
                  width: 8,
                  height: 8,
                  borderRadius: "50%",
                  bgcolor: "primary.main",
                  mt: 1,
                  flexShrink: 0,
                }}
              />
            )}
          </Box>
        ))}
      </Box>
    </Paper>
  );

  return (
    <>
      <Tooltip title="Notifications">
        <IconButton
          ref={anchorRef}
          size="small"
          aria-label={`Notifications (${unreadCount} unread)`}
          onClick={() => setOpen((o) => !o)}
        >
          <Badge
            color="error"
            badgeContent={unreadCount}
            invisible={unreadCount === 0}
            overlap="circular"
          >
            <Bell size={20} />
          </Badge>
        </IconButton>
      </Tooltip>
      {panel && createPortal(panel, document.body)}
    </>
  );
}
