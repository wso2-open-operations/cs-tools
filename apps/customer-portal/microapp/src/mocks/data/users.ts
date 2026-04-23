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

import type { UserListItemProps } from "@components/features/users";

export const MOCK_METRICS = [
  { label: "Total Users", value: 12 },
  { label: "Active", value: 4 },
  { label: "Admins", value: 1 },
];

export const MOCK_USERS: UserListItemProps[] = [
  {
    name: "Lithika Damnod",
    email: "user@example.com",
    role: "admin",
    lastActive: "2 hours ago",
  },
  {
    name: "Sarah Chan",
    email: "user@example.com",
    role: "developer",
    lastActive: "5 hours ago",
  },
  {
    name: "Mike Johnson",
    email: "user@example.com",
    role: "security",
    lastActive: "1 day ago",
  },
  {
    name: "Emily Rodriguez",
    email: "user@example.com",
    role: "procurement",
    lastActive: "3 days ago",
  },
  {
    name: "David Kim",
    email: "user@example.com",
    role: "manager",
    lastActive: "2 weeks ago",
  },
];

export const MOCK_ROLES = [
  {
    name: "Admin",
    description: "Full access to manage project, users, and settings",
  },
  {
    name: "Developer",
    description: "Create and manage cases, chats, and requests",
  },
  {
    name: "Security",
    description: "Security-focused access and monitoring",
  },
  {
    name: "Procurement",
    description: "Procurement and purchasing management",
  },
  {
    name: "Manager",
    description: "Team oversight and reporting access",
  },
];
