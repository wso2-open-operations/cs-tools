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

import { Box, Button, Card, Chip, Skeleton, Typography } from "@wso2/oxygen-ui";
import { ArrowLeft } from "@wso2/oxygen-ui-icons-react";
import { useEffect, type JSX } from "react";
import { useNavigate, useParams } from "react-router";
import { useGetCsmCaseDetail } from "@features/csm-cases/api/useGetCsmCaseDetail";
import {
  useGetCsmCaseComments,
  usePostCsmCaseComment,
} from "@features/csm-cases/api/useCsmCaseComments";
import CsmCaseCommentBubble from "@features/csm-cases/components/CsmCaseCommentBubble";
import CsmCaseCommentInput from "@features/csm-cases/components/CsmCaseCommentInput";
import { useRecordRecentView } from "@features/csm-recent/hooks/useRecentViews";
import {
  SEVERITY_COLOR,
  SLA_CLOCK_LABEL,
  STATE_LABEL,
  formatRelativeTime,
  formatTimeToBreach,
} from "@features/csm-dashboard/utils/abtDashboard";

function MetaCell({
  label,
  children,
}: {
  label: string;
  children: React.ReactNode;
}): JSX.Element {
  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 0.25, minWidth: 0 }}>
      <Typography variant="caption" color="text.secondary">
        {label}
      </Typography>
      <Box>{children}</Box>
    </Box>
  );
}

// TODO: replace with the engineer name from useGetUserDetails once the
// `firstName`/`lastName` shape from the CSM backend is wired. For mocks we
// match the dashboard's ABT engineer.
const CURRENT_ENGINEER_NAME = "Sajith Ekanayaka";

export default function CsmCaseDetailPage(): JSX.Element {
  const { caseId } = useParams<{ caseId: string }>();
  const navigate = useNavigate();
  const { data, isLoading, isError } = useGetCsmCaseDetail(caseId);
  const {
    data: comments,
    isLoading: isCommentsLoading,
    isError: isCommentsError,
  } = useGetCsmCaseComments(caseId);
  const postComment = usePostCsmCaseComment();
  const recordView = useRecordRecentView();

  useEffect(() => {
    if (!data) return;
    recordView({
      kind: "case",
      id: data.id,
      title: `${data.caseNumber} · ${data.subject}`,
      subtitle: `${data.customer} · ${data.projectName}`,
      href: `/cases/${data.id}`,
    });
  }, [data, recordView]);

  if (isLoading) {
    return (
      <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
        <Skeleton variant="rectangular" height={32} width={240} />
        <Skeleton variant="rectangular" height={200} />
      </Box>
    );
  }

  if (isError) {
    return (
      <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
        <Button
          variant="text"
          size="small"
          startIcon={<ArrowLeft size={16} />}
          onClick={() => navigate("/cases")}
          sx={{ alignSelf: "flex-start" }}
        >
          Back to cases
        </Button>
        <Typography variant="body1" color="error">
          Could not load case {caseId}.
        </Typography>
      </Box>
    );
  }

  if (!data) {
    return (
      <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
        <Button
          variant="text"
          size="small"
          startIcon={<ArrowLeft size={16} />}
          onClick={() => navigate("/cases")}
          sx={{ alignSelf: "flex-start" }}
        >
          Back to cases
        </Button>
        <Typography variant="h5">Case not found</Typography>
        <Typography variant="body2" color="text.secondary">
          No case with id <code>{caseId}</code> in the current mock dataset.
        </Typography>
      </Box>
    );
  }

  const c = data;
  const isClosed = c.state === "closed";
  const breached = c.minutesToBreach < 0;

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
      <Button
        variant="text"
        size="small"
        startIcon={<ArrowLeft size={16} />}
        onClick={() => navigate("/cases")}
        sx={{ alignSelf: "flex-start" }}
      >
        Back to cases
      </Button>

      <Box sx={{ display: "flex", flexDirection: "column", gap: 1 }}>
        <Box sx={{ display: "flex", alignItems: "center", gap: 1.5, flexWrap: "wrap" }}>
          <Typography variant="overline" color="text.secondary">
            {c.caseNumber}
          </Typography>
          <Chip size="small" label={c.severity} color={SEVERITY_COLOR[c.severity]} />
          <Chip
            size="small"
            label={STATE_LABEL[c.state]}
            variant="outlined"
            color={isClosed ? "success" : "primary"}
          />
          {!isClosed && (
            <Chip
              size="small"
              variant="outlined"
              color={breached ? "error" : c.minutesToBreach <= 60 ? "warning" : "default"}
              label={`${SLA_CLOCK_LABEL[c.slaClockType]} · ${formatTimeToBreach(c.minutesToBreach)}`}
            />
          )}
        </Box>
        <Typography variant="h5">{c.subject}</Typography>
        <Typography variant="body2" color="text.secondary">
          {c.customer} · {c.projectName}
        </Typography>
      </Box>

      <Card sx={{ p: 2.5, display: "flex", flexDirection: "column", gap: 2 }}>
        <Typography variant="subtitle2">Summary</Typography>
        <Box
          sx={{
            display: "grid",
            gridTemplateColumns: {
              xs: "1fr 1fr",
              md: "repeat(4, minmax(0, 1fr))",
            },
            gap: 2,
          }}
        >
          <MetaCell label="Customer">
            <Typography variant="body2">{c.customer}</Typography>
          </MetaCell>
          <MetaCell label="Project">
            <Typography variant="body2">{c.projectName}</Typography>
          </MetaCell>
          <MetaCell label="Owner">
            <Typography variant="body2">
              {c.ownerIsMe ? <strong>{c.owner}</strong> : c.owner}
            </Typography>
          </MetaCell>
          <MetaCell label="State">
            <Typography variant="body2">{STATE_LABEL[c.state]}</Typography>
          </MetaCell>
          <MetaCell label="Severity">
            <Typography variant="body2">{c.severity}</Typography>
          </MetaCell>
          <MetaCell label="SLA clock">
            <Typography variant="body2">{SLA_CLOCK_LABEL[c.slaClockType]}</Typography>
          </MetaCell>
          <MetaCell label="Created">
            <Typography variant="body2">{formatRelativeTime(c.createdAt)}</Typography>
          </MetaCell>
          <MetaCell label="Last update">
            <Typography variant="body2">{formatRelativeTime(c.updatedAt)}</Typography>
          </MetaCell>
        </Box>
      </Card>

      <Card sx={{ p: 2.5, display: "flex", flexDirection: "column", gap: 2 }}>
        <Box sx={{ display: "flex", alignItems: "center", gap: 1.5 }}>
          <Typography variant="subtitle2">Comments &amp; activity</Typography>
          {!isCommentsLoading && (
            <Chip
              size="small"
              variant="outlined"
              label={`${comments?.length ?? 0} ${(comments?.length ?? 0) === 1 ? "entry" : "entries"}`}
            />
          )}
        </Box>

        <Box sx={{ display: "flex", flexDirection: "column", gap: 1.5 }}>
          {isCommentsLoading &&
            [0, 1].map((i) => (
              <Skeleton key={i} variant="rectangular" height={64} />
            ))}
          {isCommentsError && !isCommentsLoading && (
            <Typography variant="body2" color="error">
              Could not load comments.
            </Typography>
          )}
          {!isCommentsLoading && !isCommentsError && (comments?.length ?? 0) === 0 && (
            <Typography variant="body2" color="text.secondary">
              No comments yet. Start the thread below.
            </Typography>
          )}
          {!isCommentsLoading &&
            comments?.map((c) => (
              <CsmCaseCommentBubble key={c.id} comment={c} />
            ))}
        </Box>

        <Box sx={{ pt: 1.5, borderTop: 1, borderColor: "divider" }}>
          <CsmCaseCommentInput
            disabled={!caseId}
            onSubmit={async (bodyHtml) => {
              if (!caseId) return;
              await postComment.mutateAsync({
                caseId,
                bodyHtml,
                authorName: CURRENT_ENGINEER_NAME,
              });
            }}
          />
        </Box>
      </Card>
    </Box>
  );
}
