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

import type { ReactNode } from "react";
import { Avatar, Badge, Box, Card, Skeleton, Stack, Typography, colors } from "@wso2/oxygen-ui";
import { Headset } from "@wso2/oxygen-ui-icons-react";
import { stringAvatar } from "@utils/others";
import { SYSTEM_AUTHOR_LABEL } from "@utils/caseComments";

interface CommentProps {
  children: string | ReactNode;
  author: string;
  timestamp: string;
  /** The author is a support engineer: their avatar carries a headset badge. */
  isInternal?: boolean;
}

/**
 * The author's avatar: their initials, and for a support engineer a small headset badge, as in the
 * webapp's activity tab. The backend resolves the author's own user type and sends it as a flag, so
 * nothing here guesses from the name. A customer, an unresolved author and Novera get the plain avatar.
 */
function AuthorAvatar({ name, isInternal }: { name: string; isInternal: boolean }) {
  return (
    <Badge
      overlap="circular"
      anchorOrigin={{ vertical: "bottom", horizontal: "right" }}
      invisible={!isInternal}
      badgeContent={
        isInternal ? (
          <Box
            component="span"
            role="img"
            aria-label="Technical Team"
            sx={{
              display: "inline-flex",
              alignItems: "center",
              justifyContent: "center",
              width: 16,
              height: 16,
              borderRadius: "50%",
              bgcolor: "common.white",
              color: colors.blue[700],
              boxShadow: 1,
            }}
          >
            <Headset size={12} strokeWidth={2.5} />
          </Box>
        ) : undefined
      }
      sx={{ flexShrink: 0 }}
    >
      <Avatar sx={{ width: 32, height: 32, fontSize: "0.75rem" }}>{stringAvatar(name)}</Avatar>
    </Badge>
  );
}

export function Comment({ children, author: authorName, timestamp, isInternal = false }: CommentProps) {
  // A comment with no author is the system's, as in the webapp: never a blank name or a "?" avatar.
  const author = authorName.trim() || SYSTEM_AUTHOR_LABEL;
  return (
    <Card component={Stack} sx={{ bgcolor: "background.default" }}>
      <Stack direction="row" alignItems="center" gap={1.25} p={1}>
        <AuthorAvatar name={author} isInternal={isInternal} />
        <Stack sx={{ minWidth: 0 }}>
          <Typography variant="body2" fontWeight="medium">
            {author}
          </Typography>
          <Typography variant="caption" color="text.secondary">
            {timestamp}
          </Typography>
        </Stack>
      </Stack>
      <Box bgcolor="background.paper" p={1}>
        <Typography variant="body2">{children}</Typography>
      </Box>
    </Card>
  );
}

export function CommentSkeleton() {
  return (
    <Card component={Stack} p={1} gap={1.5} sx={{ bgcolor: "background.default" }}>
      <Stack direction="row" justifyContent="space-between" alignItems="center">
        <Typography variant="body2">
          <Skeleton width={100} />
        </Typography>
        <Typography variant="caption">
          <Skeleton width={60} />
        </Typography>
      </Stack>

      <Typography variant="body2">
        <Skeleton variant="text" />
        <Skeleton variant="text" width="80%" />
      </Typography>
    </Card>
  );
}
