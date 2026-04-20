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
  Skeleton,
  Stack,
  Typography,
  alpha,
  useTheme,
} from "@wso2/oxygen-ui";
import { useMemo, useEffect, useRef, useState, type JSX } from "react";
import useGetCaseComments from "@features/support/api/useGetCaseComments";
import { useGetConversationMessages } from "@features/support/api/useGetConversationMessages";
import useGetUserDetails from "@features/settings/api/useGetUserDetails";
import EmptyIcon from "@components/empty-state/EmptyIcon";
import { compareByCreatedOnThenId, formatCommentDate } from "@features/support/utils/support";
import ActivityCommentInput from "@case-details-activity/ActivityCommentInput";
import CommentBubble from "@case-details-activity/CommentBubble";
import ImageFullscreenModal from "@case-details-activity/ImageFullscreenModal";
import { hasDisplayableContent } from "@features/support/utils/support";
import type {
  ActivityContentProps,
  CaseDetailsActivityPanelProps,
} from "@features/support/types/supportComponents";
import type { CaseComment } from "@features/support/types/cases";
import ApiErrorState from "@components/error/ApiErrorState";

// TODO : DUE TO URGENCY THIS COMPONENT BREAKS THE BEST PRACTICES , NEED FULL REFACTOR
/**
 * Renders the Activity tab content: timeline of case comments (current user on right, others on left).
 *
 * @param {CaseDetailsActivityPanelProps} props - projectId, caseId, optional case created date.
 * @returns {JSX.Element} The activity timeline panel.
 */
export default function CaseDetailsActivityPanel({
  projectId,
  caseId,
  conversationId,
  caseCreatedOn,
  caseStatus,
}: CaseDetailsActivityPanelProps): JSX.Element {
  const theme = useTheme();
  const { data: userDetails } = useGetUserDetails();
  const {
    data: commentsData,
    isLoading: isCommentsLoading,
    isError: isCommentsError,
    error: commentsError,
  } = useGetCaseComments(projectId, caseId, { offset: 0, limit: 50 });
  const {
    data: conversationData,
    isLoading: isConversationLoading,
    isError: isConversationError,
    error: conversationError,
  } = useGetConversationMessages(conversationId ?? "", { pageSize: 50 });

  const currentUserEmail = userDetails?.email?.toLowerCase() ?? "";

  const mergedTimeline = useMemo(() => {
    const caseComments = commentsData?.comments ?? [];
    const conversationComments: CaseComment[] = (
      conversationData?.pages?.flatMap((page) => page.comments) ?? []
    ).map((comment) => ({
      ...comment,
      type: comment.type ?? "comments",
      isEscalated: comment.isEscalated ?? false,
    }));
    return [...caseComments, ...conversationComments].sort(compareByCreatedOnThenId);
  }, [commentsData?.comments, conversationData?.pages]);

  const commentsToShow = useMemo(
    () => mergedTimeline.filter(hasDisplayableContent),
    [mergedTimeline],
  );

  const primaryLight = theme.palette.primary?.light ?? "#fa7b3f";
  const primaryBg = alpha(primaryLight, 0.1);

  // Render content based on state
  let content: JSX.Element;

  const isLoading = isCommentsLoading || (!!conversationId && isConversationLoading);
  const isError = isCommentsError || (!!conversationId && isConversationError);
  const resolvedError = commentsError ?? conversationError;

  if (isLoading) {
    content = (
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
    );
  } else if (isError || !commentsData) {
    content = (
      <Box sx={{ p: 2, flex: 1, minHeight: 0, overflow: "auto" }}>
        <ApiErrorState
          error={resolvedError}
          fallbackMessage="Unable to load activity."
        />
      </Box>
    );
  } else {
    content = (
      <ActivityContentWithImageModal
        commentsToShow={commentsToShow}
        caseCreatedOn={caseCreatedOn}
        currentUserEmail={currentUserEmail}
        primaryBg={primaryBg}
        userDetails={userDetails}
      />
    );
  }

  return (
    <Box
      sx={{
        display: "flex",
        flexDirection: "column",
        minHeight: "100%",
      }}
    >
      {content}
      <ActivityCommentInput caseId={caseId} caseStatus={caseStatus} />
    </Box>
  );
}

function ActivityContentWithImageModal(
  props: ActivityContentProps,
): JSX.Element {
  const [fullscreenImageSrc, setFullscreenImageSrc] = useState<string | null>(
    null,
  );
  return (
    <>
      <ActivityContent
        {...props}
        onImageClick={(src) => setFullscreenImageSrc(src)}
      />
      <ImageFullscreenModal
        open={!!fullscreenImageSrc}
        imageSrc={fullscreenImageSrc}
        onClose={() => setFullscreenImageSrc(null)}
      />
    </>
  );
}

function ActivityContent({
  commentsToShow,
  caseCreatedOn,
  currentUserEmail,
  primaryBg,
  userDetails,
  onImageClick,
}: ActivityContentProps): JSX.Element {
  const scrollContainerRef = useRef<HTMLDivElement>(null);

  // Scroll to bottom on initial load
  useEffect(() => {
    if (scrollContainerRef.current) {
      scrollContainerRef.current.scrollTop =
        scrollContainerRef.current.scrollHeight;
    }
  }, []); // Empty dependency array - only run on mount

  // Scroll to bottom when new comments are added
  useEffect(() => {
    if (scrollContainerRef.current && commentsToShow.length > 0) {
      scrollContainerRef.current.scrollTop =
        scrollContainerRef.current.scrollHeight;
    }
  }, [commentsToShow.length]); // Run when comments count changes

  return (
    <Box
      ref={scrollContainerRef}
      sx={{
        p: 2,
        flex: 1,
        minHeight: 0,
        overflow: "auto",
        WebkitOverflowScrolling: "touch",
      }}
    >
      <Stack spacing={2}>
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
          commentsToShow.map((comment) => {
            const commentCreatorEmail = comment.createdBy?.toLowerCase() ?? "";
            const isCurrentUser = commentCreatorEmail === currentUserEmail;

            return (
              <CommentBubble
                key={comment.id}
                comment={comment}
                isCurrentUser={isCurrentUser}
                primaryBg={primaryBg}
                userDetails={userDetails}
                onImageClick={onImageClick}
              />
            );
          })
        )}
      </Stack>
    </Box>
  );
}
