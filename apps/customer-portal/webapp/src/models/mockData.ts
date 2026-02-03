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

/**
 * Sample projects for the project switcher.
 */
export const mockProjects: ProjectListItem[] = [
  {
    id: "1",
    key: "project-1",
    name: "Project 1",
    description: "Description for Project 1",
    createdOn: "2026-01-29T11:28:40+05:30",
  },
  {
    id: "2",
    key: "project-2",
    name: "Project 2",
    description: "Description for Project 2",
    createdOn: "2026-01-29T11:28:40+05:30",
  },
  {
    id: "3",
    key: "project-3",
    name: "Project 3",
    description: "Description for Project 3",
    createdOn: "2026-01-29T11:28:40+05:30",
  },
  {
    id: "4",
    key: "project-4",
    name: "Project 4",
    description: "Description for Project 4",
    createdOn: "2026-01-29T11:28:40+05:30",
  },
];

/**
 * Sample user for the user menu.
 */
export const mockUser: UserProfile = {
  name: "John Doe",
  email: "John@example.com",
  avatar: "JD",
  role: "Admin",
};
