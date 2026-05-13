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
import { useEffect, useRef, useState } from "react";

import { useNavigate } from "react-router-dom";

import { Stack } from "@wso2/oxygen-ui";

import { useProject } from "@context/project";

import { CreateCaseBanner, MessageBubble } from "@features/chats/components";
import { useConversation } from "@features/chats/hooks/useConversation";
import { useNovera } from "@features/chats/hooks/useNovera";
import { useStream } from "@features/chats/hooks/useStream";

import { ROUTES } from "@shared/constants";

import { StickyCommentBar } from "@components/detail";

export default function ChatPage() {
  const navigate = useNavigate();
  const { projectId } = useProject();
  const { draft, committed, pending, stream, finish, reset } = useStream();
  const { messages, append } = useConversation(committed, reset);
  const { status, send } = useNovera(projectId!, stream);

  const bottomRef = useRef<HTMLDivElement | null>(null);
  const [comment, setComment] = useState("");

  const handleSend = () => {
    if (!comment.trim()) return;
    append(comment);
    setComment("");
    send(comment);
  };

  const handleCreateCase = () => {
    navigate(ROUTES.default_case.create, { state: { messages } });
  };

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [messages, draft]);

  return (
    <>
      <Stack mb={20} gap={2}>
        {messages.map((message, index) => (
          <MessageBubble key={index} {...message} />
        ))}

        {draft && <MessageBubble {...draft} onAnimationComplete={finish} />}
        <div ref={bottomRef} />
      </Stack>
      <StickyCommentBar
        loading={pending}
        disabled={status !== WebSocket.OPEN}
        value={comment}
        placeholder="Type your message"
        onChange={setComment}
        onSend={handleSend}
        topSlot={messages.length > 2 && <CreateCaseBanner onCreateCase={handleCreateCase} />}
      />
    </>
  );
}
