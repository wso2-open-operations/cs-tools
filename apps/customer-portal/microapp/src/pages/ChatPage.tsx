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
import { useEffect, useState } from "react";

import { Box, Stack } from "@wso2/oxygen-ui";
import { MessageSquareQuote } from "@wso2/oxygen-ui-icons-react";

import { useDeclareLayout } from "@context/layout";
import { useProject } from "@context/project";

import { Bubble, PromptCreateCase } from "@features/chats/components";
import { useClassify, useConversation, useEnvProducts, useNovera, useStream } from "@features/chats/hooks";

import { CommentBar } from "@shared/components/core";

import { Tab } from "@shared/constants";
import { scrollToBottom, toTranscript } from "@shared/utils";

export default function ChatPage() {
  useDeclareLayout({
    tabIndex: Tab.Support,
    title: "Chat with Novera",
    visibility: { backAction: true },
    slots: {
      leading: (
        <Box color="primary.main">
          <MessageSquareQuote size={32} />
        </Box>
      ),
    },
  });

  const { projectId, projectTypeId } = useProject();
  const { draft, committed, pending, stream, finish, reset } = useStream();
  const { messages, append } = useConversation(committed, reset);
  const { status, send } = useNovera(projectId!, stream);
  const classify = useClassify(messages);
  const { envProducts } = useEnvProducts();

  const [comment, setComment] = useState("");

  const handleSend = () => {
    if (!comment.trim()) return;

    append(comment);
    send(comment);
    setComment("");
  };

  const handleCreateCase = () => {
    classify.mutate({
      projectTypeId,
      chatHistory: toTranscript(messages),
      envProducts,
    });
  };

  useEffect(scrollToBottom, [messages, draft?.content]);

  return (
    <>
      <Stack mb={20} gap={2}>
        {messages.map((message, index) => (
          <Bubble key={index} {...message} />
        ))}

        {/* Temporary message bubble for streamed content */}
        {draft && <Bubble {...draft} onAnimationComplete={finish} />}
      </Stack>

      <CommentBar
        placeholder="Type a message..."
        value={comment}
        onChange={setComment}
        onSend={handleSend}
        loading={pending}
        slots={{
          top:
            messages.length > 1 ? (
              <PromptCreateCase pending={classify.isPending} onCreateCase={handleCreateCase} />
            ) : undefined,
        }}
        disabled={status === WebSocket.CLOSED}
      />
    </>
  );
}
