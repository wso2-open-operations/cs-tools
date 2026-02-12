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
  mockChatHistory,
  mockOpenCasesOptions,
  mockDeployments,
  mockStatusOptions,
  mockUpdatesStats,
  mockProductUpdateLevels,
} from "@models/mockData";
import type { CaseCreationMetadata } from "@models/mockData";

import type {
  ProjectCasesStats,
  ProjectSupportStats,
  ProjectTimeTrackingStats,
  DashboardMockStats,
  ProjectStatsResponse,
  ChatHistoryResponse,
  DeploymentsResponse,
  UpdatesStats,
  ProductUpdateLevelsResponse,
  CaseClassificationResponse,
} from "@models/responses";
import type { CaseClassificationRequest } from "@models/requests";

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
 * Returns mock case statistics for a project.
 *
 * @returns {ProjectCasesStats} Mock project case statistics.
 */
export const getMockProjectCasesStats = (): ProjectCasesStats => {
  const workInProgress = 1;
  const waitingOnClient = 0;
  const waitingOnWso2 = 1;

  const medium = 15;
  const high = 0;
  const critical = 0;

  return {
    activeCases: {
      total: workInProgress + waitingOnClient + waitingOnWso2,
      waitingOnClient,
      waitingOnWso2,
      workInProgress,
    },
    averageResponseTime: 0.0,
    openCases: 58,
    outstandingCases: {
      critical,
      high,
      medium,
      total: medium + high + critical,
    },
    resolvedCases: {
      currentMonth: 0,
      total: 1,
    },
    totalCases: 61,
  };
};

/**
 * Returns mock time tracking statistics for a project.
 *
 * @returns {ProjectTimeTrackingStats} Mock project time tracking statistics.
 */
export const getMockProjectTimeTrackingStats =
  (): ProjectTimeTrackingStats => ({
    totalHours: 17.5,
    billableHours: 15,
    nonBillableHours: 2.5,
  });

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

/**
 * Returns mock dashboard statistics.
 *
 * @returns {DashboardMockStats} Mock dashboard statistics.
 */
export const getMockDashboardStats = (): DashboardMockStats => {
  return {
    totalCases: {
      value: 156,
      trend: { value: "12%", direction: "up", color: "success" },
    },
    openCases: {
      value: 42,
      trend: { value: "5%", direction: "down", color: "error" },
    },
    resolvedCases: {
      value: 114,
      trend: { value: "8%", direction: "up", color: "success" },
    },
    avgResponseTime: {
      value: "4.5h",
      trend: { value: "0.5h", direction: "down", color: "error" },
    },
    casesTrend: [
      {
        name: "Jan",
        TypeA: 400,
        TypeB: 240,
        TypeC: 240,
        TypeD: 240,
      },
      {
        name: "Feb",
        TypeA: 300,
        TypeB: 139,
        TypeC: 221,
        TypeD: 221,
      },
      {
        name: "Mar",
        TypeA: 200,
        TypeB: 980,
        TypeC: 229,
        TypeD: 229,
      },
      {
        name: "Apr",
        TypeA: 278,
        TypeB: 390,
        TypeC: 200,
        TypeD: 200,
      },
      {
        name: "May",
        TypeA: 189,
        TypeB: 480,
        TypeC: 218,
        TypeD: 218,
      },
      {
        name: "Jun",
        TypeA: 239,
        TypeB: 380,
        TypeC: 250,
        TypeD: 250,
      },
      {
        name: "Jul",
        TypeA: 349,
        TypeB: 430,
        TypeC: 210,
        TypeD: 210,
      },
    ],
  };
};

/**
 * Returns mock chat history for a project.
 *
 * @returns {ChatHistoryResponse} Mock chat history list.
 */
export const getMockChatHistory = (): ChatHistoryResponse => {
  return mockChatHistory;
};

/**
 * Returns mock project statistics.
 *
 * @returns {ProjectStatsResponse} Mock project statistics.
 */
export const getMockProjectStats = (): ProjectStatsResponse => {
  return {
    projectStats: {
      activeChats: Math.floor(Math.random() * 10),
      deployments: Math.floor(Math.random() * 20),
      openCases: Math.floor(Math.random() * 15),
      slaStatus: Math.random() > 0.5 ? "All Good" : "Critical Issues",
    },
    recentActivity: {
      billableHours: Math.floor(Math.random() * 100),
      lastDeploymentOn: new Date().toISOString(),
      systemHealth: Math.random() > 0.8 ? "Critical" : "Healthy",
      totalTimeLogged: Math.floor(Math.random() * 200),
    },
  };
};

/**
 * Returns mock deployments for a project (used when isMockEnabled).
 *
 * @param {string} [_projectId] - Optional project ID; currently returns same list for all projects.
 * @returns {DeploymentsResponse} Mock deployments list.
 */
export const getMockDeployments = (
  _projectId?: string,
): DeploymentsResponse => {
  return { deployments: mockDeployments };
};

/**
 * Returns mock updates statistics (used when isMockEnabled for useGetProductUpdatesStats).
 *
 * @returns {UpdatesStats} Mock updates statistics.
 */
export const getMockUpdatesStats = (): UpdatesStats => mockUpdatesStats;

/**
 * Returns mock product update levels (used when isMockEnabled for useGetProductUpdateLevels).
 *
 * @returns {ProductUpdateLevelsResponse} Mock product update levels.
 */
export const getMockProductUpdateLevels =
  (): ProductUpdateLevelsResponse => mockProductUpdateLevels;

/**
 * Returns mock case classification response.
 *
 * @param {CaseClassificationRequest} request - Classification request body.
 * @returns {CaseClassificationResponse} Mock classification response.
 */
export const getMockCaseClassification = (
  request: CaseClassificationRequest,
): CaseClassificationResponse => {
  const firstProduct = request.productDetails[0] || "";
  const [productName, productVersion] = firstProduct
    ? firstProduct.split(" - ").map((value) => value.trim())
    : ["", ""];

  return {
    issueType: "Question",
    severityLevel: "S4",
    case_info: {
      description:
        "I am using WSO2 Identity Server with a .NET 8 application and need to configure custom claims in JWT tokens to include user roles and custom organization data.",
      shortDescription:
        "Need help configuring custom claims in JWT tokens using WSO2 Identity Server.",
      productName,
      productVersion: productVersion || "",
      environment: request.environments[0] || "",
      tier: request.tier,
      region: request.region,
    },
  };
};
