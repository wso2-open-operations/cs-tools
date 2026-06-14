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
  IconButton,
  Paper,
  Tooltip,
  Typography,
  useTheme,
} from "@wso2/oxygen-ui";
import {
  Building,
  FolderOpen,
  Headset,
  History,
  Pin,
  PinOff,
} from "@wso2/oxygen-ui-icons-react";
import {
  useCallback,
  useEffect,
  useLayoutEffect,
  useMemo,
  useRef,
  useState,
  type JSX,
} from "react";
import { createPortal } from "react-dom";
import { useNavigate } from "react-router";
import {
  clearRecentViews,
  toggleRecentViewPin,
  useRecentViews,
  type RecentView,
  type RecentViewKind,
} from "@features/csm-recent/hooks/useRecentViews";
import RelativeTime from "@components/RelativeTime";

const PANEL_WIDTH = 360;

const KIND_LABEL: Record<RecentViewKind, string> = {
  case: "Cases",
  project: "Projects",
  account: "Accounts",
};

const KIND_ORDER: RecentViewKind[] = ["case", "project", "account"];

function kindIcon(kind: RecentViewKind): JSX.Element {
  switch (kind) {
    case "case":
      return <Headset size={16} />;
    case "project":
      return <FolderOpen size={16} />;
    case "account":
      return <Building size={16} />;
  }
}

function groupByKind(entries: RecentView[]): Record<RecentViewKind, RecentView[]> {
  const out: Record<RecentViewKind, RecentView[]> = {
    case: [],
    project: [],
    account: [],
  };
  for (const e of entries) out[e.kind].push(e);
  return out;
}

export default function RecentViewsButton(): JSX.Element {
  const navigate = useNavigate();
  const theme = useTheme();
  const recents = useRecentViews();
  const [open, setOpen] = useState(false);
  const anchorRef = useRef<HTMLButtonElement | null>(null);
  const panelRef = useRef<HTMLDivElement | null>(null);
  const [anchorRect, setAnchorRect] = useState<DOMRect | null>(null);

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

  const pinned = useMemo(() => recents.filter((e) => e.pinned), [recents]);
  const grouped = useMemo(
    () => groupByKind(recents.filter((e) => !e.pinned)),
    [recents],
  );

  const handleSelect = (entry: RecentView) => {
    setOpen(false);
    navigate(entry.href);
  };

  const renderRow = (entry: RecentView): JSX.Element => (
    <Box
      key={`${entry.kind}-${entry.id}`}
      role="button"
      tabIndex={0}
      onClick={() => handleSelect(entry)}
      onKeyDown={(e) => {
        if (e.key === "Enter" || e.key === " ") {
          e.preventDefault();
          handleSelect(entry);
        }
      }}
      sx={{
        display: "flex",
        alignItems: "center",
        gap: 1.5,
        px: 1.5,
        py: 1,
        cursor: "pointer",
        "&:hover": { bgcolor: "action.hover" },
      }}
    >
      {kindIcon(entry.kind)}
      <Box sx={{ flex: 1, minWidth: 0 }}>
        <Typography variant="body2" noWrap>
          {entry.title}
        </Typography>
        {entry.subtitle && (
          <Typography variant="caption" color="text.secondary" noWrap>
            {entry.subtitle}
          </Typography>
        )}
      </Box>
      <Tooltip title={entry.pinned ? "Unpin" : "Pin to working set"}>
        <IconButton
          size="small"
          aria-label={
            entry.pinned
              ? `Unpin ${entry.title}`
              : `Pin ${entry.title} to working set`
          }
          // Stop the row's navigate; pinning must not leave the panel.
          onClick={(e) => {
            e.stopPropagation();
            toggleRecentViewPin(entry.kind, entry.id);
          }}
          // Pinned icon carries the brand accent, but orange-on-light is ~2.5:1
          // (below the 3:1 icon floor), so shift to primary.dark in light mode
          // and keep the brighter accent in dark — `palette.mode` is unreliable
          // under CssVars, so scope with applyStyles.
          sx={(t) => ({
            flexShrink: 0,
            ...(entry.pinned
              ? {
                  color: t.palette.primary.dark,
                  ...t.applyStyles("dark", { color: t.palette.primary.main }),
                }
              : {}),
          })}
        >
          {entry.pinned ? <PinOff size={15} /> : <Pin size={15} />}
        </IconButton>
      </Tooltip>
      <Typography variant="caption" color="text.secondary" sx={{ flexShrink: 0 }}>
        <RelativeTime iso={entry.visitedAt} href={entry.href} />
      </Typography>
    </Box>
  );

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
        <Typography variant="subtitle2">Recently viewed</Typography>
        {grouped.case.length +
          grouped.project.length +
          grouped.account.length >
          0 && (
          <Tooltip title="Clear history (pinned items are kept)">
            <Button
              size="small"
              variant="text"
              onClick={() => clearRecentViews()}
            >
              Clear history
            </Button>
          </Tooltip>
        )}
      </Box>

      <Box sx={{ overflowY: "auto", flex: 1 }}>
        {recents.length === 0 && (
          <Box sx={{ px: 1.5, py: 2.5, textAlign: "center" }}>
            <Typography variant="body2" color="text.secondary">
              No recent items. Open a case, project, or account to start tracking history. Pin the ones you are actively working to keep them here.
            </Typography>
          </Box>
        )}

        {pinned.length > 0 && (
          <Box>
            <Box
              sx={{
                display: "flex",
                alignItems: "center",
                gap: 0.5,
                px: 1.5,
                py: 0.5,
                bgcolor: "background.default",
                borderBottom: 1,
                borderColor: "divider",
              }}
            >
              <Pin size={12} />
              <Typography
                variant="caption"
                color="text.secondary"
                sx={{ fontWeight: 600 }}
              >
                Pinned · working set
              </Typography>
            </Box>
            {pinned.map(renderRow)}
          </Box>
        )}

        {KIND_ORDER.map((kind) => {
          const group = grouped[kind];
          if (group.length === 0) return null;
          return (
            <Box key={kind}>
              <Box
                sx={{
                  px: 1.5,
                  py: 0.5,
                  bgcolor: "background.default",
                  borderBottom: 1,
                  borderColor: "divider",
                }}
              >
                <Typography variant="caption" color="text.secondary" sx={{ fontWeight: 600 }}>
                  {KIND_LABEL[kind]}
                </Typography>
              </Box>
              {group.map(renderRow)}
            </Box>
          );
        })}
      </Box>
    </Paper>
  );

  return (
    <>
      <Tooltip title="Recently viewed">
        <IconButton
          ref={anchorRef}
          size="small"
          aria-label="Recently viewed"
          onClick={() => setOpen((o) => !o)}
        >
          <History size={20} />
        </IconButton>
      </Tooltip>
      {panel && createPortal(panel, document.body)}
    </>
  );
}
