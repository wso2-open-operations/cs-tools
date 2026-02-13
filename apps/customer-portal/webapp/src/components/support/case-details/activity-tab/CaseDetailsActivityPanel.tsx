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
  CircularProgress,
  Divider,
  IconButton,
  Paper,
  Skeleton,
  Stack,
  TextField,
  Typography,
  alpha,
  useTheme,
} from "@wso2/oxygen-ui";
import { ChevronDown, Send } from "@wso2/oxygen-ui-icons-react";
import { useMemo, useState, type JSX } from "react";
import useGetCaseComments from "@api/useGetCaseComments";
import { usePostComment } from "@api/usePostComment";
import useGetUserDetails from "@api/useGetUserDetails";
import type { CaseComment } from "@models/responses";
import {
  stripCodeWrapper,
  stripCustomerCommentAddedLabel,
  replaceInlineImageSources,
  formatCommentDate,
} from "@utils/support";

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

          {commentsSorted.length === 0 ? (
            <Typography variant="body2" color="text.secondary">
              No activity yet.
            </Typography>
          ) : (
            commentsSorted.map((comment) => (
              <CommentBubble
                key={comment.id}
                comment={comment}
                isCurrentUser={
                  comment.createdBy?.toLowerCase() === currentUserEmail
                }
                primaryBg={primaryBg}
              />
            ))
          )}
        </Stack>
      </Box>
      <ActivityCommentInput caseId={caseId} />
    </Box>
  );
}

interface ActivityCommentInputProps {
  caseId: string;
}

/**
 * Fixed (non-scrollable) input row with text field and send button.
 * Posts comment via usePostComment; on success invalidates and refetches comments.
 *
 * @param {ActivityCommentInputProps} props - caseId for POST.
 * @returns {JSX.Element} The comment input component.
 */
function ActivityCommentInput({ caseId }: ActivityCommentInputProps): JSX.Element {
  const [value, setValue] = useState("");
  const postComment = usePostComment();

  const handleSend = () => {
    if (!value.trim()) return;
    const content = value.trim();
    postComment.mutate(
      { caseId, body: { content } },
      {
        onSuccess: () => setValue(""),
      },
    );
  };

  return (
    <Box
      sx={{
        display: "flex",
        gap: 1,
        px: 2,
        py: 1.5,
        flexShrink: 0,
      }}
    >
      <TextField
        fullWidth
        placeholder="Add a comment..."
        size="small"
        value={value}
        disabled={postComment.isPending}
        onChange={(e) => setValue(e.target.value)}
        onKeyDown={(e) => {
          if (e.key === "Enter" && !e.shiftKey) {
            e.preventDefault();
            handleSend();
          }
        }}
      />
      <IconButton
        disabled={!value.trim() || postComment.isPending}
        onClick={handleSend}
        color="warning"
        aria-label="Send comment"
      >
        {postComment.isPending ? (
          <CircularProgress color="inherit" size={18} />
        ) : (
          <Send size={18} />
        )}
      </IconButton>
    </Box>
  );
}

/** Line count threshold for showing "Show more" (approximately 4 lines). */
const COLLAPSE_CHAR_THRESHOLD = 200;

interface ChatMessageCardProps {
  htmlContent: string;
  isExpanded: boolean;
  onToggleExpand: () => void;
  isCurrentUser: boolean;
  primaryBg: string;
}

/**
 * Card-style chat message with collapsible long content and "Show more" button.
 * Uses Paper without border or border radius.
 *
 * @param {ChatMessageCardProps} props - Content, expand state, and styling.
 * @returns {JSX.Element} The chat message card.
 */
function ChatMessageCard({
  htmlContent,
  isExpanded,
  onToggleExpand,
  isCurrentUser,
  primaryBg,
}: ChatMessageCardProps): JSX.Element {
  const plainLength = htmlContent.replace(/<[^>]+>/g, "").length;
  const showExpandButton = plainLength > COLLAPSE_CHAR_THRESHOLD;

  return (
    <Paper
      elevation={0}
      sx={{
        display: "flex",
        flexDirection: "column",
        gap: 1.5,
        p: 1.5,
        maxWidth: "100%",
        bgcolor: isCurrentUser ? primaryBg : "background.paper",
        border: "none",
        borderRadius: 0,
        boxShadow: "none",
      }}
    >
      <Box
        sx={{
          fontSize: "0.75rem",
          fontFamily: "monospace",
          "& p": {
            margin: "0 0 0.5em 0",
            whiteSpace: "pre-wrap",
            wordBreak: "break-word",
          },
          "& p:last-child": { marginBottom: 0 },
          "& img": {
            display: "block",
            maxWidth: "100%",
            maxHeight: 320,
            height: "auto",
            objectFit: "contain",
            mt: 0.5,
            mb: 0.5,
          },
          "& br": { display: "block", content: '""', marginTop: "0.25em" },
          ...(!isExpanded &&
            showExpandButton && {
              display: "-webkit-box",
              WebkitLineClamp: 4,
              WebkitBoxOrient: "vertical" as const,
              overflow: "hidden",
            }),
        }}
        dangerouslySetInnerHTML={{ __html: htmlContent }}
      />
      {showExpandButton && (
        <>
          <Divider sx={{ my: 0.25 }} />
          <Button
            size="small"
            variant="text"
            onClick={onToggleExpand}
            endIcon={
              <ChevronDown
                size={14}
                style={{
                  transform: isExpanded ? "rotate(180deg)" : "none",
                  transition: "transform 0.2s",
                }}
              />
            }
            sx={{
              alignSelf: "stretch",
              justifyContent: "center",
              fontSize: "0.75rem",
              color: "text.secondary",
              "&:hover": {
                color: "text.primary",
                bgcolor: "action.hover",
              },
            }}
          >
            {isExpanded ? "Show less" : "Show more"}
          </Button>
        </>
      )}
    </Paper>
  );
}

interface CommentBubbleProps {
  comment: CaseComment;
  isCurrentUser: boolean;
  primaryBg: string;
}

function CommentBubble({
  comment,
  isCurrentUser,
  primaryBg,
}: CommentBubbleProps): JSX.Element {
  const theme = useTheme();
  const [expanded, setExpanded] = useState(false);
  const rawContent = comment.content ?? "";
  const stripped = stripCodeWrapper(rawContent);
  const withoutLabel = stripCustomerCommentAddedLabel(stripped);
  const htmlContent = replaceInlineImageSources(
    withoutLabel,
    comment.inlineAttachments,
  );
  const displayName = isCurrentUser ? null : (comment.createdBy || "Unknown");
  const initials = useMemo(() => {
    if (isCurrentUser) return "YO";
    const email = comment.createdBy ?? "";
    const part = email.split("@")[0] ?? "";
    return (
      part
        .replace(/[._-]/g, " ")
        .trim()
        .split(/\s+/)
        .filter(Boolean)
        .map((s) => s[0])
        .join("")
        .toUpperCase()
        .slice(0, 2) || "?"
    );
  }, [isCurrentUser, comment.createdBy]);

  const isRight = isCurrentUser;

  return (
    <Stack
      direction="row"
      spacing={1.5}
      alignItems="flex-start"
      sx={{
        flexDirection: isRight ? "row-reverse" : "row",
      }}
    >
      <Avatar
        sx={{
          width: 32,
          height: 32,
          fontSize: "0.75rem",
          flexShrink: 0,
          bgcolor: isCurrentUser
            ? primaryBg
            : alpha(theme.palette.info?.light ?? "#0288d1", 0.2),
          color: isCurrentUser
            ? theme.palette.primary.main
            : (theme.palette.info?.main ?? "#0288d1"),
        }}
      >
        {initials}
      </Avatar>
      <Stack
        spacing={1}
        sx={{
          flex: 1,
          minWidth: 0,
          alignItems: isRight ? "flex-end" : "flex-start",
        }}
      >
        <Stack
          direction="row"
          spacing={1}
          alignItems="center"
          flexWrap="wrap"
          sx={{
            flexDirection: isRight ? "row-reverse" : "row",
            minHeight: 32,
          }}
        >
          {displayName && (
            <Typography variant="body2" color="text.primary" fontWeight={500}>
              {displayName}
            </Typography>
          )}
          <Typography variant="caption" color="text.secondary">
            {formatCommentDate(comment.createdOn)}
          </Typography>
          {!isCurrentUser && (
            <Chip
              label="Support Engineer"
              size="small"
              variant="outlined"
              sx={{
                height: 20,
                fontSize: "0.75rem",
                borderColor: "transparent",
                bgcolor: "action.hover",
              }}
            />
          )}
        </Stack>
        <ChatMessageCard
          htmlContent={htmlContent}
          isExpanded={expanded}
          onToggleExpand={() => setExpanded((prev) => !prev)}
          isCurrentUser={isCurrentUser}
          primaryBg={primaryBg}
        />
      </Stack>
    </Stack>
  );
}
