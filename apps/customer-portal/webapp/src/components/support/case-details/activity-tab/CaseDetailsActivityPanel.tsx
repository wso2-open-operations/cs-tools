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

import { Box, Skeleton, Stack, Typography, alpha, useTheme } from "@wso2/oxygen-ui";
import { useMemo, type JSX } from "react";
import useGetCaseComments from "@api/useGetCaseComments";
import useGetUserDetails from "@api/useGetUserDetails";
import type { CaseComment } from "@models/responses";
import EmptyIcon from "@components/common/empty-state/EmptyIcon";
import { formatCommentDate } from "@utils/support";
import ActivityCommentInput from "@case-details-activity/ActivityCommentInput";
import CommentBubble from "@case-details-activity/CommentBubble";
import { hasDisplayableContent } from "@utils/support";

// TODO : DUE TO URGENCY THIS COMPONENT BREAKS THE BEST PRACTICES , NEED FULL REFACTOR
export interface CaseDetailsActivityPanelProps {
  projectId: string;
  caseId: string;
  caseCreatedOn?: string | null;
}

/**
 * Renders the Activity tab content: timeline of case comments (current user on right, others on left).
 *
 * @param {CaseDetailsActivityPanelProps} props - projectId, caseId, optional case created date.
 * @returns {JSX.Element} The activity timeline panel.
 */
export default function CaseDetailsActivityPanel({
  projectId,
  caseId,
  caseCreatedOn,
}: CaseDetailsActivityPanelProps): JSX.Element {
  const theme = useTheme();
  const { data: userDetails } = useGetUserDetails();
  const {
    data: commentsData,
    isLoading,
    isError,
  } = useGetCaseComments(projectId, caseId, { offset: 0, limit: 50 });

  const currentUserEmail = userDetails?.email?.toLowerCase() ?? "";

  const commentsSorted = useMemo(() => {
    const list = commentsData?.comments ?? [];
    return [...list].sort(
      (a, b) =>
        new Date(a.createdOn).getTime() - new Date(b.createdOn).getTime(),
    );
  }, [commentsData?.comments]);

  const commentsToShow = useMemo(
    () => commentsSorted.filter(hasDisplayableContent),
    [commentsSorted],
  );

  if (isLoading) {
    return (
      <Box
        sx={{
          display: "flex",
          flexDirection: "column",
          minHeight: "100%",
        }}
      >
        <Box sx={{ p: 2, flex: 1, minHeight: 0, overflow: "auto" }}>
          <Stack spacing={2}>
            {[1, 2, 3].map((i) => (
              <Stack
                key={i}
                direction="row"
                spacing={1.5}
                alignItems="flex-start"
              >
                <Skeleton variant="circular" width={32} height={32} />
                <Box sx={{ flex: 1 }}>
                  <Skeleton variant="text" width="40%" height={20} />
                  <Skeleton variant="rectangular" height={60} sx={{ mt: 1 }} />
                </Box>
              </Stack>
            ))}
          </Stack>
        </Box>
        <ActivityCommentInput caseId={caseId} />
      </Box>
    );
  }

  if (isError || !commentsData) {
    return (
      <Box
        sx={{
          display: "flex",
          flexDirection: "column",
          minHeight: "100%",
        }}
      >
        <Box sx={{ p: 2, flex: 1, minHeight: 0, overflow: "auto" }}>
          <Typography variant="body2" color="text.secondary">
            Unable to load activity.
          </Typography>
        </Box>
        <ActivityCommentInput caseId={caseId} />
      </Box>
    );
  }

  const primaryLight = theme.palette.primary?.light ?? "#fa7b3f";
  const primaryBg = alpha(primaryLight, 0.1);

  return (
    <Box
      sx={{
        display: "flex",
        flexDirection: "column",
        minHeight: "100%",
      }}
    >
      <ActivityContent
        commentsToShow={commentsToShow}
        caseCreatedOn={caseCreatedOn}
        currentUserEmail={currentUserEmail}
        primaryBg={primaryBg}
        userDetails={userDetails}
      />
      <ActivityCommentInput caseId={caseId} />
    </Box>
  );
}

interface ActivityContentProps {
  commentsToShow: CaseComment[];
  caseCreatedOn?: string | null;
  currentUserEmail: string;
  primaryBg: string;
  userDetails?: { email?: string; firstName?: string; lastName?: string } | null;
}

function ActivityContent({
  commentsToShow,
  caseCreatedOn,
  currentUserEmail,
  primaryBg,
  userDetails,
}: ActivityContentProps): JSX.Element {
  return (
    <Box
        sx={{
          p: 2,
          flex: 1,
          minHeight: 0,
          overflow: "auto",
          WebkitOverflowScrolling: "touch",
        }}
      >
        <Stack spacing={3}>
          {caseCreatedOn && (
            <Box
              sx={{
                display: "flex",
                alignItems: "center",
                gap: 1,
              }}
            >
              <Box
                sx={{
                  flex: 1,
                  height: 1,
                  bgcolor: "divider",
                }}
              />
              <Typography variant="caption" color="text.secondary">
                Case created on {formatCommentDate(caseCreatedOn)}
              </Typography>
              <Box
                sx={{
                  flex: 1,
                  height: 1,
                  bgcolor: "divider",
                }}
              />
            </Box>
          )}

          {commentsToShow.length === 0 ? (
            <Stack
              spacing={2}
              alignItems="center"
              justifyContent="center"
              sx={{ py: 4 }}
            >
              <Box
                sx={{
                  width: 160,
                  maxWidth: "100%",
                  "& svg": { width: "100%", height: "auto" },
                }}
                aria-hidden
              >
                <EmptyIcon />
              </Box>
              <Typography variant="body2" color="text.secondary">
                No activity yet.
              </Typography>
            </Stack>
          ) : (
            commentsToShow.map((comment) => (
              <CommentBubble
                key={comment.id}
                comment={comment}
                isCurrentUser={
                  comment.createdBy?.toLowerCase() === currentUserEmail
                }
                primaryBg={primaryBg}
                userDetails={userDetails}
              />
            ))
          )}
        </Stack>
      </Box>
  );
}
