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
  CaseDetails,
  CaseComment,
  CaseAttachment,
  ProjectListItem,
  ProjectDetails,
  UserDetails,
  Deployment,
  ChatHistoryResponse,
  CaseMetadataResponse,
  UpdatesStats,
  ProductUpdateLevelsResponse,
} from "@models/responses";
import {
  PROJECT_TYPE,
  SUPPORT_TIER,
  CASE_STATUS,
} from "@constants/projectDetailsConstants";

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

// Mock detailed project data including subscription information.
export const mockProjectDetails: ProjectDetails[] = [
  {
    id: "1890347890",
    key: "CON2026",
    name: "WSO2 Con App",
    description:
      "Official conference management app for WSO2 events, providing schedules, sessions, speaker details, and attendee engagement features.",
    createdOn: "2025-07-17 09:06:14",
    type: PROJECT_TYPE.FREE,
    subscription: {
      startDate: "2025-07-01",
      endDate: "2026-07-01",
      supportTier: SUPPORT_TIER.ENTERPRISE,
    },
  },
  {
    id: "27494027489",
    key: "SUPERAPP2026",
    name: "WSO2 Super App",
    description:
      "A unified platform that brings multiple WSO2 services and tools into a single application for seamless user experience.",
    createdOn: "2025-08-15 10:30:00",
    type: PROJECT_TYPE.SUBSCRIPTION,
    subscription: {
      startDate: "2025-08-01",
      endDate: "2026-08-01",
      supportTier: SUPPORT_TIER.ENTERPRISE,
    },
  },
  {
    id: "3678392038",
    key: "PITSTOP2026",
    name: "WSO2 Pitstop",
    description:
      "An internal support and operations tool designed to streamline issue tracking, quick fixes, and service monitoring.",
    createdOn: "2025-09-02 14:15:22",
    type: PROJECT_TYPE.SUBSCRIPTION,
    subscription: {
      startDate: "2025-09-01",
      endDate: "2026-09-01",
      supportTier: SUPPORT_TIER.STANDARD,
    },
  },
  {
    id: "49830478390",
    key: "PARTNERPORTAL2026",
    name: "WSO2 Partner Portal",
    description:
      "A dedicated portal for WSO2 partners to manage collaborations, access resources, track integrations, and view partner analytics.",
    createdOn: "2025-10-10 08:45:10",
    type: PROJECT_TYPE.SUBSCRIPTION,
    subscription: {
      startDate: "2025-10-01",
      endDate: "2026-10-01",
      supportTier: SUPPORT_TIER.ENTERPRISE,
    },
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

// Mock chat history returned when isMockEnabled (useGetChatHistory).
export const mockChatHistory: ChatHistoryResponse = {
  chatHistory: [
    {
      chatId: "1628192673",
      title: "How do I configure custom claims in JWT tokens?",
      startedTime: "2 hours ago",
      messages: 8,
      kbArticles: 3,
      status: "Resolved",
    },
    {
      chatId: "1628192674",
      title: "Getting error 401 when calling the API endpoint...",
      startedTime: "1 day ago",
      messages: 5,
      kbArticles: 2,
      status: "Still Open",
    },
    {
      chatId: "1628192675",
      title: "Need help understanding rate limiting configuration",
      startedTime: "2 days ago",
      messages: 12,
      kbArticles: 5,
      status: "Resolved",
    },
    {
      chatId: "1628192676",
      title: "Can I integrate with Azure AD for authentication?",
      startedTime: "3 days ago",
      messages: 6,
      kbArticles: 4,
      status: "Resolved",
    },
    {
      chatId: "1628192677",
      title: "Deployment best practices for high availability setup",
      startedTime: "5 days ago",
      messages: 15,
      kbArticles: 7,
      status: "Abandoned",
    },
  ],
};

export const mockUserDetails: UserDetails = {
  id: "c2d1c8961b2dfe50a002c9d3604bcba0",
  email: "John@example.com",
  lastName: "Doe",
  firstName: "John",
  timeZone: "--None--",
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
    "Error",
    "Partial Outage",
    "Performance Degradation",
    "Question",
    "Security or Compliance",
    "Total Outage",
  ],
  severityLevels: [
    { id: "60", label: "S0", description: "Business critical system down" },
    { id: "61", label: "S1", description: "Important features affected" },
    { id: "62", label: "S2", description: "Moderate impact" },
    { id: "63", label: "S3", description: "Minor issue" },
    { id: "64", label: "S4", description: "General question" },
  ],
  conversationSummary: {
    messagesExchanged: 8,
    troubleshootingAttempts: "2 steps completed",
    kbArticlesReviewed: "3 articles suggested",
  },
};

// Full mock metadata based on real response structure
export const mockCaseMetadata: CaseMetadataResponse = {
  statuses: [
    { id: "1", label: "Open" },
    { id: "10", label: "Work In Progress" },
    { id: "18", label: "Awaiting Info" },
    { id: "1003", label: "Waiting On WSO2" },
    { id: "6", label: "Solution Proposed" },
    { id: "3", label: "Closed" },
    { id: "1006", label: "Reopened" },
  ],
  severities: [
    { id: "60", label: "S0" },
    { id: "61", label: "S1" },
    { id: "62", label: "S2" },
    { id: "63", label: "S3" },
    { id: "64", label: "S4" },
  ],
  issueTypes: [
    { id: "6", label: "Error" },
    { id: "2", label: "Partial Outage" },
    { id: "3", label: "Performance Degradation" },
    { id: "4", label: "Question" },
    { id: "5", label: "Security or Compliance" },
    { id: "1", label: "Total Outage" },
  ],
  deployments: [
    { id: "1", label: "Development" },
    { id: "2", label: "Production" },
    { id: "3", label: "Non-Production" },
  ],
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
    issueType: {
      id: "6",
      label: "Error",
    },
    deployedProduct: null,
    deployment: {
      id: "2",
      label: "Production",
    },
    severity: {
      id: "60",
      label: "S0",
    },
    status: {
      id: "1",
      label: CASE_STATUS.OPEN,
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
    issueType: {
      id: "3",
      label: "Performance Degradation",
    },
    deployedProduct: null,
    deployment: {
      id: "2",
      label: "Production",
    },
    severity: {
      id: "61",
      label: "S1",
    },
    status: {
      id: "10",
      label: CASE_STATUS.WORK_IN_PROGRESS,
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
    issueType: {
      id: "4",
      label: "Question",
    },
    deployedProduct: null,
    deployment: null,
    severity: {
      id: "62",
      label: "S2",
    },
    status: {
      id: "1",
      label: CASE_STATUS.OPEN,
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
    issueType: {
      id: "4",
      label: "Question",
    },
    deployedProduct: null,
    deployment: null,
    severity: {
      id: "62",
      label: "S2",
    },
    status: {
      id: "1",
      label: CASE_STATUS.OPEN,
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
    issueType: {
      id: "3",
      label: "Performance Degradation",
    },
    deployedProduct: null,
    deployment: {
      id: "3",
      label: "Non-Production",
    },
    severity: {
      id: "61",
      label: "S1",
    },
    status: {
      id: "1",
      label: CASE_STATUS.OPEN,
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
    issueType: {
      id: "6",
      label: "Error",
    },
    deployedProduct: null,
    deployment: null,
    severity: {
      id: "63",
      label: "S3",
    },
    status: {
      id: "3",
      label: CASE_STATUS.CLOSED,
    },
  },
  {
    id: "case-007",
    internalId: "CUPRSUB-107",
    number: "CS0001007",
    createdOn: "2026-01-18 15:45:00",
    title: "Database connection timeout",
    description: "Occasional timeouts when connecting to the primary DB.",
    assignedEngineer: "engineer-123",
    project: {
      id: "project-001",
      label: "Customer Portal – Subscription",
    },
    issueType: {
      id: "3",
      label: "Performance Degradation",
    },
    deployedProduct: null,
    deployment: {
      id: "2",
      label: "Production",
    },
    severity: {
      id: "61",
      label: "S1",
    },
    status: {
      id: "1",
      label: CASE_STATUS.OPEN,
    },
  },
  {
    id: "case-008",
    internalId: "CUPRSUB-108",
    number: "CS0001008",
    createdOn: "2026-01-15 10:30:00",
    title: "Incorrect data in analytics report",
    description: "Users reporting mismatched totals in the monthly report.",
    assignedEngineer: null,
    project: {
      id: "project-002",
      label: "Customer Analytics Platform",
    },
    issueType: {
      id: "6",
      label: "Error",
    },
    deployedProduct: null,
    deployment: null,
    severity: {
      id: "62",
      label: "S2",
    },
    status: {
      id: "1",
      label: CASE_STATUS.OPEN,
    },
  },
  {
    id: "case-009",
    internalId: "CUPRSUB-109",
    number: "CS0001009",
    createdOn: "2026-01-12 14:20:00",
    title: "UI glitch in mobile view",
    description: "Header overlaps content on smaller screens.",
    assignedEngineer: "engineer-456",
    project: {
      id: "project-001",
      label: "Customer Portal – Subscription",
    },
    issueType: {
      id: "6",
      label: "Error",
    },
    deployedProduct: null,
    deployment: null,
    severity: {
      id: "63",
      label: "S3",
    },
    status: {
      id: "1",
      label: CASE_STATUS.OPEN,
    },
  },
  {
    id: "case-010",
    internalId: "CUPRSUB-110",
    number: "CS0001010",
    createdOn: "2026-01-10 09:15:00",
    title: "Request for API quota increase",
    description: "Need higher rate limits for the staging environment.",
    assignedEngineer: null,
    project: {
      id: "project-003",
      label: "Internal Tools",
    },
    issueType: {
      id: "1",
      label: "Total Outage",
    },
    deployedProduct: null,
    deployment: {
      id: "3",
      label: "Non-Production",
    },
    severity: {
      id: "62",
      label: "S2",
    },
    status: {
      id: "1",
      label: CASE_STATUS.OPEN,
    },
  },
  {
    id: "case-011",
    internalId: "CUPRSUB-111",
    number: "CS0001011",
    createdOn: "2026-01-08 11:00:00",
    title: "Broken link in footer",
    description: "The 'Privacy Policy' link leads to a 404 page.",
    assignedEngineer: "engineer-789",
    project: {
      id: "project-001",
      label: "Customer Portal – Subscription",
    },
    issueType: {
      id: "6",
      label: "Error",
    },
    deployedProduct: null,
    deployment: null,
    severity: {
      id: "63",
      label: "S3",
    },
    status: {
      id: "3",
      label: CASE_STATUS.CLOSED,
    },
  },
  {
    id: "case-012",
    internalId: "CUPRSUB-112",
    number: "CS0001012",
    createdOn: "2026-01-05 16:30:00",
    title: "Performance issue in search indexing",
    description: "Indexing process takes much longer than expected.",
    assignedEngineer: "engineer-123",
    project: {
      id: "project-002",
      label: "Customer Analytics Platform",
    },
    issueType: {
      id: "3",
      label: "Performance Degradation",
    },
    deployedProduct: null,
    deployment: null,
    severity: {
      id: "61",
      label: "S1",
    },
    status: {
      id: "10",
      label: CASE_STATUS.WORK_IN_PROGRESS,
    },
  },
  {
    id: "case-013",
    internalId: "CUPRSUB-113",
    number: "CS0001013",
    createdOn: "2026-01-02 13:00:00",
    title: "New feature: Export to Excel",
    description: "Requested ability to export analytics data to CSV/Excel.",
    assignedEngineer: null,
    project: {
      id: "project-002",
      label: "Customer Analytics Platform",
    },
    issueType: {
      id: "4",
      label: "Question",
    },
    deployedProduct: null,
    deployment: null,
    severity: {
      id: "62",
      label: "S2",
    },
    status: {
      id: "1",
      label: CASE_STATUS.OPEN,
    },
  },
  {
    id: "case-014",
    internalId: "CUPRSUB-114",
    number: "CS0001014",
    createdOn: "2025-12-30 10:00:00",
    title: "Security vulnerability report",
    description: "Potential XSS found in comments section.",
    assignedEngineer: "engineer-456",
    project: {
      id: "project-001",
      label: "Customer Portal – Subscription",
    },
    issueType: {
      id: "5",
      label: "Security or Compliance",
    },
    deployedProduct: null,
    deployment: {
      id: "2",
      label: "Production",
    },
    severity: {
      id: "60",
      label: "S0",
    },
    status: {
      id: "1",
      label: CASE_STATUS.OPEN,
    },
  },
  {
    id: "case-015",
    internalId: "CUPRSUB-115",
    number: "CS0001015",
    createdOn: "2025-12-28 09:00:00",
    title: "Email notification failure",
    description: "Welcome emails not being sent to new signups.",
    assignedEngineer: null,
    project: {
      id: "project-001",
      label: "Customer Portal – Subscription",
    },
    issueType: {
      id: "3",
      label: "Performance Degradation",
    },
    deployedProduct: null,
    deployment: null,
    severity: {
      id: "61",
      label: "S1",
    },
    status: {
      id: "1",
      label: CASE_STATUS.OPEN,
    },
  },
  {
    id: "case-016",
    internalId: "CUPRSUB-116",
    number: "CS0001016",
    createdOn: "2025-12-25 14:00:00",
    title: "Server maintenance notification",
    description: "Planned maintenance for high-availability cluster.",
    assignedEngineer: "engineer-789",
    project: {
      id: "project-003",
      label: "Internal Tools",
    },
    issueType: {
      id: "6",
      label: "Error",
    },
    deployedProduct: null,
    deployment: null,
    severity: {
      id: "63",
      label: "S3",
    },
    status: {
      id: "3",
      label: CASE_STATUS.CLOSED,
    },
  },
  {
    id: "case-017",
    internalId: "CUPRSUB-117",
    number: "CS0001017",
    createdOn: "2025-12-22 11:00:00",
    title: "Slow load times on dashboard",
    description: "Dashboards taking more than 5 seconds to fully load.",
    assignedEngineer: "engineer-123",
    project: {
      id: "project-001",
      label: "Customer Portal – Subscription",
    },
    issueType: {
      id: "6",
      label: "Error",
    },
    deployedProduct: null,
    deployment: null,
    severity: {
      id: "62",
      label: "S2",
    },
    status: {
      id: "6",
      label: CASE_STATUS.SOLUTION_PROPOSED,
    },
  },
  {
    id: "case-018",
    internalId: "CUPRSUB-118",
    number: "CS0001018",
    createdOn: "2025-12-20 09:30:00",
    title: "SSL certificate expiry warning",
    description: "Certificate for staging cluster expires in 15 days.",
    assignedEngineer: null,
    project: {
      id: "project-003",
      label: "Internal Tools",
    },
    issueType: {
      id: "6",
      label: "Error",
    },
    deployedProduct: null,
    deployment: {
      id: "3",
      label: "Non-Production",
    },
    severity: {
      id: "61",
      label: "S1",
    },
    status: {
      id: "1",
      label: CASE_STATUS.OPEN,
    },
  },
  {
    id: "case-019",
    internalId: "CUPRSUB-119",
    number: "CS0001019",
    createdOn: "2025-12-18 15:00:00",
    title: "Localization issue: Japanese translation",
    description: "Several buttons have incorrect Japanese text.",
    assignedEngineer: "engineer-456",
    project: {
      id: "project-001",
      label: "Customer Portal – Subscription",
    },
    issueType: {
      id: "6",
      label: "Error",
    },
    deployedProduct: null,
    deployment: null,
    severity: {
      id: "63",
      label: "S3",
    },
    status: {
      id: "1",
      label: CASE_STATUS.OPEN,
    },
  },
  {
    id: "case-020",
    internalId: "CUPRSUB-120",
    number: "CS0001020",
    createdOn: "2025-12-15 10:00:00",
    title: "Unauthorized access attempt alert",
    description: "Suspicious activity detected on user accounts.",
    assignedEngineer: "engineer-789",
    project: {
      id: "project-001",
      label: "Customer Portal – Subscription",
    },
    issueType: {
      id: "3",
      label: "Performance Degradation",
    },
    deployedProduct: null,
    deployment: {
      id: "2",
      label: "Production",
    },
    severity: {
      id: "61",
      label: "S1",
    },
    status: {
      id: "1",
      label: CASE_STATUS.OPEN,
    },
  },
  {
    id: "case-021",
    internalId: "CUPRSUB-121",
    number: "CS0001021",
    createdOn: "2025-12-12 14:00:00",
    title: "System downtime summary",
    description: "Post-mortem analysis of last week's outage.",
    assignedEngineer: null,
    project: {
      id: "project-002",
      label: "Customer Analytics Platform",
    },
    issueType: {
      id: "6",
      label: "Error",
    },
    deployedProduct: null,
    deployment: null,
    severity: {
      id: "62",
      label: "S2",
    },
    status: {
      id: "3",
      label: CASE_STATUS.CLOSED,
    },
  },
  {
    id: "case-022",
    internalId: "CUPRSUB-122",
    number: "CS0001022",
    createdOn: "2025-12-10 11:30:00",
    title: "Deployment failure in QA",
    description: "Automated deployment pipeline failed for QA environment.",
    assignedEngineer: "engineer-123",
    project: {
      id: "project-001",
      label: "Customer Portal – Subscription",
    },
    issueType: {
      id: "6",
      label: "Error",
    },
    deployedProduct: null,
    deployment: {
      id: "2",
      label: "Production",
    },
    severity: {
      id: "61",
      label: "S1",
    },
    status: {
      id: "1",
      label: CASE_STATUS.OPEN,
    },
  },
  {
    id: "case-023",
    internalId: "CUPRSUB-123",
    number: "CS0001023",
    createdOn: "2025-12-08 09:45:00",
    title: "Feature request: Dark mode",
    description: "Users are requesting dark mode support for the dashboard.",
    assignedEngineer: null,
    project: {
      id: "project-002",
      label: "Customer Analytics Platform",
    },
    issueType: {
      id: "4",
      label: "Question",
    },
    deployedProduct: null,
    deployment: null,
    severity: {
      id: "63",
      label: "S3",
    },
    status: {
      id: "1",
      label: CASE_STATUS.OPEN,
    },
  },
  {
    id: "case-024",
    internalId: "CUPRSUB-124",
    number: "CS0001024",
    createdOn: "2025-12-05 16:20:00",
    title: "Slow query performance",
    description: "Analytics queries taking longer than expected.",
    assignedEngineer: "engineer-456",
    project: {
      id: "project-002",
      label: "Customer Analytics Platform",
    },
    issueType: {
      id: "3",
      label: "Performance Degradation",
    },
    deployedProduct: null,
    deployment: null,
    severity: {
      id: "62",
      label: "S2",
    },
    status: {
      id: "10",
      label: CASE_STATUS.WORK_IN_PROGRESS,
    },
  },
  {
    id: "case-025",
    internalId: "CUPRSUB-125",
    number: "CS0001025",
    createdOn: "2025-12-03 14:10:00",
    title: "Login issue with SSO",
    description: "Users unable to login using Google SSO.",
    assignedEngineer: null,
    project: {
      id: "project-001",
      label: "Customer Portal – Subscription",
    },
    issueType: {
      id: "6",
      label: "Error",
    },
    deployedProduct: null,
    deployment: null,
    severity: {
      id: "60",
      label: "S0",
    },
    status: {
      id: "1",
      label: CASE_STATUS.OPEN,
    },
  },
  {
    id: "case-026",
    internalId: "CUPRSUB-126",
    number: "CS0001026",
    createdOn: "2025-11-30 10:00:00",
    title: "API documentation error",
    description: "Found incorrect parameter description in v1 docs.",
    assignedEngineer: "engineer-789",
    project: {
      id: "project-003",
      label: "Internal Tools",
    },
    issueType: {
      id: "6",
      label: "Error",
    },
    deployedProduct: null,
    deployment: null,
    severity: {
      id: "63",
      label: "S3",
    },
    status: {
      id: "3",
      label: CASE_STATUS.CLOSED,
    },
  },
  {
    id: "case-027",
    internalId: "CUPRSUB-127",
    number: "CS0001027",
    createdOn: "2025-11-28 13:45:00",
    title: "Request for backup restore",
    description: "Need to restore database backup from previous week.",
    assignedEngineer: null,
    project: {
      id: "project-001",
      label: "Customer Portal – Subscription",
    },
    issueType: {
      id: "1",
      label: "Total Outage",
    },
    deployedProduct: null,
    deployment: {
      id: "2",
      label: "Production",
    },
    severity: {
      id: "61",
      label: "S1",
    },
    status: {
      id: "1",
      label: CASE_STATUS.OPEN,
    },
  },
  {
    id: "case-028",
    internalId: "CUPRSUB-128",
    number: "CS0001028",
    createdOn: "2025-11-25 09:30:00",
    title: "User permission issue",
    description: "Client cannot access project settings.",
    assignedEngineer: "engineer-123",
    project: {
      id: "project-002",
      label: "Customer Analytics Platform",
    },
    issueType: {
      id: "6",
      label: "Error",
    },
    deployedProduct: null,
    deployment: null,
    severity: {
      id: "62",
      label: "S2",
    },
    status: {
      id: "10",
      label: CASE_STATUS.WORK_IN_PROGRESS,
    },
  },
  {
    id: "case-029",
    internalId: "CUPRSUB-129",
    number: "CS0001029",
    createdOn: "2025-11-22 15:15:00",
    title: "Integration with Slack",
    description: "Requesting help to set up Slack notifications.",
    assignedEngineer: null,
    project: {
      id: "project-003",
      label: "Internal Tools",
    },
    issueType: {
      id: "4",
      label: "Question",
    },
    deployedProduct: null,
    deployment: null,
    severity: {
      id: "63",
      label: "S3",
    },
    status: {
      id: "1",
      label: CASE_STATUS.OPEN,
    },
  },
  {
    id: "case-030",
    internalId: "CUPRSUB-130",
    number: "CS0001030",
    createdOn: "2025-11-20 11:00:00",
    title: "Server 500 error on checkout",
    description: "Checkout process failing intermittently.",
    assignedEngineer: "engineer-456",
    project: {
      id: "project-001",
      label: "Customer Portal – Subscription",
    },
    issueType: {
      id: "6",
      label: "Error",
    },
    deployedProduct: null,
    deployment: {
      id: "2",
      label: "Production",
    },
    severity: {
      id: "60",
      label: "S0",
    },
    status: {
      id: "1",
      label: CASE_STATUS.OPEN,
    },
  },
  {
    id: "case-031",
    internalId: "CUPRSUB-131",
    number: "CS0001031",
    createdOn: "2025-11-18 14:30:00",
    title: "Incorrect billing amount",
    description: "Customer charged twice for subscription.",
    assignedEngineer: null,
    project: {
      id: "project-002",
      label: "Customer Analytics Platform",
    },
    issueType: {
      id: "6",
      label: "Error",
    },
    deployedProduct: null,
    deployment: null,
    severity: {
      id: "61",
      label: "S1",
    },
    status: {
      id: "1",
      label: CASE_STATUS.OPEN,
    },
  },
  {
    id: "case-032",
    internalId: "CUPRSUB-132",
    number: "CS0001032",
    createdOn: "2025-11-15 10:45:00",
    title: "SSL certificate expiry warning",
    description: "Certificate expiring in 3 days.",
    assignedEngineer: "engineer-789",
    project: {
      id: "project-001",
      label: "Customer Portal – Subscription",
    },
    issueType: {
      id: "3",
      label: "Performance Degradation",
    },
    deployedProduct: null,
    deployment: {
      id: "2",
      label: "Production",
    },
    severity: {
      id: "61",
      label: "S1",
    },
    status: {
      id: "10",
      label: CASE_STATUS.WORK_IN_PROGRESS,
    },
  },
  {
    id: "case-033",
    internalId: "CUPRSUB-133",
    number: "CS0001033",
    createdOn: "2025-11-12 16:00:00",
    title: "Dashboard loading slow",
    description: "Takes over 10 seconds to load main dashboard.",
    assignedEngineer: null,
    project: {
      id: "project-003",
      label: "Internal Tools",
    },
    issueType: {
      id: "3",
      label: "Performance Degradation",
    },
    deployedProduct: null,
    deployment: null,
    severity: {
      id: "62",
      label: "S2",
    },
    status: {
      id: "1",
      label: CASE_STATUS.OPEN,
    },
  },
  {
    id: "case-034",
    internalId: "CUPRSUB-134",
    number: "CS0001034",
    createdOn: "2025-11-10 13:20:00",
    title: "Typo in footer",
    description: "Copyright year is incorrect.",
    assignedEngineer: "engineer-123",
    project: {
      id: "project-001",
      label: "Customer Portal – Subscription",
    },
    issueType: {
      id: "6",
      label: "Error",
    },
    deployedProduct: null,
    deployment: null,
    severity: {
      id: "63",
      label: "S3",
    },
    status: {
      id: "3",
      label: CASE_STATUS.CLOSED,
    },
  },
  {
    id: "case-035",
    internalId: "CUPRSUB-135",
    number: "CS0001035",
    createdOn: "2025-11-08 11:50:00",
    title: "Request for log access",
    description: "Need access to server logs for debugging.",
    assignedEngineer: null,
    project: {
      id: "project-002",
      label: "Customer Analytics Platform",
    },
    issueType: {
      id: "4",
      label: "Question",
    },
    deployedProduct: null,
    deployment: null,
    severity: {
      id: "62",
      label: "S2",
    },
    status: {
      id: "1",
      label: CASE_STATUS.OPEN,
    },
  },
  {
    id: "case-036",
    internalId: "CUPRSUB-136",
    number: "CS0001036",
    createdOn: "2025-11-05 09:10:00",
    title: "Search functionality broken",
    description: "Search returns no results for valid queries.",
    assignedEngineer: "engineer-456",
    project: {
      id: "project-001",
      label: "Customer Portal – Subscription",
    },
    issueType: {
      id: "6",
      label: "Error",
    },
    deployedProduct: null,
    deployment: {
      id: "2",
      label: "Production",
    },
    severity: {
      id: "61",
      label: "S1",
    },
    status: {
      id: "1",
      label: CASE_STATUS.OPEN,
    },
  },
  {
    id: "case-037",
    internalId: "CUPRSUB-137",
    number: "CS0001037",
    createdOn: "2025-11-03 14:40:00",
    title: "Export to PDF failure",
    description: "PDF export crashes for large reports.",
    assignedEngineer: null,
    project: {
      id: "project-003",
      label: "Internal Tools",
    },
    issueType: {
      id: "6",
      label: "Error",
    },
    deployedProduct: null,
    deployment: null,
    severity: {
      id: "62",
      label: "S2",
    },
    status: {
      id: "1",
      label: CASE_STATUS.OPEN,
    },
  },
  {
    id: "case-038",
    internalId: "CUPRSUB-138",
    number: "CS0001038",
    createdOn: "2025-11-01 10:25:00",
    title: "New user onboarding issue",
    description: "Welcome email not being sent.",
    assignedEngineer: "engineer-789",
    project: {
      id: "project-002",
      label: "Customer Analytics Platform",
    },
    issueType: {
      id: "6",
      label: "Error",
    },
    deployedProduct: null,
    deployment: null,
    severity: {
      id: "61",
      label: "S1",
    },
    status: {
      id: "10",
      label: CASE_STATUS.WORK_IN_PROGRESS,
    },
  },
  {
    id: "case-039",
    internalId: "CUPRSUB-139",
    number: "CS0001039",
    createdOn: "2025-10-30 16:50:00",
    title: "Mobile app crash on iOS 18",
    description: "Crash observed only on latest iOS version.",
    assignedEngineer: null,
    project: {
      id: "project-001",
      label: "Customer Portal – Subscription",
    },
    issueType: {
      id: "6",
      label: "Error",
    },
    deployedProduct: null,
    deployment: {
      id: "2",
      label: "Production",
    },
    severity: {
      id: "60",
      label: "S0",
    },
    status: {
      id: "1",
      label: CASE_STATUS.OPEN,
    },
  },
  {
    id: "case-040",
    internalId: "CUPRSUB-140",
    number: "CS0001040",
    createdOn: "2025-10-28 12:15:00",
    title: "Request for custom domain",
    description: "Client wants to use their own domain for portal.",
    assignedEngineer: "engineer-123",
    project: {
      id: "project-003",
      label: "Internal Tools",
    },
    issueType: {
      id: "4",
      label: "Question",
    },
    deployedProduct: null,
    deployment: null,
    severity: {
      id: "63",
      label: "S3",
    },
    status: {
      id: "3",
      label: CASE_STATUS.CLOSED,
    },
  },
  {
    id: "case-041",
    internalId: "CUPRSUB-141",
    number: "CS0001041",
    createdOn: "2025-10-25 09:00:00",
    title: "Database migration plan",
    description: "Planning for next month's DB upgrade.",
    assignedEngineer: null,
    project: {
      id: "project-002",
      label: "Customer Analytics Platform",
    },
    issueType: {
      id: "4",
      label: "Question",
    },
    deployedProduct: null,
    deployment: null,
    severity: {
      id: "62",
      label: "S2",
    },
    status: {
      id: "1",
      label: CASE_STATUS.OPEN,
    },
  },
];

// Mock case details.
export const mockCaseDetails: CaseDetails = {
  id: "case-001",
  internalId: "CUPRSUB-101",
  number: "CS0001001",
  createdOn: "2026-01-31 10:45:12",
  updatedOn: "2026-02-10 23:47:57",
  title: "Application crashes on startup",
  description:
    "App crashes immediately after launch with a null pointer error.",
  slaResponseTime: "129671000",
  product: null,
  account: {
    type: null,
    id: "9460f8a91bfaa694a002c9d3604bcbbb",
    name: "Customer 3i",
  },
  csManager: null,
  assignedEngineer: null,
  project: {
    id: "project-001",
    name: "Customer Portal – Subscription",
  },
  deployment: {
    id: "deployment-prod",
    label: "Production",
  },
  deployedProduct: null,
  issueType: null,
  state: { id: 1, label: "Open" },
  severity: { id: 60, label: "S0" },
};

// Mock case comments.
export const mockCaseComments: CaseComment[] = [
  {
    id: "1398232c1bceb290a002c9d3604bcb27",
    content:
      '[code]<p>Test comment</p><p><img src="/db98232c1bceb290a002c9d3604bcb27.iix"></p><p><br></p><p>test</p>[/code]',
    type: "comments",
    createdOn: "2025-12-23 14:49:58",
    createdBy: "para-admin@wso2.com",
    isEscalated: false,
  },
  {
    id: "712727a81bceb290a002c9d3604bcbcc",
    content:
      '[code]<p>www</p><p><img src="/3d2727a81bceb290a002c9d3604bcbcc.iix"></p>[/code]',
    type: "comments",
    createdOn: "2025-12-23 14:43:36",
    createdBy: "para-admin@wso2.com",
    isEscalated: false,
  },
  {
    id: "61f6a7a81bceb290a002c9d3604bcb53",
    content:
      "[code]<br><b> <u>Description</u> </b><br><p><p>Test Description</p></p>[/code]",
    type: "comments",
    createdOn: "2025-12-23 14:42:46",
    createdBy: "para-admin@wso2.com",
    isEscalated: false,
  },
];

// Mock case attachments returned when isMockEnabled (useGetCaseAttachments).
export const mockCaseAttachments: CaseAttachment[] = [
  {
    id: "att-001",
    name: "screenshot-error.png",
    type: "image/png",
    sizeBytes: "245760",
    downloadUrl: "https://example.com/files/att-001",
    createdOn: "2025-12-23 14:49:58",
    createdBy: "para-admin@wso2.com",
  },
  {
    id: "att-002",
    name: "logs-debug.txt",
    type: "text/plain",
    sizeBytes: "10240",
    downloadUrl: "https://example.com/files/att-002",
    createdOn: "2025-12-23 14:43:36",
    createdBy: "para-admin@wso2.com",
  },
  {
    id: "att-003",
    name: "config-backup.zip",
    type: "application/zip",
    sizeBytes: "524288",
    downloadUrl: "https://example.com/files/att-003",
    createdOn: "2025-12-23 14:42:46",
    createdBy: "para-admin@wso2.com",
  },
];

// Mock deployments for project detail Deployments tab.
export const mockDeployments: Deployment[] = [
  {
    id: "dep-001",
    name: "Production",
    status: "Healthy",
    url: "https://api.dreamworks.com",
    version: "v2.5.1",
    description: "Primary production environment serving live traffic",
    products: [
      {
        id: "product-dep-001-prod-001",
        name: "WSO2 API Manager",
        version: "4.2.0",
        supportStatus: "Active Support",
        description: "API Gateway and management platform",
        cores: 8,
        tps: 5000,
        releasedDate: "2023-05-15",
        endOfLifeDate: "2026-05-15",
        updateLevel: "U22",
      },
      {
        id: "product-dep-001-prod-002",
        name: "WSO2 Identity Server",
        version: "6.1.0",
        supportStatus: "Active Support",
        description: "Identity and access management",
        cores: 4,
        tps: 2000,
        releasedDate: "2023-08-20",
        endOfLifeDate: "2026-08-20",
        updateLevel: "U15",
      },
    ],
    documents: [
      {
        id: "doc-dep-001-1",
        name: "Production Deployment Architecture.pdf",
        category: "Architecture",
        sizeBytes: 2453606,
        uploadedAt: "2026-02-03",
        uploadedBy: "John Doe",
      },
      {
        id: "doc-dep-001-2",
        name: "API Gateway Deployment Diagram.png",
        category: "Deployment Diagram",
        sizeBytes: 1236992,
        uploadedAt: "2026-02-05",
        uploadedBy: "Sarah Chen",
      },
      {
        id: "doc-dep-001-3",
        name: "Production Test Cases.xlsx",
        category: "Test Case",
        sizeBytes: 987738,
        uploadedAt: "2026-02-07",
        uploadedBy: "Mike Johnson",
      },
    ],
    deployedAt: "2026-02-08",
    uptimePercent: 99.98,
  },
  {
    id: "dep-002",
    name: "QA Environment",
    status: "Healthy",
    url: "https://qa-api.dreamworks.com",
    version: "v2.6.0-rc1",
    description: "Quality assurance and testing environment",
    products: [
      {
        id: "product-dep-002-prod-003",
        name: "WSO2 API Manager",
        version: "4.2.0",
        supportStatus: "Active Support",
        description: "API Gateway for testing",
        cores: 4,
        tps: 1000,
        releasedDate: "2023-05-15",
        endOfLifeDate: "2026-05-15",
        updateLevel: "U22",
      },
    ],
    documents: [
      {
        id: "doc-dep-002-1",
        name: "QA Environment Setup.docx",
        category: "Configuration",
        sizeBytes: 456440,
        uploadedAt: "2026-02-08",
        uploadedBy: "Alex Kumar",
      },
    ],
    deployedAt: "2026-02-09",
    uptimePercent: 99.95,
  },
  {
    id: "dep-003",
    name: "Development",
    status: "Warning",
    url: "https://dev-api.dreamworks.com",
    version: "v2.7.0-dev",
    description: "Development and integration environment",
    products: [],
    documents: [],
    deployedAt: "2026-02-10",
    uptimePercent: 98.5,
  },
];

// Mock updates statistics (used when isMockEnabled for useGetProductUpdatesStats).
export const mockUpdatesStats: UpdatesStats = {
  productsTracked: 4,
  totalUpdatesInstalled: 70,
  totalUpdatesInstalledBreakdown: { regular: 50, security: 20 },
  totalUpdatesPending: 69,
  totalUpdatesPendingBreakdown: { regular: 37, security: 32 },
  securityUpdatesPending: 32,
};

// Mock product update levels (used when isMockEnabled for useGetProductUpdateLevels).
export const mockProductUpdateLevels: ProductUpdateLevelsResponse = [
  {
    "product-name": "wso2das",
    "product-update-levels": [
      {
        "product-base-version": "3.2.0",
        channel: "full",
        "update-levels": [1, 2, 3, 4, 5, 6, 7, 8, 9, 10],
      },
      {
        "product-base-version": "3.1.0",
        channel: "full",
        "update-levels": [1, 2, 3, 4, 5, 6, 7, 8, 9],
      },
    ],
  },
  {
    "product-name": "choreo-connect",
    "product-update-levels": [
      {
        "product-base-version": "1.2.0",
        channel: "full",
        "update-levels": [1],
      },
      {
        "product-base-version": "1.0.0",
        channel: "full",
        "update-levels": [1, 2, 3, 4, 5, 6, 7, 8, 9, 10],
      },
    ],
  },
  {
    "product-name": "wso2mi",
    "product-update-levels": [
      {
        "product-base-version": "1.2.0",
        channel: "full",
        "update-levels": [1, 2, 3, 4, 5, 6, 7, 8, 9, 10],
      },
    ],
  },
];
