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

import {
  mockActiveChatsOptions,
  mockCaseCreationMetadata,
  mockOpenCasesOptions,
  mockStatusOptions,
} from "@/models/mockData";
import type { CaseCreationMetadata } from "@/models/mockData";

import type { ProjectSupportStats } from "@/models/responses";

/**
 * Returns a random status from the mock status options.
 *
 * @returns {string} A random status string.
 */
export const getMockStatus = (): string => {
  return mockStatusOptions[
    Math.floor(Math.random() * mockStatusOptions.length)
  ];
};

/**
 * Returns a random count of open cases from the mock options.
 *
 * @returns {number} A random number of open cases.
 */
export const getMockOpenCases = (): number => {
  return mockOpenCasesOptions[
    Math.floor(Math.random() * mockOpenCasesOptions.length)
  ];
};

/**
 * Returns a random count of active chats from the mock options.
 *
 * @returns {number} A random number of active chats.
 */
export const getMockActiveChats = (): number => {
  return mockActiveChatsOptions[
    Math.floor(Math.random() * mockActiveChatsOptions.length)
  ];
};

/**
 * Returns mock support statistics for a project.
 *
 * @returns {ProjectSupportStats} Mock project support statistics.
 */
export const getMockProjectSupportStats = (): ProjectSupportStats => {
  return {
    activeChats: Math.floor(Math.random() * 10),
    resolvedChats: Math.floor(Math.random() * 30),
    sessionChats: Math.floor(Math.random() * 20),
    totalCases: Math.floor(Math.random() * 50),
  };
};

/**
 * Returns a random AI response from Novera.
 *
 * @returns {string} A random AI response.
 */
export const getNoveraResponse = (): string => {
  const responses = [
    "I understand the issue. Let me look into that for you.",
    "That sounds like a configuration problem. Have you checked your environment variables?",
    "I can help with that. Could you provide more details about the error message?",
    "I'm analyzing the logs now. One moment please.",
    "It seems like a known issue. I'll guide you through the fix.",
  ];
  return responses[Math.floor(Math.random() * responses.length)];
};

/**
 * Returns mock case creation metadata.
 *
 * @returns {CaseCreationMetadata} Mock case creation metadata.
 */
export const getCaseCreationMetadata = (): CaseCreationMetadata => {
  return mockCaseCreationMetadata;
};

/**
 * Returns a generated issue title.
 *
 * @returns {string} Generated issue title.
 */
export const getGeneratedIssueTitle = (): string => {
  return "Unstable API Manager Performance in Production";
};

/**
 * Returns a generated issue description based on conversation.
 *
 * @returns {string} Generated issue description.
 */
export const getGeneratedIssueDescription = (): string => {
  return "The customer is experiencing intermittent latencies in their Production environment. Based on our conversation, we have identified that this happens during peak load and might be related to thread pool exhaustion. They have already checked the basic logs but need deep analysis.";
};
