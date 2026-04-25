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
  IconButton,
  Skeleton,
  Stack,
  Tooltip,
  Typography,
  alpha,
  useTheme,
} from "@wso2/oxygen-ui";
import { ChevronUp } from "@wso2/oxygen-ui-icons-react";
import { useMemo, useRef, useState, type JSX } from "react";
import useGetCaseComments from "@features/support/api/useGetCaseComments";
import { useGetConversationMessages } from "@features/support/api/useGetConversationMessages";
import useGetUserDetails from "@features/settings/api/useGetUserDetails";
import EmptyIcon from "@components/empty-state/EmptyIcon";
import { compareByCreatedOnThenId } from "@features/support/utils/support";
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
import { ApiError } from "@utils/ApiError";

// TODO : DUE TO URGENCY THIS COMPONENT BREAKS THE BEST PRACTICES , NEED FULL REFACTOR
/**
 * Renders the Activity tab: editor at top, comments below (latest first), floating scroll-to-top button.
 *
 * @param {CaseDetailsActivityPanelProps} props - projectId, caseId, optional case created date.
 * @returns {JSX.Element} The activity timeline panel.
 */
export default function CaseDetailsActivityPanel({
  projectId,
  caseId,
  conversationId,
  caseStatus,
}: CaseDetailsActivityPanelProps): JSX.Element {
  const theme = useTheme();
  const scrollRef = useRef<HTMLDivElement>(null);
  const [showScrollTop, setShowScrollTop] = useState(false);

  const {
    data: userDetails,
    isError: isUserDetailsError,
    error: userDetailsError,
  } = useGetUserDetails();
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
  const hideAvatar =
    isUserDetailsError &&
    userDetailsError instanceof ApiError &&
    (userDetailsError.status === 401 || userDetailsError.status === 403);

  const mergedTimeline = useMemo(() => {
    const caseComments = commentsData?.comments ?? [];
    const conversationComments: CaseComment[] = (
      conversationData?.pages?.flatMap((page) => page.comments) ?? []
    ).map((comment) => ({
      ...comment,
      type: comment.type ?? "comments",
      isEscalated: comment.isEscalated ?? false,
    }));
    return [...caseComments, ...conversationComments].sort(
      (a, b) => -compareByCreatedOnThenId(a, b),
    );
  }, [commentsData?.comments, conversationData?.pages]);

  const commentsToShow = useMemo(
    () => mergedTimeline.filter(hasDisplayableContent),
    [mergedTimeline],
  );

  const primaryLight = theme.palette.primary?.light ?? "#fa7b3f";
  const primaryBg = alpha(primaryLight, 0.1);

  const isLoading = isCommentsLoading || (!!conversationId && isConversationLoading);
  const isError = isCommentsError || (!!conversationId && isConversationError);
  const resolvedError = commentsError ?? conversationError;

  const handleScroll = () => {
    if (scrollRef.current) {
      setShowScrollTop(scrollRef.current.scrollTop > 120);
    }
  };

  const scrollToTop = () => {
    scrollRef.current?.scrollTo({ top: 0, behavior: "smooth" });
  };

  let commentsContent: JSX.Element;

  if (isLoading) {
    commentsContent = (
      <Stack spacing={2} sx={{ p: 2 }}>
        {[1, 2, 3].map((i) => (
          <Stack key={i} direction="row" spacing={1.5} alignItems="flex-start">
            <Skeleton variant="circular" width={32} height={32} />
            <Box sx={{ flex: 1, width: "100%" }}>
              <Skeleton variant="text" width="40%" height={20} />
              <Skeleton variant="rectangular" height={60} sx={{ mt: 1 }} />
            </Box>
          </Stack>
        ))}
      </Stack>
    );
  } else if (isError || !commentsData) {
    commentsContent = (
      <Box sx={{ p: 2 }}>
        <ApiErrorState
          error={resolvedError}
          fallbackMessage="Unable to load activity."
        />
      </Box>
    );
  } else {
    commentsContent = (
      <ActivityContentWithImageModal
        commentsToShow={commentsToShow}
        caseCreatedOn={undefined}
        currentUserEmail={currentUserEmail}
        primaryBg={primaryBg}
        hideAvatar={hideAvatar}
        userDetails={userDetails}
      />
    );
  }

  return (
    <Box
      sx={{
        display: "flex",
        flexDirection: "column",
        height: "100%",
        overflow: "hidden",
        position: "relative",
      }}
    >
      {/* Single scrollable area — editor + comments scroll together */}
      <Box
        ref={scrollRef}
        onScroll={handleScroll}
        sx={{
          flex: 1,
          overflow: "auto",
          display: "flex",
          flexDirection: "column",
          WebkitOverflowScrolling: "touch",
        }}
      >
        <Box sx={{ px: 2, pt: 2, pb: 1 }}>
          <ActivityCommentInput caseId={caseId} caseStatus={caseStatus} />
        </Box>
        {commentsContent}
      </Box>

      {/* Floating scroll-to-top button */}
      {showScrollTop && (
        <Tooltip title="Back to top">
          <IconButton
            onClick={scrollToTop}
            size="small"
            aria-label="Scroll to top"
            sx={{
              position: "absolute",
              bottom: 16,
              right: 16,
              bgcolor: "transparent",
              boxShadow: 3,
              zIndex: 10,
              width: 36,
              height: 36,
            }}
          >
            <ChevronUp size={18} />
          </IconButton>
        </Tooltip>
      )}
    </Box>
  );
}

function ActivityContentWithImageModal(
  props: ActivityContentProps,
): JSX.Element {
  const [fullscreenImageSrc, setFullscreenImageSrc] = useState<string | null>(null);
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
  currentUserEmail,
  primaryBg,
  hideAvatar = false,
  userDetails,
  onImageClick,
}: ActivityContentProps): JSX.Element {
  return (
    <Box sx={{ p: 2, flex: 1 }}>
      <Stack spacing={2}>
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
                hideAvatar={hideAvatar}
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
