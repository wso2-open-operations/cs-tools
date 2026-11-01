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

import { Box } from "@wso2/oxygen-ui";
import type { JSX, ReactNode } from "react";

interface CommentCardHeaderProps {
  /** The author, chips and timestamp. These wrap onto further lines when the
   * card is narrow. */
  children: ReactNode;
  /** Icon-buttons for the card's top-right corner (copy link, actions menu). */
  actions: ReactNode;
}

/**
 * Header row shared by the comment-style cards of a case's activity feed
 * (comments, lifecycle entries, feedback, attachments).
 *
 * The actions are a fixed column on the right and only the left side wraps, so
 * the buttons stay in the card's top-right corner at any width. Putting them
 * in the same wrapping row as the author and chips let a long header push
 * them onto a line of their own.
 */
export default function CommentCardHeader({
  children,
  actions,
}: CommentCardHeaderProps): JSX.Element {
  return (
    <Box sx={{ display: "flex", alignItems: "flex-start", gap: 1 }}>
      <Box
        sx={{
          display: "flex",
          alignItems: "center",
          flexWrap: "wrap",
          gap: 1,
          flex: 1,
          minWidth: 0,
          // One long unbroken token (an author name) breaks anywhere rather
          // than running out of the column and under the actions. `anywhere`,
          // unlike `break-word`, also lowers the token's min-content width, so
          // the column can shrink to fit.
          overflowWrap: "anywhere",
          // The height of a small icon-button, so a one-line header centres
          // on the buttons beside it instead of sitting a few pixels above.
          minHeight: 26,
        }}
      >
        {children}
      </Box>
      <Box sx={{ display: "flex", alignItems: "center", flexShrink: 0 }}>
        {actions}
      </Box>
    </Box>
  );
}
