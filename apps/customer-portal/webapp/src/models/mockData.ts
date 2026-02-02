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

import type { ProjectListItem, UserProfile } from "@/models/responses";

// Interface for case creation metadata.
export interface CaseCreationMetadata {
  projects: string[];
  products: string[];
  deploymentTypes: string[];
  issueTypes: string[];
  severityLevels: {
    id: string;
    label: string;
    description: string;
  }[];
  conversationSummary: {
    messagesExchanged: number;
    troubleshootingAttempts: string;
    kbArticlesReviewed: string;
  };
}

// Sample projects for the project switcher.
export const mockProjects: ProjectListItem[] = [
  {
    id: "1890347890",
    key: "CON2026",
    name: "WSO2 Con App",
    description:
      "Official conference management app for WSO2 events, providing schedules, sessions, speaker details, and attendee engagement features.",
    createdOn: "2025-07-17 09:06:14",
  },
  {
    id: "27494027489",
    key: "SUPERAPP2026",
    name: "WSO2 Super App",
    description:
      "A unified platform that brings multiple WSO2 services and tools into a single application for seamless user experience.",
    createdOn: "2025-08-15 10:30:00",
  },
  {
    id: "3678392038",
    key: "PITSTOP2026",
    name: "WSO2 Pitstop",
    description:
      "An internal support and operations tool designed to streamline issue tracking, quick fixes, and service monitoring.",
    createdOn: "2025-09-02 14:15:22",
  },
  {
    id: "49830478390",
    key: "PARTNERPORTAL2026",
    name: "WSO2 Partner Portal",
    description:
      "A dedicated portal for WSO2 partners to manage collaborations, access resources, track integrations, and view partner analytics.",
    createdOn: "2025-10-10 08:45:10",
  },
];

// Mock status options for projects.
export const mockStatusOptions: string[] = [
  "All Good",
  "Need Attention",
  "Critical Issues",
];

// Mock open cases counts for projects.
export const mockOpenCasesOptions: number[] = [0, 5, 12, 24, 48, 72];

// Mock active chats counts for projects.
export const mockActiveChatsOptions: number[] = [0, 2, 5, 10, 15, 20];

// Sample user for the user menu.
export const mockUser: UserProfile = {
  name: "John Doe",
  email: "John@example.com",
  avatar: "JD",
  role: "Admin",
};

// Mock metadata for case creation.
export const mockCaseCreationMetadata: CaseCreationMetadata = {
  projects: [
    "Production Environment-Main",
    "Development Environment",
    "Staging environment",
  ],
  products: [
    "WSO2 API Manager - v4.2.0",
    "WSO2 API Manager - v4.1.0",
    "WSO2 Identity Server - v6.1.0",
    "WSO2 Identity Server - v6.0.0",
  ],
  deploymentTypes: ["Production", "Non-Production", "Development"],
  issueTypes: [
    "Total Outage",
    "Partial Outage",
    "Performance Degradation",
    "Question",
    "Security or Compliance",
    "Error",
  ],
  severityLevels: [
    {
      id: "S1",
      label: "S1 - Critical",
      description: "Business critical system down",
    },
    {
      id: "S2",
      label: "S2 - Medium",
      description: "Important features affected",
    },
    { id: "S3", label: "S3 - Low", description: "Minor issues" },
    { id: "S4", label: "S4 - Minimal", description: "General questions" },
  ],
  conversationSummary: {
    messagesExchanged: 8,
    troubleshootingAttempts: "2 steps completed",
    kbArticlesReviewed: "3 articles suggested",
  },
};
