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

import { alpha, Avatar, Box, Chip, Paper, Typography, useTheme } from "@wso2/oxygen-ui";
import type { JSX } from "react";
import RelativeTime from "@components/RelativeTime";
import SemanticChip from "@components/SemanticChip";
import { pickAccessibleText } from "@utils/contrastText";
import { sanitizeRichTextHtml } from "@utils/sanitizeHtml";
import { initialsOf } from "@utils/userClaims";
import type {
  CsmCaseComment,
  CsmCommentAuthorRole,
} from "@features/csm-cases/types/csmCases";

interface CsmCaseCommentBubbleProps {
  comment: CsmCaseComment;
}

const ROLE_LABEL: Record<CsmCommentAuthorRole, string> = {
  customer: "Customer",
  wso2_engineer: "WSO2",
  system: "System",
};

const ROLE_COLOR: Record<
  CsmCommentAuthorRole,
  "default" | "primary" | "warning"
> = {
  customer: "default",
  wso2_engineer: "primary",
  system: "warning",
};

export default function CsmCaseCommentBubble({
  comment,
}: CsmCaseCommentBubbleProps): JSX.Element {
  const theme = useTheme();
  const safeHtml = sanitizeRichTextHtml(comment.bodyHtml);
  const isSystem = comment.authorRole === "system";

  if (isSystem) {
    return (
      <Box
        id={comment.id}
        sx={{
          display: "flex",
          alignItems: "center",
          gap: 1,
          py: 0.5,
          px: 1,
          color: "text.secondary",
          scrollMarginTop: 96,
        }}
      >
        <SemanticChip role="warning" label="System" />
        <Box
          sx={{
            flex: 1,
            minWidth: 0,
            overflowWrap: "anywhere",
            wordBreak: "break-word",
            "& p": { m: 0 },
            "& a": { color: "primary.main" },
            ...{ "& *": { fontSize: "0.875rem" } },
          }}
          dangerouslySetInnerHTML={{ __html: safeHtml }}
        />
        <Typography variant="caption" color="text.secondary">
          <RelativeTime iso={comment.createdAt} href={`#${comment.id}`} />
        </Typography>
      </Box>
    );
  }

  const isInternal = !!comment.internal;

  // The author avatar carries the brand orange for WSO2 engineers; its initials
  // must stay legible on that fill (white-on-orange ~2.4:1 fails AA). Pick the
  // text colour by the fill's luminance, and use a dark-enough grey for
  // customers (grey.500 also fails with either text colour).
  const avatarBg =
    comment.authorRole === "wso2_engineer"
      ? theme.palette.primary.main
      : theme.palette.grey[700];
  const avatarFg = pickAccessibleText(avatarBg);

  return (
    <Box
      id={comment.id}
      sx={{ display: "flex", gap: 1.5, alignItems: "flex-start", scrollMarginTop: 96 }}
    >
      <Avatar
        sx={{
          bgcolor: avatarBg,
          color: avatarFg,
          width: 32,
          height: 32,
          fontSize: "0.85rem",
        }}
      >
        {initialsOf(comment.authorName)}
      </Avatar>
      <Paper
        variant="outlined"
        sx={{
          p: 1.5,
          flex: 1,
          minWidth: 0,
          display: "flex",
          flexDirection: "column",
          gap: 0.75,
          backgroundColor: isInternal
            ? alpha(theme.palette.warning.main, 0.15)
            : undefined,
          borderColor: isInternal ? "warning.main" : undefined,
        }}
      >
        <Box sx={{ display: "flex", alignItems: "center", gap: 1, flexWrap: "wrap" }}>
          <Typography variant="subtitle2">{comment.authorName}</Typography>
          <Chip
            size="small"
            label={ROLE_LABEL[comment.authorRole]}
            color={ROLE_COLOR[comment.authorRole]}
            variant="outlined"
          />
          {/* A filled chip "bubble" marks the work note; paired with the tinted
              background it reads as internal without a heavy banner. */}
          {isInternal && <SemanticChip role="warning" label="Internal note" />}
          <Typography variant="caption" color="text.secondary">
            <RelativeTime iso={comment.createdAt} href={`#${comment.id}`} />
          </Typography>
        </Box>
        <Box
          sx={{
            minWidth: 0,
            overflowWrap: "anywhere",
            wordBreak: "break-word",
            "& p": { m: 0 },
            "& p + p": { mt: 0.75 },
            "& ul, & ol": { ml: 3, my: 0.5 },
            "& code": {
              bgcolor: "background.default",
              px: 0.5,
              borderRadius: 0.5,
              fontFamily: "monospace",
              fontSize: "0.85em",
              overflowWrap: "anywhere",
            },
            "& pre": {
              bgcolor: "background.default",
              p: 1,
              borderRadius: 1,
              overflowX: "auto",
              fontFamily: "monospace",
              fontSize: "0.85em",
            },
            "& a": { color: "primary.main" },
            "& img": { maxWidth: "100%" },
            "& blockquote": {
              borderLeft: 3,
              borderColor: "divider",
              pl: 1.5,
              ml: 0,
              my: 0.75,
              color: "text.secondary",
              fontStyle: "italic",
            },
            "& h1, & h2, & h3": { mt: 1, mb: 0.5 },
          }}
          dangerouslySetInnerHTML={{ __html: safeHtml }}
        />
      </Paper>
    </Box>
  );
}
