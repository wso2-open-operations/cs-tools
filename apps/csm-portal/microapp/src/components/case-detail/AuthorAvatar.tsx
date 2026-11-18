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

import { Avatar, Badge, Box, colors } from "@wso2/oxygen-ui";
import { Headset } from "@wso2/oxygen-ui-icons-react";
import { initialsOf } from "@utils/initials";

const AVATAR_SIZE = 28;

/**
 * A comment author's avatar: their initials, and, for a support engineer, a small headset badge.
 * Mirrors the webapp's comment bubble (CsmCaseCommentBubble): the backend resolves the author's own
 * user type and sends it as a flag, so nothing here looks the author up or guesses from the name.
 * A customer, an unresolved author and a bot show the plain avatar.
 */
export function AuthorAvatar({ name, isSupportEngineer }: { name: string; isSupportEngineer: boolean }) {
  return (
    <Badge
      overlap="circular"
      anchorOrigin={{ vertical: "bottom", horizontal: "right" }}
      invisible={!isSupportEngineer}
      badgeContent={
        isSupportEngineer ? (
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
      <Avatar sx={{ width: AVATAR_SIZE, height: AVATAR_SIZE, fontSize: "0.7rem" }}>{initialsOf(name)}</Avatar>
    </Badge>
  );
}
