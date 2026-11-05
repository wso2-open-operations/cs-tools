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

import { Box, Button, Divider, IconButton, Skeleton, Stack, Typography } from "@wso2/oxygen-ui";
import { X } from "@wso2/oxygen-ui-icons-react";
import { useMemo, type JSX, type ReactNode } from "react";
import { Link as RouterLink } from "react-router";
import RelativeTime from "@components/RelativeTime";
import CaseSeverityChip from "@features/csm-cases/components/CaseSeverityChip";
import StateChip from "@components/StateChip";
import WorkStateChip from "@components/WorkStateChip";
import CsmCaseCommentBubble from "@features/csm-cases/components/CsmCaseCommentBubble";
import { useGetCsmCaseComments } from "@features/csm-cases/api/useCsmCaseComments";
import { caseTypeDetailBasePath } from "@features/csm-cases/utils/caseType";
import { effectiveWorkState } from "@features/csm-cases/utils/caseWorkState";
import type { CsmCaseRow } from "@features/csm-cases/types/csmCases";

// "The last few at least" — deliberately not the full thread (that's the
// whole reason this exists instead of just linking to the case): a quick
// look, not a second copy of the Activities tab.
const RECENT_COMMENTS_LIMIT = 5;

interface CasePreviewContentProps {
  row: CsmCaseRow;
  onClose: () => void;
  /** Skips the panel's own "Close preview" (✕) button — for a caller that
   * embeds this beneath its own header with its own close control already
   * (see `TimeCardCasePreviewDrawer`), so the drawer doesn't end up with two
   * ✕ buttons doing the same thing. `onClose` is still used for the "View
   * full details" button either way. */
  hideCloseButton?: boolean;
}

function MetaField({ label, children }: { label: string; children: ReactNode }): JSX.Element {
  return (
    <Box sx={{ minWidth: 0 }}>
      <Typography variant="caption" color="text.secondary">
        {label}
      </Typography>
      <Typography variant="body2" noWrap>
        {children}
      </Typography>
    </Box>
  );
}

/**
 * The actual "quick look" content for a case — subject/state/severity, a meta
 * grid, and the last handful of comments, plus a "View full details" escape
 * hatch. Extracted from `CasePreviewDrawer` so it can be embedded inside a
 * different drawer shell too (see `TimeCardCasePreviewDrawer`, which stacks
 * this beneath a time card's own summary) without duplicating the case-side
 * fetch/render logic. `CasePreviewDrawer` itself is just this wrapped in a
 * `Drawer` — no behavior changed by the extraction.
 */
export default function CasePreviewContent({
  row,
  onClose,
  hideCloseButton = false,
}: CasePreviewContentProps): JSX.Element {
  const { data: comments, isLoading, isError } = useGetCsmCaseComments(row.id);

  const recentComments = useMemo(() => {
    if (!comments) return [];
    return [...comments]
      .sort((a, b) => new Date(b.createdAt).getTime() - new Date(a.createdAt).getTime())
      .slice(0, RECENT_COMMENTS_LIMIT);
  }, [comments]);

  const detailPath = `${caseTypeDetailBasePath(row.caseType)}/${row.id}`;

  return (
    <Box sx={{ display: "flex", flexDirection: "column", height: "100%" }}>
      <Box sx={{ p: 2.5, display: "flex", flexDirection: "column", gap: 1.5 }}>
        <Box sx={{ display: "flex", alignItems: "flex-start", justifyContent: "space-between", gap: 1 }}>
          <Box sx={{ minWidth: 0 }}>
            <Typography variant="subtitle2" color="text.secondary" noWrap>
              {row.wso2CaseId}
              {row.wso2CaseId && row.caseNumber ? " · " : ""}
              {row.caseNumber}
            </Typography>
            <Typography variant="h6" sx={{ wordBreak: "break-word" }}>
              {row.subject}
            </Typography>
          </Box>
          {!hideCloseButton && (
            <IconButton size="small" onClick={onClose} aria-label="Close preview">
              <X size={18} />
            </IconButton>
          )}
        </Box>
        <Box sx={{ display: "flex", gap: 1, flexWrap: "wrap" }}>
          <StateChip state={row.state} variant="outlined" />
          {row.state === "work_in_progress" && (
            <WorkStateChip workState={effectiveWorkState(row.workState)} />
          )}
          <CaseSeverityChip caseType={row.caseType} severity={row.severity} emptyFallback={null} />
        </Box>
      </Box>

      <Divider />

      <Box sx={{ flex: 1, overflowY: "auto", p: 2.5, display: "flex", flexDirection: "column", gap: 2.5 }}>
        <Box sx={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 2 }}>
          <MetaField label="Project">{row.projectName || "—"}</MetaField>
          <MetaField label="Product">{row.product || "—"}</MetaField>
          <MetaField label="Assignee">{row.assignee || "Unassigned"}</MetaField>
          <MetaField label="Updated">
            <RelativeTime iso={row.updatedAt} />
          </MetaField>
        </Box>

        <Box>
          <Typography variant="subtitle2" sx={{ mb: 1.5 }}>
            Recent activity
          </Typography>
          {isLoading ? (
            <Stack gap={1.5}>
              <Skeleton variant="rounded" height={64} />
              <Skeleton variant="rounded" height={64} />
            </Stack>
          ) : isError ? (
            <Typography variant="body2" color="error">
              Could not load comments.
            </Typography>
          ) : recentComments.length === 0 ? (
            <Typography variant="body2" color="text.secondary">
              No comments yet.
            </Typography>
          ) : (
            <Stack gap={1.5}>
              {recentComments.map((comment) => (
                <CsmCaseCommentBubble key={comment.id} comment={comment} compact />
              ))}
            </Stack>
          )}
        </Box>
      </Box>

      <Divider />

      <Box sx={{ p: 2 }}>
        <Button component={RouterLink} to={detailPath} variant="contained" fullWidth onClick={onClose}>
          View full details
        </Button>
      </Box>
    </Box>
  );
}
