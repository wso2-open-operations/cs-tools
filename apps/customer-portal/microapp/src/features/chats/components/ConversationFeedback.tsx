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

import { useState } from "react";
import { Card, IconButton, pxToRem, Stack, Typography, useTheme } from "@wso2/oxygen-ui";
import { ThumbsDown, ThumbsUp } from "@wso2/oxygen-ui-icons-react";

export function ConversationFeedback() {
  const theme = useTheme();
  const [feedback, setFeedback] = useState<"up" | "down" | undefined>(undefined);
  const up = feedback === "up";
  const down = feedback === "down";

  return (
    <Card component={Stack} direction="row" justifyContent="space-between" p={1.5}>
      <Stack>
        <Typography variant="body1" fontWeight="medium">
          Was this conversation helpful?
        </Typography>
        <Typography variant="subtitle2" color="text.secondary">
          Your feedback helps us improve Novera
        </Typography>
      </Stack>
      <Stack direction="row">
        <IconButton
          aria-label="Mark conversation as helpful"
          onClick={() => setFeedback("up")}
          sx={{ color: up ? "primary.main" : "text.secondary" }}
          disableRipple
        >
          <ThumbsUp size={pxToRem(18)} fill={up ? theme.palette.primary.main : "none"} />
        </IconButton>
        <IconButton
          aria-label="Mark conversation as not helpful"
          onClick={() => setFeedback("down")}
          sx={{ color: down ? "primary.main" : "text.secondary" }}
          disableRipple
        >
          <ThumbsDown size={pxToRem(18)} fill={down ? theme.palette.primary.main : "none"} />
        </IconButton>
      </Stack>
    </Card>
  );
}
