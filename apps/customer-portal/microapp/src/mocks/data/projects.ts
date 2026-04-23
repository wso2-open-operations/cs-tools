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

import type { ProjectCardProps } from "@components/features/projects";

export const MOCK_PROJECTS: Omit<ProjectCardProps, "onClick">[] = [
  {
    id: "DREAMSUB",
    name: "Dreamworks Inc",
    description: "Production deployment of API Manager for e-commerce platform",
    type: "Managed Cloud",
    status: "All Good",
    numberOfOpenCases: 3,
    metrics: {
      cases: 3,
      chats: 2,
      service: 3,
      change: 2,
      users: 12,
      date: "Jan 2024",
    },
  },
  {
    id: "NEWSLINEENSUB",
    name: "Newsline Enterprise",
    description: "Staging environment for Identity Server integration testing",
    type: "Regular",
    status: "All Good",
    numberOfOpenCases: 1,
    metrics: {
      cases: 1,
      chats: 0,
      users: 5,
      date: "Mar 2024",
    },
  },
  {
    id: "GOODSTMSUB",
    name: "Goods Store Mart",
    description: "Development environment for ESB integration workflows",
    type: "Managed Cloud",
    status: "Needs Attention",
    numberOfOpenCases: 5,
    metrics: {
      cases: 5,
      chats: 3,
      service: 1,
      change: 0,
      users: 8,
      date: "Feb 2024",
    },
  },
];
