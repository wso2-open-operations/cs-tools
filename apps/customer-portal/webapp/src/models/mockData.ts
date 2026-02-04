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

import type {
  CaseListItem,
  ProjectListItem,
  UserProfile,
} from "@/models/responses";

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

// Mock cases for the cases table.
export const mockCases: CaseListItem[] = [
  {
    id: "case-001",
    internalId: "CUPRSUB-101",
    number: "CS0001001",
    createdOn: "2026-01-31 10:45:12",
    title: "Application crashes on startup",
    description:
      "App crashes immediately after launch with a null pointer error.",
    assignedEngineer: null,
    project: {
      id: "project-001",
      label: "Customer Portal – Subscription",
    },
    type: {
      id: "type-incident",
      label: "Incident",
    },
    deployment: {
      id: "deployment-prod",
      label: "Production",
    },
    severity: {
      id: "1",
      label: "Critical (P1)",
    },
    status: {
      id: "status-open",
      label: "Open",
    },
  },
  {
    id: "case-002",
    internalId: "CUPRSUB-102",
    number: "CS0001002",
    createdOn: "2026-01-30 18:20:05",
    title: "High latency in API Gateway",
    description: "Observed increased response times during peak hours.",
    assignedEngineer: "engineer-123",
    project: {
      id: "project-001",
      label: "Customer Portal – Subscription",
    },
    type: {
      id: "type-incident",
      label: "Incident",
    },
    deployment: null,
    severity: {
      id: "2",
      label: "High (P2)",
    },
    status: {
      id: "status-in-progress",
      label: "In Progress",
    },
  },
  {
    id: "case-003",
    internalId: "CUPRSUB-103",
    number: "CS0001003",
    createdOn: "2026-01-28 09:10:44",
    title: "Question about deployment options",
    description: "Need clarification on supported deployment environments.",
    assignedEngineer: null,
    project: {
      id: "project-002",
      label: "Customer Analytics Platform",
    },
    type: {
      id: "type-query",
      label: "Query",
    },
    deployment: null,
    severity: null,
    status: {
      id: "status-open",
      label: "Open",
    },
  },
  {
    id: "case-004",
    internalId: "CUPRSUB-104",
    number: "CS0001004",
    createdOn: "2026-01-25 14:30:00",
    title: "Request for new feature in dashboard",
    description: "User wants a custom widget for tracking API usage.",
    assignedEngineer: "engineer-456",
    project: {
      id: "project-001",
      label: "Customer Portal – Subscription",
    },
    type: {
      id: "type-feature",
      label: "Feature Request",
    },
    deployment: null,
    severity: {
      id: "3",
      label: "Low (P3)",
    },
    status: {
      id: "status-open",
      label: "Open",
    },
  },
  {
    id: "case-005",
    internalId: "CUPRSUB-105",
    number: "CS0001005",
    createdOn: "2026-01-22 11:15:20",
    title: "Login failure on staging",
    description:
      "Unable to login to staging environment with admin credentials.",
    assignedEngineer: null,
    project: {
      id: "project-003",
      label: "Internal Tools",
    },
    type: {
      id: "type-incident",
      label: "Incident",
    },
    deployment: {
      id: "deployment-staging",
      label: "Staging",
    },
    severity: {
      id: "2",
      label: "High (P2)",
    },
    status: {
      id: "status-open",
      label: "Open",
    },
  },
  {
    id: "case-006",
    internalId: "CUPRSUB-106",
    number: "CS0001006",
    createdOn: "2026-01-20 09:00:00",
    title: "Documentation update request",
    description: "Update API documentation for version 2.0.",
    assignedEngineer: "engineer-789",
    project: {
      id: "project-002",
      label: "Customer Analytics Platform",
    },
    type: {
      id: "type-task",
      label: "Task",
    },
    deployment: null,
    severity: {
      id: "4",
      label: "Minimal (P4)",
    },
    status: {
      id: "status-resolved",
      label: "Resolved",
    },
  },
];
