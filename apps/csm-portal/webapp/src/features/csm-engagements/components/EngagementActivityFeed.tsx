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
  Avatar,
  Box,
  Button,
  Chip,
  FormControlLabel,
  Paper,
  Switch,
  TextField,
  Typography,
} from "@wso2/oxygen-ui";
import {
  Activity,
  Link as LinkIcon,
  Paperclip,
  TriangleAlert,
  User,
  Users,
} from "@wso2/oxygen-ui-icons-react";
import DOMPurify from "dompurify";
import { useMemo, useState, type JSX } from "react";
import RelativeTime from "@components/RelativeTime";
import type {
  CsmEngagementAttachment,
  CsmEngagementAuditEntry,
  CsmEngagementAuditKind,
  CsmEngagementComment,
} from "@features/csm-engagements/types/csmEngagements";

const PURIFY = {
  ALLOWED_TAGS: [
    "p",
    "br",
    "strong",
    "b",
    "em",
    "i",
    "u",
    "s",
    "code",
    "pre",
    "ul",
    "ol",
    "li",
    "a",
    "blockquote",
  ],
  ALLOWED_ATTR: ["href", "target", "rel"],
};

const AUDIT_ICON: Partial<Record<CsmEngagementAuditKind, JSX.Element>> = {
  created: <Activity size={14} />,
  state_change: <Activity size={14} />,
  stage_change: <Activity size={14} />,
  owner_change: <User size={14} />,
  task_added: <Activity size={14} />,
  task_completed: <Activity size={14} />,
  deliverable_added: <Activity size={14} />,
  deliverable_completed: <Activity size={14} />,
  deliverable_waived: <Activity size={14} />,
  status_update: <Activity size={14} />,
  watcher_added: <Users size={14} />,
  blocker_logged: <TriangleAlert size={14} />,
  comment_added: <Activity size={14} />,
  attachment_added: <Paperclip size={14} />,
  case_linked: <LinkIcon size={14} />,
};

function formatBytes(n: number): string {
  if (n < 1024) return `${n} B`;
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`;
  return `${(n / (1024 * 1024)).toFixed(1)} MB`;
}

function initials(name: string): string {
  return name
    .split(/\s+/)
    .filter(Boolean)
    .slice(0, 2)
    .map((p) => p[0]?.toUpperCase() ?? "")
    .join("");
}

interface EngagementActivityFeedProps {
  comments: CsmEngagementComment[];
  audit: CsmEngagementAuditEntry[];
  attachments: CsmEngagementAttachment[];
  onPostComment: (bodyHtml: string, internal: boolean) => Promise<void>;
  isPosting?: boolean;
}

type FeedItem =
  | { kind: "comment"; at: string; data: CsmEngagementComment }
  | { kind: "audit"; at: string; data: CsmEngagementAuditEntry }
  | { kind: "attachment"; at: string; data: CsmEngagementAttachment };

export default function EngagementActivityFeed({
  comments,
  audit,
  attachments,
  onPostComment,
  isPosting = false,
}: EngagementActivityFeedProps): JSX.Element {
  const [draft, setDraft] = useState("");
  const [internal, setInternal] = useState(false);
  const [showWorkNotes, setShowWorkNotes] = useState(true);
  const [showLifecycle, setShowLifecycle] = useState(true);
  const [showAttachments, setShowAttachments] = useState(true);

  const items: FeedItem[] = useMemo(() => {
    const all: FeedItem[] = [];
    comments.forEach((c) => {
      if (!showWorkNotes && c.internal) return;
      all.push({ kind: "comment", at: c.createdAt, data: c });
    });
    if (showLifecycle) {
      audit.forEach((a) => all.push({ kind: "audit", at: a.createdAt, data: a }));
    }
    if (showAttachments) {
      attachments.forEach((a) =>
        all.push({ kind: "attachment", at: a.uploadedAt, data: a }),
      );
    }
    return all.sort((a, b) => new Date(a.at).getTime() - new Date(b.at).getTime());
  }, [comments, audit, attachments, showWorkNotes, showLifecycle, showAttachments]);

  const handlePost = async (): Promise<void> => {
    const trimmed = draft.trim();
    if (!trimmed) return;
    // Convert newlines to <br> so the simple textarea-style input
    // still renders multi-line comments nicely.
    const html = `<p>${trimmed.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;").replace(/\n\n+/g, "</p><p>").replace(/\n/g, "<br>")}</p>`;
    await onPostComment(html, internal);
    setDraft("");
    setInternal(false);
  };

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
      {/* Reply / work-note composer */}
      <Paper variant="outlined" sx={{ p: 2, display: "flex", flexDirection: "column", gap: 1 }}>
        <Typography variant="subtitle2">Add a comment or work note</Typography>
        <TextField
          size="small"
          placeholder={
            internal
              ? "Internal work note (not customer-visible)…"
              : "Reply to the customer / share an update…"
          }
          value={draft}
          onChange={(e) => setDraft(e.target.value)}
          multiline
          minRows={3}
          sx={
            internal
              ? { "& .MuiOutlinedInput-root": { backgroundColor: "warning.50" } }
              : undefined
          }
        />
        <Box sx={{ display: "flex", alignItems: "center", gap: 1.5, flexWrap: "wrap" }}>
          <FormControlLabel
            control={
              <Switch
                size="small"
                checked={internal}
                onChange={(_, v) => setInternal(v)}
              />
            }
            label={
              <Typography variant="caption">
                Internal work note (not visible to the customer)
              </Typography>
            }
          />
          <Box sx={{ flex: 1 }} />
          <Button
            size="small"
            variant="contained"
            disabled={isPosting || draft.trim() === ""}
            onClick={() => void handlePost()}
          >
            {isPosting ? "Posting…" : internal ? "Save work note" : "Post comment"}
          </Button>
        </Box>
      </Paper>

      {/* Filter toggles */}
      <Paper
        variant="outlined"
        sx={{ p: 1, display: "flex", gap: 1.5, alignItems: "center", flexWrap: "wrap" }}
      >
        <Typography variant="caption" color="text.secondary">
          Show:
        </Typography>
        <FormControlLabel
          control={
            <Switch
              size="small"
              checked={showWorkNotes}
              onChange={(_, v) => setShowWorkNotes(v)}
            />
          }
          label={<Typography variant="caption">Work notes</Typography>}
        />
        <FormControlLabel
          control={
            <Switch
              size="small"
              checked={showLifecycle}
              onChange={(_, v) => setShowLifecycle(v)}
            />
          }
          label={<Typography variant="caption">Lifecycle events</Typography>}
        />
        <FormControlLabel
          control={
            <Switch
              size="small"
              checked={showAttachments}
              onChange={(_, v) => setShowAttachments(v)}
            />
          }
          label={<Typography variant="caption">Attachments</Typography>}
        />
        <Chip size="small" variant="outlined" label={`${items.length} entries`} />
      </Paper>

      {/* Timeline */}
      {items.length === 0 ? (
        <Typography variant="body2" color="text.secondary" sx={{ textAlign: "center", py: 4 }}>
          No activity yet.
        </Typography>
      ) : (
        items.map((it) => {
          const id = `${it.kind}-${(it.data as { id: string }).id}`;
          if (it.kind === "comment") {
            const c = it.data;
            const safe = DOMPurify.sanitize(c.bodyHtml, PURIFY);
            const isInternal = !!c.internal;
            return (
              <Box key={id} sx={{ display: "flex", gap: 1.5, alignItems: "flex-start" }}>
                <Avatar
                  sx={{
                    bgcolor:
                      c.authorRole === "customer_contact" ? "grey.500" : "primary.main",
                    width: 32,
                    height: 32,
                    fontSize: "0.85rem",
                  }}
                >
                  {initials(c.authorName)}
                </Avatar>
                <Paper
                  variant="outlined"
                  sx={{
                    p: 1.5,
                    flex: 1,
                    minWidth: 0,
                    display: "flex",
                    flexDirection: "column",
                    gap: 0.75,
                    backgroundColor: isInternal ? "warning.50" : undefined,
                    borderColor: isInternal ? "warning.main" : undefined,
                    borderLeft: isInternal ? 3 : undefined,
                  }}
                >
                  <Box sx={{ display: "flex", alignItems: "center", gap: 1, flexWrap: "wrap" }}>
                    <Typography variant="subtitle2">{c.authorName}</Typography>
                    <Chip
                      size="small"
                      label={c.authorRole.replace(/_/g, " ")}
                      variant="outlined"
                    />
                    {isInternal && (
                      <Chip size="small" label="Internal work note" color="warning" />
                    )}
                    <Typography variant="caption" color="text.secondary">
                      <RelativeTime iso={c.createdAt} />
                    </Typography>
                  </Box>
                  <Box
                    sx={{
                      "& p": { m: 0 },
                      "& p + p": { mt: 0.75 },
                      "& code": {
                        bgcolor: "background.default",
                        px: 0.5,
                        borderRadius: 0.5,
                        fontSize: "0.85em",
                      },
                      "& pre": {
                        bgcolor: "background.default",
                        p: 1,
                        borderRadius: 1,
                        overflowX: "auto",
                        fontSize: "0.85em",
                      },
                    }}
                    dangerouslySetInnerHTML={{ __html: safe }}
                  />
                </Paper>
              </Box>
            );
          }
          if (it.kind === "audit") {
            const a = it.data;
            return (
              <Box
                key={id}
                sx={{
                  display: "flex",
                  alignItems: "center",
                  gap: 1,
                  px: 1,
                  py: 0.5,
                  color: "text.secondary",
                }}
              >
                <Box sx={{ color: "text.disabled", display: "inline-flex" }}>
                  {AUDIT_ICON[a.kind] ?? <Activity size={14} />}
                </Box>
                <Typography variant="body2" color="text.secondary" sx={{ flex: 1 }}>
                  {a.description}
                </Typography>
                <Typography variant="caption" color="text.disabled">
                  {a.actor}
                </Typography>
                <Typography variant="caption" color="text.disabled" sx={{ minWidth: 110, textAlign: "right" }}>
                  <RelativeTime iso={a.createdAt} />
                </Typography>
              </Box>
            );
          }
          const a = it.data;
          return (
            <Box
              key={id}
              sx={{
                display: "flex",
                alignItems: "center",
                gap: 1.5,
                px: 1,
                py: 1,
                border: 1,
                borderColor: "divider",
                borderRadius: 1,
              }}
            >
              <Paperclip size={16} />
              <Box sx={{ flex: 1, minWidth: 0 }}>
                <Typography variant="body2" noWrap>
                  {a.filename}
                </Typography>
                <Typography variant="caption" color="text.secondary">
                  {formatBytes(a.size)} · uploaded by {a.uploadedBy}
                </Typography>
              </Box>
              <Typography variant="caption" color="text.secondary">
                <RelativeTime iso={a.uploadedAt} />
              </Typography>
            </Box>
          );
        })
      )}
    </Box>
  );
}
