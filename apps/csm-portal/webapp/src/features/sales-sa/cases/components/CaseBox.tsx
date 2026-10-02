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

// Ported from apps/support-portal-lite/webapp's own
// features/spl/cases/components/CaseBox.tsx — the comments/worknotes feed
// shown under a case's work-note composer. Rewritten against
// useGetCaseComments (React Query, keyed on [caseId, offset]) instead of
// useSplApi's imperative getApiData(url); the "accumulate pages as offset
// grows" client-side logic is otherwise unchanged from the source.
import { useEffect, useState } from "react";
import { Box, Button, Card, CardContent, Stack, Typography } from "@wso2/oxygen-ui";
import { useTheme } from "@mui/material/styles";
import { CircleAlertIcon, ClockIcon, StickyNoteIcon } from "@wso2/oxygen-ui-icons-react";
import DOMPurify from "dompurify";
import { usePermissions } from "@features/sales-sa/api/permissionsContext";
import { useInlineAttachmentImages } from "../utils/useInlineAttachmentImages";
import { useGetCaseComments } from "../api/useCases";
import type { CaseCommentDetails } from "../api/caseTypes";
import { LinearLoadingPanel } from "./StatePanels";

function getCommentKey(comment: CaseCommentDetails): string {
  return comment.id;
}

// entity-service's own createdOn (via the accounts/projects/cases merge --
// see cs-tools' main.go route registration comment) is already a full
// RFC3339 timestamp with its own "Z"/offset; the old ServiceNow-backed value
// this was written against was a bare "YYYY-MM-DDTHH:mm:ss" needing one
// appended. Only append when it's actually missing one, so entity-service's
// value isn't turned into an invalid "...ssssssZZ" string.
function withTimezone(dateTime: string): string {
  return /[Zz]$|[+-]\d{2}:?\d{2}$/.test(dateTime) ? dateTime : dateTime + "Z";
}

function getTimeDifference(createdOn: string): string {
  const createdDate = new Date(withTimezone(createdOn));
  const now = new Date();
  const diffMinutes = Math.floor((now.getTime() - createdDate.getTime()) / 60000);
  const diffHours = Math.floor(diffMinutes / 60);
  const diffDays = Math.floor(diffHours / 24);
  const diffMonths = Math.floor(diffDays / 30);
  const diffYears = Math.floor(diffDays / 365.25);

  if (diffMinutes < 60) return `${diffMinutes} minute${diffMinutes === 1 ? "" : "s"} ago`;
  if (diffHours < 24) return `${diffHours} hour${diffHours === 1 ? "" : "s"} ago`;
  if (diffDays < 30) return `${diffDays} day${diffDays === 1 ? "" : "s"} ago`;
  if (diffMonths < 12) return `${diffMonths} month${diffMonths === 1 ? "" : "s"} ago`;
  return `${diffYears} year${diffYears === 1 ? "" : "s"} ago`;
}

export function CaseBox({ caseId }: { caseId: string | undefined }) {
  const theme = useTheme();
  const [offset, setOffset] = useState(0);
  const [endOfComments, setEndOfComments] = useState(false);
  const [allComments, setAllComments] = useState<CaseCommentDetails[]>([]);

  const { data, isLoading, isFetching, error } = useGetCaseComments(caseId ?? "", offset, 10);

  // Resets pagination whenever the case changes.
  useEffect(() => {
    setOffset(0);
    setEndOfComments(false);
    setAllComments([]);
  }, [caseId]);

  useEffect(() => {
    if (!data?.comments) return;
    const { comments, total } = data;
    if (offset === 0) {
      setAllComments(comments);
      setEndOfComments(comments.length >= total);
    } else if (comments.length > 0) {
      setAllComments((prev) => {
        const existingKeys = new Set(prev.map(getCommentKey));
        const newComments = comments.filter((c) => !existingKeys.has(getCommentKey(c)));
        setEndOfComments(prev.length + newComments.length >= total);
        return [...prev, ...newComments];
      });
    } else {
      setEndOfComments(true);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [data]);

  const nextComments = () => setOffset((o) => o + 10);

  if (isLoading && allComments.length === 0) return <LinearLoadingPanel />;
  if (error) {
    return (
      <Stack direction="row" alignItems="center" spacing={1} sx={{ p: 4 }}>
        <CircleAlertIcon size={28} color={theme.palette.error.main} />
        <Typography variant="h6" color="error">
          error while loading comments and worknotes!
        </Typography>
      </Stack>
    );
  }

  return (
    <>
      {allComments.map((item, index) => (
        <CommentEntry key={index} item={item} />
      ))}
      {!endOfComments && allComments.length > 0 && (
        <Box sx={{ display: "flex", justifyContent: "center", pt: 3 }}>
          <Button variant="outlined" onClick={nextComments} disabled={isFetching}>
            Show More
          </Button>
        </Box>
      )}
    </>
  );
}

function CommentEntry({ item }: { item: CaseCommentDetails }) {
  const { canDownloadAttachments } = usePermissions();
  const sanitizedValue = DOMPurify.sanitize(item.value);
  const { resolvedHtml } = useInlineAttachmentImages(sanitizedValue, canDownloadAttachments);

  return (
    <Card
      variant="outlined"
      sx={{ mt: 2, ...(item.type === "work_notes" ? { borderLeft: "4px solid #ff7300" } : {}) }}
    >
      <CardContent>
        <Stack direction="row" spacing={1} alignItems="center" sx={{ color: "text.secondary", mb: 1 }}>
          {item.type === "comments" ? <ClockIcon size={16} /> : <StickyNoteIcon size={16} />}
          <Typography variant="caption">
            {new Date(withTimezone(item.createdOn)).toLocaleString()} ({getTimeDifference(item.createdOn)})
          </Typography>
          <Typography variant="caption" fontWeight={600}>
            {item.createdBy}
          </Typography>
        </Stack>
        <Box
          sx={{ overflow: "auto" }}
          dangerouslySetInnerHTML={{ __html: DOMPurify.sanitize(resolvedHtml) }}
        />
      </CardContent>
    </Card>
  );
}
