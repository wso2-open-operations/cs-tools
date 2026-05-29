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
export interface ConversationCreatedNoveraResponse {
  type: "conversation_created";
  conversationId: string;
}

export interface ThinkingStartNoveraResponse {
  type: "thinking_start";
}

export interface TokenNoveraResponse {
  type: "token";
  content: string;
}

export interface ThinkingStepNoveraResponse {
  type: "thinking_step";
  step: string;
  label: string;
}

export interface ThinkingEndNoveraResponse {
  type: "thinking_end";
}

export interface FinalNoveraResponse {
  type: "final";
  payload: {
    message: string;
    sessionId: string;
    conversationId: string;
  };
}

export type NoveraResponse =
  | ConversationCreatedNoveraResponse
  | ThinkingStartNoveraResponse
  | ThinkingEndNoveraResponse
  | ThinkingStepNoveraResponse
  | TokenNoveraResponse
  | FinalNoveraResponse;
