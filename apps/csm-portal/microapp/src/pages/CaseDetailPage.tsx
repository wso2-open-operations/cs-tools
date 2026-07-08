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

import { Suspense, type ReactNode } from "react";
import { useNavigate, useParams } from "react-router-dom";
import { Divider, IconButton, Skeleton, Stack, Typography, pxToRem } from "@wso2/oxygen-ui";
import { ArrowLeft } from "@wso2/oxygen-ui-icons-react";
import { useQueryErrorResetBoundary, useSuspenseQuery } from "@tanstack/react-query";
import { cases } from "@src/services/cases";
import type { CaseDetail, Comment } from "@src/types";
import { ErrorBoundary } from "@components/common/ErrorBoundary";
import { SeverityChip, StatusChip } from "@components/support/Chips";
import { ErrorState } from "@components/support/ErrorState";
import { TYPE_CONFIG } from "@components/support/config";
import { formatDate } from "@utils/dateTime";

export default function CaseDetailPage() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();

  return (
    <Stack gap={2}>
      <Stack direction="row" alignItems="center" gap={1}>
        <IconButton onClick={() => navigate(-1)} size="small" aria-label="Go back">
          <ArrowLeft size={pxToRem(20)} />
        </IconButton>
        <Typography variant="h6">Case Details</Typography>
      </Stack>

      <CaseDetailErrorBoundary>
        <Suspense fallback={<CaseDetailSkeleton />}>
          <CaseDetailContent id={id ?? ""} />
        </Suspense>
      </CaseDetailErrorBoundary>
    </Stack>
  );
}

function CaseDetailContent({ id }: { id: string }) {
  const { data: caseDetail } = useSuspenseQuery(cases.get(id));
  const { data: comments } = useSuspenseQuery(cases.comments(id));

  return (
    <Stack gap={2}>
      <CaseSummarySection caseDetail={caseDetail} />
      <Divider />
      <CaseMetadataSection caseDetail={caseDetail} />
      <Divider />
      <CaseCommentsSection comments={comments} />
    </Stack>
  );
}

function CaseSummarySection({ caseDetail }: { caseDetail: CaseDetail }) {
  const { icon: Icon, color } = TYPE_CONFIG[caseDetail.type ?? "case"] ?? TYPE_CONFIG.case;

  return (
    <Stack gap={1}>
      <Stack direction="row" alignItems="center" gap={1}>
        <Icon size={pxToRem(18)} color={color} />
        <Typography variant="subtitle2" color="text.secondary">
          {caseDetail.number}
          {caseDetail.wso2Id ? ` · ${caseDetail.wso2Id}` : ""}
        </Typography>
      </Stack>

      <Typography variant="h6">{caseDetail.subject}</Typography>

      <Stack direction="row" gap={1} flexWrap="wrap">
        <StatusChip state={caseDetail.state} />
        <SeverityChip severity={caseDetail.severity} />
      </Stack>

      <Typography variant="body1" color="text.primary" sx={{ whiteSpace: "pre-wrap" }}>
        {caseDetail.description}
      </Typography>
    </Stack>
  );
}

function CaseMetadataSection({ caseDetail }: { caseDetail: CaseDetail }) {
  return (
    <Stack gap={1}>
      <DetailRow label="Project" value={caseDetail.project?.name} />
      <DetailRow label="Deployment" value={caseDetail.deployment?.name} />
      <DetailRow label="Product" value={caseDetail.product?.name} />
      <DetailRow label="Account" value={caseDetail.account?.name} />
      <DetailRow label="Assigned Engineer" value={caseDetail.assignedEngineer?.name} />
      <DetailRow label="Created By" value={caseDetail.createdBy?.displayName || caseDetail.createdBy?.email} />
      <DetailRow label="Created On" value={formatDate(caseDetail.createdOn)} />
      <DetailRow label="Updated On" value={formatDate(caseDetail.updatedOn)} />
      {caseDetail.closedOn && <DetailRow label="Closed On" value={formatDate(caseDetail.closedOn)} />}
    </Stack>
  );
}

function DetailRow({ label, value }: { label: string; value?: string | null }) {
  if (!value) return null;

  return (
    <Stack direction="row" justifyContent="space-between" gap={2}>
      <Typography variant="body2" color="text.secondary">
        {label}
      </Typography>
      <Typography variant="body2" color="text.primary" textAlign="right">
        {value}
      </Typography>
    </Stack>
  );
}

function CaseCommentsSection({ comments }: { comments: Comment[] }) {
  return (
    <Stack gap={1.5}>
      <Typography variant="h6">Comments</Typography>

      {comments.length === 0 && (
        <Typography variant="body2" color="text.secondary">
          No comments yet.
        </Typography>
      )}

      {comments.map((comment) => (
        <Stack key={comment.id} gap={0.5} sx={{ p: 1.5, border: "1px solid", borderColor: "divider", borderRadius: 1 }}>
          <Stack direction="row" justifyContent="space-between" gap={2}>
            <Typography variant="subtitle2" color="text.secondary" noWrap>
              {comment.createdBy}
            </Typography>
            <Typography variant="subtitle2" color="text.secondary" noWrap>
              {formatDate(comment.createdOn)}
            </Typography>
          </Stack>
          <Typography variant="body2" color="text.primary" sx={{ whiteSpace: "pre-wrap" }}>
            {comment.content}
          </Typography>
        </Stack>
      ))}
    </Stack>
  );
}

function CaseDetailSkeleton() {
  return (
    <Stack gap={2}>
      <Skeleton variant="text" width="40%" height={24} />
      <Skeleton variant="text" width="80%" height={32} />
      <Skeleton variant="rounded" height={100} />
      <Skeleton variant="rounded" height={150} />
    </Stack>
  );
}

function CaseDetailErrorBoundary({ children }: { children: ReactNode }) {
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
