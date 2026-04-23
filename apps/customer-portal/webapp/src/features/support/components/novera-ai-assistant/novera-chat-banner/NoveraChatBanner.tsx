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

import { Box, Button, Paper, Typography } from "@wso2/oxygen-ui";
import { Bot } from "@wso2/oxygen-ui-icons-react";
import { type JSX } from "react";
import { useNavigate } from "react-router";

/**
 * NoveraChatBanner component to display a "Start New Chat" card.
 *
 * @returns {JSX.Element} The rendered NoveraChatBanner component.
 */
export default function NoveraChatBanner(): JSX.Element {
  const navigate = useNavigate();

  return (
    <Paper
      sx={{
        p: 3,
        display: "flex",
        alignItems: "center",
        justifyContent: "space-between",
        flexWrap: { xs: "wrap", sm: "nowrap" },
        gap: 2,
      }}
    >
      <Box
        sx={{
          display: "flex",
          alignItems: "center",
          gap: 2,
        }}
      >
        <Paper
          sx={{
            width: 48,
            height: 48,
            bgcolor: "primary.main",
            display: "flex",
            alignItems: "center",
            justifyContent: "center",
            flexShrink: 0,
          }}
        >
          <Bot size={24} color="white" />
        </Paper>
        <Box>
          <Typography variant="h6">Need help with something new?</Typography>
          <Typography variant="body2">
            Chat with Novera to get instant assistance or create a new support
            case
          </Typography>
        </Box>
      </Box>
      <Button
        variant="contained"
        color="warning"
        onClick={() => navigate("chat/describe-issue")}
      >
        Start New Chat
      </Button>
    </Paper>
  );
}
