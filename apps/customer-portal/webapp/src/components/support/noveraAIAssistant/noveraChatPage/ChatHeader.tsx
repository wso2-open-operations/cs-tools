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

import { Box, Button, Typography } from "@wso2/oxygen-ui";
import { Bot, ArrowLeft } from "@wso2/oxygen-ui-icons-react";
import { type JSX } from "react";

interface ChatHeaderProps {
  onBack: () => void;
}

/**
 * Renders the header section for the Novera Chat page.
 *
 * Includes navigation controls such as the back action.
 *
 * @returns The ChatHeader JSX element.
 */
export default function ChatHeader({ onBack }: ChatHeaderProps): JSX.Element {
  return (
    <Box sx={{ mb: 3 }}>
      <Button
        startIcon={<ArrowLeft size={18} />}
        onClick={onBack}
        sx={{ mb: 2, textTransform: "none" }}
        variant="text"
      >
        Back to Support
      </Button>

      <Box sx={{ display: "flex", alignItems: "center", gap: 2 }}>
        <Box
          sx={{
            width: (theme) => theme.spacing(5),
            height: (theme) => theme.spacing(5),
            bgcolor: "primary.main",
            borderRadius: "50%",
            display: "flex",
            alignItems: "center",
            justifyContent: "center",
          }}
        >
          <Bot size={20} color="white" />
        </Box>
        <Box>
          <Typography variant="h6">Chat with Novera</Typography>
          <Typography variant="body2" color="text.secondary">
            AI-powered support assistant
          </Typography>
        </Box>
      </Box>
    </Box>
  );
}
