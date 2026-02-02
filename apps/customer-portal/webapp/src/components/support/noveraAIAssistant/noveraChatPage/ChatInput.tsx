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

import { Box, IconButton, TextField } from "@wso2/oxygen-ui";
import { Send } from "@wso2/oxygen-ui-icons-react";
import { type JSX } from "react";
import EscalationBanner from "@/components/support/noveraAIAssistant/noveraChatPage/EscalationBanner";

interface ChatInputProps {
  inputValue: string;
  setInputValue: (value: string) => void;
  onSend: () => void;
  showEscalationBanner: boolean;
  onCreateCase: () => void;
}

/**
 * Renders the input area for the Novera Chat page.
 *
 * Handles message input, sending messages, and optionally displays
 * an escalation banner for creating support cases.
 *
 * @returns The ChatInput JSX element.
 */
export default function ChatInput({
  inputValue,
  setInputValue,
  onSend,
  showEscalationBanner,
  onCreateCase,
}: ChatInputProps): JSX.Element {
  return (
    <Box sx={{ p: 2, bgcolor: "background.paper", flexShrink: 0 }}>
      <EscalationBanner
        visible={showEscalationBanner}
        onCreateCase={onCreateCase}
      />
      <Box sx={{ display: "flex", gap: 1 }}>
        <TextField
          fullWidth
          placeholder="Type your message..."
          size="small"
          value={inputValue}
          onChange={(e) => setInputValue(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === "Enter") {
              if (!inputValue.trim()) return;
              e.preventDefault();
              onSend();
            }
          }}
        />
        <IconButton
          disabled={!inputValue.trim()}
          onClick={onSend}
          color="warning"
        >
          <Send size={18} />
        </IconButton>
      </Box>
    </Box>
  );
}
