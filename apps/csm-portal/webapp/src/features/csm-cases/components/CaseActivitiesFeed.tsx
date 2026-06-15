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
  Checkbox,
  Chip,
  ListItemText,
  Menu,
  MenuItem,
  Paper,
  Typography,
} from "@wso2/oxygen-ui";
import {
  Activity,
  ArrowDown,
  ArrowUp,
  ArrowUpRight,
  CheckCircle,
  Link as LinkIcon,
  ListFilter,
  Paperclip,
  Plus,
  TriangleAlert,
  User,
  Users,
} from "@wso2/oxygen-ui-icons-react";
import { useMemo, useState, type JSX } from "react";
import CsmCaseCommentBubble from "@features/csm-cases/components/CsmCaseCommentBubble";
import RelativeTime from "@components/RelativeTime";
import { formatBytes } from "@utils/formatBytes";
import type {
  CaseAttachment,
  CaseAuditEntry,
  CsmCaseComment,
} from "@features/csm-cases/types/csmCases";

type FeedEntry =
  | { kind: "comment"; at: string; comment: CsmCaseComment }
  | { kind: "audit"; at: string; entry: CaseAuditEntry }
  | { kind: "attachment"; at: string; attachment: CaseAttachment };

interface CaseActivitiesFeedProps {
  comments: CsmCaseComment[];
  audit: CaseAuditEntry[];
  attachments: CaseAttachment[];
}

const AUDIT_ICON: Record<CaseAuditEntry["kind"], JSX.Element> = {
  state_change: <CheckCircle size={14} />,
  assignee_change: <User size={14} />,
  severity_change: <TriangleAlert size={14} />,
  linked: <LinkIcon size={14} />,
  escalated: <ArrowUpRight size={14} />,
  watcher_added: <Users size={14} />,
  comment_added: <Activity size={14} />,
  attachment_added: <Paperclip size={14} />,
  sla_breached: <TriangleAlert size={14} />,
  created: <Plus size={14} />,
};

export default function CaseActivitiesFeed({
  comments,
  audit,
  attachments,
}: CaseActivitiesFeedProps): JSX.Element {
  const [showWorkNotes, setShowWorkNotes] = useState(true);
  const [showLifecycle, setShowLifecycle] = useState(true);
  const [showAttachments, setShowAttachments] = useState(true);
  const [newestFirst, setNewestFirst] = useState(true);
  const [filterAnchor, setFilterAnchor] = useState<HTMLElement | null>(null);

  const entries: FeedEntry[] = useMemo(() => {
    const out: FeedEntry[] = [];
    for (const c of comments) {
      if (c.internal && !showWorkNotes) continue;
      out.push({ kind: "comment", at: c.createdAt, comment: c });
    }
    if (showLifecycle) {
      for (const a of audit) {
        out.push({ kind: "audit", at: a.createdAt, entry: a });
      }
    }
    if (showAttachments) {
      for (const a of attachments) {
        out.push({ kind: "attachment", at: a.uploadedAt, attachment: a });
      }
    }
    out.sort((a, b) =>
      newestFirst ? b.at.localeCompare(a.at) : a.at.localeCompare(b.at),
    );
    return out;
  }, [
    comments,
    audit,
    attachments,
    showWorkNotes,
    showLifecycle,
    showAttachments,
    newestFirst,
  ]);

  const counts = {
    comments: comments.filter((c) => !c.internal).length,
    workNotes: comments.filter((c) => c.internal).length,
    lifecycle: audit.length,
    attachments: attachments.length,
  };

  // Filters live in a dropdown so the timeline reads as content, not a row of
  // loud toggles. Surface how many of the three categories are showing when not
  // all are, so the (collapsed) filter state stays discoverable.
  const filterOptions: {
    label: string;
    checked: boolean;
    toggle: () => void;
  }[] = [
    {
      label: `Work notes (${counts.workNotes})`,
      checked: showWorkNotes,
      toggle: () => setShowWorkNotes((v) => !v),
    },
    {
      label: `State changes (${counts.lifecycle})`,
      checked: showLifecycle,
      toggle: () => setShowLifecycle((v) => !v),
    },
    {
      label: `Attachments (${counts.attachments})`,
      checked: showAttachments,
      toggle: () => setShowAttachments((v) => !v),
    },
  ];
  const activeFilters = filterOptions.filter((o) => o.checked).length;
  const filterLabel =
    activeFilters < filterOptions.length
      ? `Filter (${activeFilters}/${filterOptions.length})`
      : "Filter";

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 1.5 }}>
      <Box
        sx={{
          display: "flex",
          alignItems: "center",
          gap: 1,
          flexWrap: "wrap",
        }}
      >
        <Box sx={{ flex: 1 }} />
        <Chip
          size="small"
          variant="outlined"
          icon={<ListFilter size={14} />}
          label={filterLabel}
          onClick={(e) => setFilterAnchor(e.currentTarget)}
          aria-haspopup="true"
          aria-expanded={Boolean(filterAnchor)}
        />
        <Menu
          anchorEl={filterAnchor}
          open={Boolean(filterAnchor)}
          onClose={() => setFilterAnchor(null)}
          anchorOrigin={{ vertical: "bottom", horizontal: "right" }}
          transformOrigin={{ vertical: "top", horizontal: "right" }}
        >
          {filterOptions.map((o) => (
            <MenuItem key={o.label} dense onClick={o.toggle}>
              <Checkbox
                size="small"
                edge="start"
                checked={o.checked}
                tabIndex={-1}
                disableRipple
              />
              <ListItemText primary={o.label} />
            </MenuItem>
          ))}
        </Menu>
        <Chip
          size="small"
          variant="outlined"
          icon={newestFirst ? <ArrowDown size={14} /> : <ArrowUp size={14} />}
          label={newestFirst ? "Newest first" : "Oldest first"}
          onClick={() => setNewestFirst((v) => !v)}
          aria-label={`Sort: ${newestFirst ? "newest first" : "oldest first"} (click to reverse)`}
        />
      </Box>

      {entries.length === 0 ? (
        <Typography variant="body2" color="text.secondary">
          Nothing to show — enable more categories in the Filter menu.
        </Typography>
      ) : (
        <Box sx={{ display: "flex", flexDirection: "column", gap: 1.25 }}>
          {entries.map((e) => {
            if (e.kind === "comment") {
              return (
                <CsmCaseCommentBubble
                  key={`c-${e.comment.id}`}
                  comment={e.comment}
                />
              );
            }
            if (e.kind === "audit") {
              const isBreach = e.entry.kind === "sla_breached";
              return (
                <Paper
                  id={e.entry.id}
                  key={`a-${e.entry.id}`}
                  variant="outlined"
                  sx={{
                    p: 1,
                    display: "flex",
                    alignItems: "center",
                    gap: 1.25,
                    backgroundColor: isBreach ? "error.50" : "background.default",
                    borderColor: isBreach ? "error.main" : undefined,
                    scrollMarginTop: 96,
                  }}
                >
                  <Box
                    sx={{
                      color: isBreach ? "error.main" : "text.secondary",
                      display: "flex",
                    }}
                  >
                    {AUDIT_ICON[e.entry.kind]}
                  </Box>
                  <Box sx={{ flex: 1, minWidth: 0 }}>
                    <Typography variant="body2">
                      {e.entry.description}
                    </Typography>
                    <Typography variant="caption" color="text.secondary">
                      {e.entry.actor} ·{" "}
                      <RelativeTime
                        iso={e.entry.createdAt}
                        href={`#${e.entry.id}`}
                      />
                    </Typography>
                  </Box>
                  <Chip
                    size="small"
                    variant="outlined"
                    label="Lifecycle"
                    color={isBreach ? "error" : "default"}
                  />
                </Paper>
              );
            }
            // attachment
            return (
              <Paper
                id={e.attachment.id}
                key={`f-${e.attachment.id}`}
                variant="outlined"
                sx={{
                  p: 1,
                  display: "flex",
                  alignItems: "center",
                  gap: 1.25,
                  backgroundColor: "background.default",
                  scrollMarginTop: 96,
                }}
              >
                <Paperclip size={14} />
                <Box sx={{ flex: 1, minWidth: 0 }}>
                  <Typography variant="body2">
                    <strong>{e.attachment.filename}</strong>{" "}
                    <Typography
                      component="span"
                      variant="caption"
                      color="text.secondary"
                    >
                      ({formatBytes(e.attachment.size)} · {e.attachment.contentType})
                    </Typography>
                  </Typography>
                  <Typography variant="caption" color="text.secondary">
                    Uploaded by {e.attachment.uploadedBy} ·{" "}
                    <RelativeTime
                      iso={e.attachment.uploadedAt}
                      href={`#${e.attachment.id}`}
                    />
                  </Typography>
                </Box>
                <Chip size="small" variant="outlined" label="Attachment" />
              </Paper>
            );
          })}
        </Box>
      )}
    </Box>
  );
}

// ---------------------------------------------------------------------------
// Standalone Attachments table — used by the Attachments tab content (if any
// surface needs the raw list separately from the unified feed).
// ---------------------------------------------------------------------------

export function AttachmentsList({
  attachments,
}: {
  attachments: CaseAttachment[];
}): JSX.Element {
  if (attachments.length === 0) {
    return (
      <Typography variant="body2" color="text.secondary">
        No attachments on this case.
      </Typography>
    );
  }
  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 0.75 }}>
      {attachments.map((a) => (
        <Paper
          key={a.id}
          variant="outlined"
          sx={{ p: 1, display: "flex", alignItems: "center", gap: 1.25 }}
        >
          <Paperclip size={14} />
          <Box sx={{ flex: 1, minWidth: 0 }}>
            <Typography variant="body2">
              <strong>{a.filename}</strong>
            </Typography>
            <Typography variant="caption" color="text.secondary">
              {formatBytes(a.size)} · {a.contentType} · uploaded by{" "}
              {a.uploadedBy} · <RelativeTime iso={a.uploadedAt} />
            </Typography>
          </Box>
        </Paper>
      ))}
    </Box>
  );
}
