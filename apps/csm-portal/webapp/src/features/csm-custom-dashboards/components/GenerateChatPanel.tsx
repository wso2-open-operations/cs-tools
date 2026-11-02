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

import { Box, Chip, IconButton, TextField, Tooltip, Typography } from "@wso2/oxygen-ui";
import { SendHorizontal, X } from "@wso2/oxygen-ui-icons-react";
import { type JSX, type KeyboardEvent, useEffect, useRef, useState } from "react";

export interface GenerateChatMessage {
  id: string;
  role: "user" | "assistant";
  text: string;
}

/** Longest message the box accepts; the backend will enforce its own limit too. */
export const GENERATE_PROMPT_MAX_LENGTH = 2000;

const GREETING =
  "Describe the dashboard you'd like, and I'll build it with live data. You can ask for changes afterwards.";
const EDIT_GREETING = "Ask for a change to this dashboard, and I'll update the preview. Save when you're happy with it.";

const EXAMPLE_PROMPTS = [
  "My open cases by severity",
  "SLA breaches in the last 7 days by severity",
  "Cases waiting on the customer",
  "Top 10 customers by open cases",
];

interface GenerateChatPanelProps {
  messages: GenerateChatMessage[];
  onSend: (text: string) => void;
  onClose: () => void;
  /** A reply is on its way: shows "Generating…" and blocks sending. */
  busy?: boolean;
  /** Changing a saved dashboard: its own title and greeting, no example prompts. */
  editing?: boolean;
}

/**
 * The chat side of the generate page: a greeting, example prompts (shown until
 * the first message, and not when editing a saved dashboard; clicking one fills
 * the box so it can be edited before sending), the messages, and the input. Enter sends, Shift+Enter adds a line.
 *
 * It only shows what it is given: the page owns the messages and decides what
 * a message does.
 */
export default function GenerateChatPanel({
  messages,
  onSend,
  onClose,
  busy = false,
  editing = false,
}: GenerateChatPanelProps): JSX.Element {
  const [draft, setDraft] = useState("");
  const endRef = useRef<HTMLDivElement>(null);
  const canSend = draft.trim().length > 0 && !busy;

  // Keep the newest message (or the "Generating…" bubble) in view. Optional
  // call: jsdom has no scrollIntoView.
  useEffect(() => {
    endRef.current?.scrollIntoView?.({ block: "end" });
  }, [messages.length, busy]);

  const send = (): void => {
    const text = draft.trim();
    if (!text || busy) return;
    onSend(text);
    setDraft("");
  };

  const handleKeyDown = (e: KeyboardEvent<HTMLDivElement>): void => {
    if (e.key === "Enter" && !e.shiftKey) {
      e.preventDefault();
      send();
    }
  };

  return (
    <Box sx={{ display: "flex", flexDirection: "column", height: "100%", minHeight: 0 }}>
      <Box
        sx={{
          px: 2,
          py: 1,
          borderBottom: 1,
          borderColor: "divider",
          display: "flex",
          alignItems: "center",
          justifyContent: "space-between",
        }}
      >
        <Typography variant="subtitle1">{editing ? "Edit with AI" : "Generate dashboard"}</Typography>
        <Tooltip title="Close chat">
          <IconButton size="small" aria-label="Close chat" onClick={onClose}>
            <X size={16} />
          </IconButton>
        </Tooltip>
      </Box>

      <Box
        sx={{ flex: 1, minHeight: 0, overflowY: "auto", p: 2, display: "flex", flexDirection: "column", gap: 1.5 }}
        aria-label="Chat messages"
        role="log"
      >
        <Bubble role="assistant" text={editing ? EDIT_GREETING : GREETING} />
        {messages.length === 0 && !editing && (
          <Box sx={{ display: "flex", flexWrap: "wrap", gap: 1 }}>
            {EXAMPLE_PROMPTS.map((prompt) => (
              <Chip key={prompt} label={prompt} size="small" variant="outlined" onClick={() => setDraft(prompt)} />
            ))}
          </Box>
        )}
        {messages.map((m) => (
          <Bubble key={m.id} role={m.role} text={m.text} />
        ))}
        {busy && <Bubble role="assistant" text="Generating…" />}
        <div ref={endRef} />
      </Box>

      <Box sx={{ p: 1.5, borderTop: 1, borderColor: "divider", display: "flex", gap: 1, alignItems: "flex-end" }}>
        <TextField
          value={draft}
          onChange={(e) => setDraft(e.target.value)}
          onKeyDown={handleKeyDown}
          placeholder={messages.length === 0 && !editing ? "Describe a dashboard…" : "Ask for a change…"}
          size="small"
          fullWidth
          disabled={busy}
          multiline
          maxRows={4}
          slotProps={{ htmlInput: { maxLength: GENERATE_PROMPT_MAX_LENGTH, "aria-label": "Message" } }}
        />
        <IconButton color="primary" aria-label="Send" onClick={send} disabled={!canSend}>
          <SendHorizontal size={18} />
        </IconButton>
      </Box>
    </Box>
  );
}

/** One chat message: the user's on the right, the assistant's on the left. */
function Bubble({ role, text }: { role: GenerateChatMessage["role"]; text: string }): JSX.Element {
  const isUser = role === "user";
  return (
    <Box
      sx={{
        alignSelf: isUser ? "flex-end" : "flex-start",
        maxWidth: "85%",
        px: 1.5,
        py: 1,
        borderRadius: 2,
        bgcolor: isUser ? "primary.main" : "action.hover",
        color: isUser ? "primary.contrastText" : "text.primary",
        whiteSpace: "pre-wrap",
        overflowWrap: "anywhere",
      }}
    >
      <Typography variant="body2" color="inherit">
        {text}
      </Typography>
    </Box>
  );
}
