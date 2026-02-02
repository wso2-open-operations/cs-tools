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

import { Box, Paper, Divider } from "@wso2/oxygen-ui";
import { useState, useRef, useEffect, type JSX } from "react";
import { useNavigate, useParams } from "react-router";
import ChatHeader from "@/components/support/noveraAIAssistant/noveraChatPage/ChatHeader";
import ChatInput from "@/components/support/noveraAIAssistant/noveraChatPage/ChatInput";
import ChatMessageList from "@/components/support/noveraAIAssistant/noveraChatPage/ChatMessageList";
import { getNoveraResponse } from "@/models/mockFunctions";

interface Message {
  id: string;
  text: string;
  sender: "user" | "bot";
  timestamp: Date;
}

/**
 * NoveraChatPage component to provide AI-powered support assistance.
 *
 * @returns {JSX.Element} The rendered NoveraChatPage.
 */
export default function NoveraChatPage(): JSX.Element {
  const navigate = useNavigate();

  const { projectId } = useParams<{ projectId: string }>();

  const handleBack = () => {
    if (projectId) {
      navigate(`/${projectId}/support`);
    } else {
      navigate(-1);
    }
  };

  const handleCreateCase = () => {
    if (projectId) {
      navigate(`/${projectId}/support/chat/create-case`);
    } else {
      navigate("/");
    }
  };
  const [messages, setMessages] = useState<Message[]>([
    {
      id: "1",
      text: "Hi! I'm Novera, your AI support assistant. I'm here to help you resolve your issue quickly. Can you describe the problem you're experiencing?",
      sender: "bot",
      timestamp: new Date(),
    },
  ]);
  const [inputValue, setInputValue] = useState("");
  const messagesEndRef = useRef<HTMLDivElement>(null);
  const pendingTimeoutsRef = useRef<number[]>([]);

  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [messages]);

  useEffect(() => {
    return () => {
      pendingTimeoutsRef.current.forEach((id) => {
        clearTimeout(id);
      });
      pendingTimeoutsRef.current = [];
    };
  }, []);

  const handleSendMessage = () => {
    if (!inputValue.trim()) return;

    const userMessage: Message = {
      id: Date.now().toString(),
      text: inputValue,
      sender: "user",
      timestamp: new Date(),
    };

    setMessages((prev) => [...prev, userMessage]);
    setInputValue("");

    const timeoutId = window.setTimeout(() => {
      const botMessage: Message = {
        id: (Date.now() + 1).toString(),
        text: getNoveraResponse(),
        sender: "bot",
        timestamp: new Date(),
      };
      setMessages((prev) => [...prev, botMessage]);
      pendingTimeoutsRef.current = pendingTimeoutsRef.current.filter(
        (id) => id !== timeoutId,
      );
    }, 1000);

    pendingTimeoutsRef.current.push(timeoutId);
  };

  return (
    <Box
      sx={{
        height: (theme) => `calc(100vh - ${theme.spacing(21)})`,
        display: "flex",
      }}
    >
      <Box
        sx={{
          flex: 1,
          display: "flex",
          flexDirection: "column",
          width: "100%",
          overflow: "hidden",
        }}
      >
        <ChatHeader onBack={handleBack} />

        {/* Chat window */}
        <Paper
          sx={{
            flex: 1,
            display: "flex",
            flexDirection: "column",
            overflow: "hidden",
          }}
        >
          <ChatMessageList
            messages={messages}
            messagesEndRef={messagesEndRef}
          />

          <Divider />

          <ChatInput
            onSend={handleSendMessage}
            inputValue={inputValue}
            setInputValue={setInputValue}
            showEscalationBanner={messages.length > 4}
            onCreateCase={handleCreateCase}
          />
        </Paper>
      </Box>
    </Box>
  );
}
