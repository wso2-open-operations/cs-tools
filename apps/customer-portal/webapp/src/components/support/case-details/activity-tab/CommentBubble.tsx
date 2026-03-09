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
  Chip,
  Stack,
  Typography,
  alpha,
  useTheme,
} from "@wso2/oxygen-ui";
import { useMemo, useState } from "react";
import type { CaseComment } from "@models/responses";
import {
  getInitials,
  hasSingleCodeWrapper,
  stripCodeWrapper,
  stripAllCodeBlocks,
  convertCodeTagsToHtml,
  trimLeadingBr,
  stripCustomerCommentAddedLabel,
  replaceInlineImageSources,
  formatCommentDate,
} from "@utils/support";
import DOMPurify from "dompurify";
import ChatMessageCard from "@case-details-activity/ChatMessageCard";

export interface CommentBubbleProps {
  comment: CaseComment;
  isCurrentUser: boolean;
  isSupportEngineer: boolean;
  primaryBg: string;
  userDetails?: {
    email?: string;
    firstName?: string;
    lastName?: string;
  } | null;
}

/**
 * Single comment bubble: avatar, display name, date, and ChatMessageCard.
 *
 * @param {CommentBubbleProps} props - Comment data and styling.
 * @returns {JSX.Element} The comment bubble.
 */
export default function CommentBubble({
  comment,
  isCurrentUser,
  isSupportEngineer,
  primaryBg,
  userDetails,
}: CommentBubbleProps): import("react").JSX.Element {
  const theme = useTheme();
  const [expanded, setExpanded] = useState(false);
  const rawContent = comment.content ?? "";
  const isFullCodeWrap = hasSingleCodeWrapper(rawContent);
  const codeBlockCount = rawContent.match(/\[code\]/gi)?.length ?? 0;
  const afterCode = isFullCodeWrap
    ? stripCodeWrapper(rawContent)
    : codeBlockCount > 1
      ? stripAllCodeBlocks(rawContent)
      : convertCodeTagsToHtml(rawContent);
  const trimmedBr = trimLeadingBr(afterCode);
  const withoutLabel = stripCustomerCommentAddedLabel(trimmedBr);
  const withImages = replaceInlineImageSources(
    withoutLabel,
    comment.inlineAttachments,
  );
  const htmlContent = DOMPurify.sanitize(withImages);
  const displayName = isCurrentUser ? null : comment.createdBy || "Unknown";
  const initials = useMemo(() => {
    if (isCurrentUser && userDetails) {
      const { firstName, lastName, email } = userDetails;
      const fromName =
        firstName != null || lastName != null
          ? getInitials([firstName, lastName].filter(Boolean).join(" "))
          : "";
      if (fromName !== "--") return fromName;
      if (email) return getInitials(email);
    }
    if (!isCurrentUser) {
      return getInitials(comment.createdBy ?? "");
    }
    return "?";
  }, [isCurrentUser, comment.createdBy, userDetails]);

  const isRight = isCurrentUser;

  return (
    <Stack
      direction="row"
      alignItems="flex-start"
      sx={{
        flexDirection: isRight ? "row-reverse" : "row",
        gap: 2,
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
        spacing={0.75}
        sx={{
          width: "100%",
          maxWidth: 1000,
          minWidth: 0,
          alignItems: isRight ? "flex-end" : "flex-start",
        }}
      >
        <Stack
          direction="row"
          alignItems="center"
          flexWrap="wrap"
          sx={{
            flexDirection: isRight ? "row-reverse" : "row",
            gap: 1,
            minHeight: 32,
          }}
        >
          {isCurrentUser && (
            <Typography variant="body2" color="text.primary" fontWeight={500}>
              You
            </Typography>
          )}
          {displayName && !isCurrentUser && (
            <Typography variant="body2" color="text.primary" fontWeight={500}>
              {displayName}
            </Typography>
          )}
          <Typography variant="caption" color="text.secondary">
            {formatCommentDate(comment.createdOn)}
          </Typography>
          {isSupportEngineer && (
            <Chip
              label="Support Engineer"
              size="small"
              variant="outlined"
              sx={{
                height: 20,
                fontSize: "0.75rem",
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
