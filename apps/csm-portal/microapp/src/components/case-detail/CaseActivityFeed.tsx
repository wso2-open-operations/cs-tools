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

import { Suspense, useMemo, type ReactNode } from "react";
import { Chip, IconButton, Skeleton, Stack, Typography } from "@wso2/oxygen-ui";
import { Eye, History, Paperclip } from "@wso2/oxygen-ui-icons-react";
import { useQueryErrorResetBoundary, useSuspenseQuery } from "@tanstack/react-query";
import { activities as activitiesService } from "@src/services/activities";
import { attachments as attachmentsService } from "@src/services/attachments";
import type { CaseAttachment, CaseAuditEntry, Comment } from "@src/types";
import { ErrorBoundary } from "@components/common/ErrorBoundary";
import { ErrorState } from "@components/support/ErrorState";
import { formatBytes } from "@utils/attachments";
import { fromNow } from "@utils/dateTime";
import { openUrl } from "@components/microapp-bridge";
import { CommentBody } from "./CommentBody";

interface CaseActivityFeedProps {
  comments: Comment[];
  audit: CaseAuditEntry[];
  attachments: CaseAttachment[];
}

type FeedEntry =
  | { kind: "comment"; at: Date; id: string; comment: Comment }
  | { kind: "audit"; at: Date; id: string; entry: CaseAuditEntry }
  | { kind: "attachment"; at: Date; id: string; attachment: CaseAttachment };

// Ascending order, id tie-break for determinism when two entries share a timestamp. Negate for
// newest-first, mirroring the webapp's caseActivityFeed.ts compareFeedEntries.
function compareFeedEntries(a: FeedEntry, b: FeedEntry): number {
  const t = a.at.getTime() - b.at.getTime();
  return t !== 0 ? t : a.id.localeCompare(b.id);
}

/** One "<label>: <old> → <new>" line for a field-change entry. */
function FieldChangeLine({ field }: { field: CaseAuditEntry["changes"][number] }) {
  const hadPrevious = !!field.previousValue?.trim();
  const hasNew = !!field.newValue?.trim();
  return (
    <Typography variant="body2" component="div">
      <strong>{field.fieldLabel}:</strong>{" "}
      {hadPrevious && (
        <>
          <Typography component="span" variant="body2" color="text.secondary">
            {field.previousValue}
          </Typography>
          {" → "}
        </>
      )}
      {hasNew ? field.newValue : <em>cleared</em>}
    </Typography>
  );
}

/**
 * Unified Activities feed — merges comments, lifecycle/field-change audit entries, and attachment
 * uploads into one chronological (newest-first) timeline. Mirrors the webapp's CaseActivitiesFeed
 * (apps/csm-portal/webapp/src/features/csm-cases/components/CaseActivitiesFeed.tsx), minus its
 * category filter/sort-order chips — everything is always shown here.
 */
export function CaseActivityFeed({ comments, audit, attachments }: CaseActivityFeedProps) {
  const entries = useMemo(() => {
    const out: FeedEntry[] = [];
    for (const c of comments) out.push({ kind: "comment", at: c.createdOn, id: c.id, comment: c });
    for (const a of audit) out.push({ kind: "audit", at: a.createdOn, id: a.id, entry: a });
    for (const a of attachments) out.push({ kind: "attachment", at: a.createdOn, id: a.id, attachment: a });
    out.sort((a, b) => -compareFeedEntries(a, b));
    return out;
  }, [comments, audit, attachments]);

  return (
    <Stack gap={1.5}>
      {entries.length === 0 && (
        <Typography variant="body2" color="text.secondary">
          Nothing to show yet.
        </Typography>
      )}

      {entries.map((e) => {
        if (e.kind === "comment") {
          return (
            <Stack
              key={`c-${e.id}`}
              gap={0.5}
              sx={{ p: 1.5, border: "1px solid", borderColor: "divider", borderRadius: 1 }}
            >
              <Stack direction="row" justifyContent="space-between" alignItems="center" gap={1} flexWrap="wrap">
                <Stack direction="row" alignItems="center" gap={1} sx={{ minWidth: 0 }}>
                  <Typography variant="subtitle2" color="text.secondary" noWrap sx={{ minWidth: 0 }}>
                    {e.comment.createdBy}
                  </Typography>
                  {e.comment.type === "work_note" && <Chip size="small" variant="outlined" label="Work note" />}
                </Stack>
                <Typography variant="subtitle2" color="text.secondary" noWrap sx={{ flexShrink: 0 }}>
                  {fromNow(e.comment.createdOn)}
                </Typography>
              </Stack>
              <CommentBody content={e.comment.content} />
            </Stack>
          );
        }

        if (e.kind === "audit") {
          return (
            <Stack
              key={`a-${e.id}`}
              gap={0.75}
              sx={{ p: 1.5, border: "1px solid", borderColor: "divider", borderRadius: 1 }}
            >
              <Stack direction="row" alignItems="center" gap={1} flexWrap="wrap">
                <History size={14} />
                <Typography variant="subtitle2">{e.entry.actor}</Typography>
                <Chip size="small" variant="outlined" label="Lifecycle" />
                <Typography variant="caption" color="text.secondary" sx={{ ml: "auto" }}>
                  {fromNow(e.entry.createdOn)}
                </Typography>
              </Stack>
              <Stack gap={0.25}>
                {e.entry.changes.map((change, i) => (
                  <FieldChangeLine key={`${change.field}-${i}`} field={change} />
                ))}
              </Stack>
            </Stack>
          );
        }

        return (
          <Stack
            key={`f-${e.id}`}
            direction="row"
            alignItems="center"
            justifyContent="space-between"
            gap={1}
            sx={{ p: 1.5, border: "1px solid", borderColor: "divider", borderRadius: 1 }}
          >
            <Stack direction="row" alignItems="center" gap={1} sx={{ minWidth: 0 }}>
              <Paperclip size={16} />
              <Stack sx={{ minWidth: 0 }}>
                <Stack direction="row" alignItems="center" gap={1}>
                  <Typography variant="subtitle2" noWrap>
                    {e.attachment.createdBy}
                  </Typography>
                  <Chip size="small" variant="outlined" label="Attachment" />
                </Stack>
                <Typography variant="body2" noWrap sx={{ fontWeight: 600 }}>
                  {e.attachment.name}
                </Typography>
                <Typography variant="caption" color="text.secondary" noWrap>
                  {formatBytes(e.attachment.sizeBytes)} · {fromNow(e.attachment.createdOn)}
                </Typography>
              </Stack>
            </Stack>
            {e.attachment.downloadUrl && (
              // Only "Open" — the native bridge exposes a single `openUrl` action (open in the
              // in-app browser), no distinct "save to device" primitive. A "Download" button
              // that called the same thing was misleading rather than offering real download
              // behavior.
              <IconButton
                size="small"
                aria-label={`Open ${e.attachment.name}`}
                onClick={() => openUrl({ url: e.attachment.downloadUrl as string, presentationStyle: "fullScreen" })}
                sx={{ flexShrink: 0 }}
              >
                <Eye size={16} />
              </IconButton>
            )}
          </Stack>
        );
      })}
    </Stack>
  );
}

/**
 * Tab-scoped fetch + Suspense/error boundary for the audit and attachment lanes, mirroring the
 * other case-detail tabs (SlaTab, AttachmentsTab, CallRequestsTab): comments are already loaded by
 * the parent for the composer, so only the two supplemental lists are fetched here, keeping their
 * loading/error states scoped to this tab instead of blocking the whole page.
 */
export function CaseActivitiesTab({ caseId, comments }: { caseId: string; comments: Comment[] }) {
  return (
    <CaseActivitiesTabErrorBoundary>
      <Suspense fallback={<CaseActivitiesTabSkeleton />}>
        <CaseActivitiesTabContent caseId={caseId} comments={comments} />
      </Suspense>
    </CaseActivitiesTabErrorBoundary>
  );
}

function CaseActivitiesTabContent({ caseId, comments }: { caseId: string; comments: Comment[] }) {
  const { data: audit } = useSuspenseQuery(activitiesService.forCase(caseId));
  const { data: caseAttachments } = useSuspenseQuery(attachmentsService.forCase(caseId));
  return <CaseActivityFeed comments={comments} audit={audit} attachments={caseAttachments} />;
}

function CaseActivitiesTabSkeleton() {
  return (
    <Stack gap={1.5}>
      <Skeleton variant="rounded" height={56} />
      <Skeleton variant="rounded" height={56} />
    </Stack>
  );
}

function CaseActivitiesTabErrorBoundary({ children }: { children: ReactNode }) {
  const { reset } = useQueryErrorResetBoundary();

  return (
    <ErrorBoundary
      fallback={(_error, resetErrorBoundary) => (
        <ErrorState
          onRetry={() => {
            reset();
            resetErrorBoundary();
          }}
        />
      )}
    >
      {children}
    </ErrorBoundary>
  );
}
