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
  SearchUsersRequest,
  SearchUsersResponse,
  User,
} from "@features/csm-users/types/csmUsers";

// Internal CRE engineers + sales-engineering names that the CSM portal mocks
// already reference (kept consistent with contactsMocks.ts). A small customer
// sample is included so user-type filtering has visible variety.
const USERS: User[] = [
  {
    id: "usr-001",
    userName: "naveen.p",
    firstName: "Naveen",
    lastName: "Perera",
    email: "naveen@wso2.com",
    phone: null,
    timezone: "Asia/Colombo",
    userType: "internal",
    createdAt: "2023-04-12T08:00:00Z",
    updatedAt: "2026-05-01T08:00:00Z",
  },
  {
    id: "usr-002",
    userName: "ramesh.m",
    firstName: "Ramesh",
    lastName: "Madawalage",
    email: "ramesh@wso2.com",
    phone: null,
    timezone: "Asia/Colombo",
    userType: "internal",
    createdAt: "2023-05-10T08:00:00Z",
    updatedAt: "2026-05-12T08:00:00Z",
  },
  {
    id: "usr-003",
    userName: "chathura.d",
    firstName: "Chathura",
    lastName: "Dilan",
    email: "chathura@wso2.com",
    phone: null,
    timezone: "Asia/Colombo",
    userType: "internal",
    createdAt: "2023-06-15T08:00:00Z",
    updatedAt: "2026-04-20T08:00:00Z",
  },
  {
    id: "usr-004",
    userName: "ishara.k",
    firstName: "Ishara",
    lastName: "Karunaratne",
    email: "ishara@wso2.com",
    phone: null,
    timezone: "Asia/Colombo",
    userType: "internal",
    createdAt: "2023-07-20T08:00:00Z",
    updatedAt: "2026-03-30T08:00:00Z",
  },
  {
    id: "usr-005",
    userName: "dilshan.a",
    firstName: "Dilshan",
    lastName: "Amarasinghe",
    email: "dilshan@wso2.com",
    phone: null,
    timezone: "Asia/Colombo",
    userType: "internal",
    createdAt: "2023-08-05T08:00:00Z",
    updatedAt: "2026-05-22T08:00:00Z",
  },
  {
    id: "usr-006",
    userName: "asanka.r",
    firstName: "Asanka",
    lastName: "Ratnayake",
    email: "asanka@wso2.com",
    phone: null,
    timezone: "Asia/Colombo",
    userType: "internal",
    createdAt: "2023-09-10T08:00:00Z",
    updatedAt: "2026-05-18T08:00:00Z",
  },
  {
    id: "usr-007",
    userName: "sajith.e",
    firstName: "Sajith",
    lastName: "Ekanayaka",
    email: "sajithe@wso2.com",
    phone: null,
    timezone: "Asia/Colombo",
    userType: "internal",
    createdAt: "2023-02-01T08:00:00Z",
    updatedAt: "2026-06-01T08:00:00Z",
  },
  {
    id: "usr-101",
    userName: "renee.park",
    firstName: "Renee",
    lastName: "Park",
    email: "renee.park@acmebank.com",
    phone: null,
    timezone: "America/New_York",
    userType: "customer",
    createdAt: "2023-11-01T08:00:00Z",
    updatedAt: "2026-05-12T08:00:00Z",
  },
  {
    id: "usr-102",
    userName: "helena.voss",
    firstName: "Helena",
    lastName: "Voss",
    email: "helena.voss@initech.com",
    phone: null,
    timezone: "Europe/Berlin",
    userType: "customer",
    createdAt: "2024-01-08T08:00:00Z",
    updatedAt: "2026-04-09T08:00:00Z",
  },
  {
    id: "usr-103",
    userName: "peter.gibbons",
    firstName: "Peter",
    lastName: "Gibbons",
    email: "peter.gibbons@initech.com",
    phone: null,
    timezone: "America/Chicago",
    userType: "customer",
    createdAt: "2024-02-14T08:00:00Z",
    updatedAt: "2026-05-19T08:00:00Z",
  },
];

function matchesQuery(u: User, q: string): boolean {
  const needle = q.toLowerCase();
  return (
    u.userName.toLowerCase().includes(needle) ||
    u.email.toLowerCase().includes(needle) ||
    `${u.firstName} ${u.lastName}`.toLowerCase().includes(needle)
  );
}

export function getMockBackendUsersResponse(
  req: SearchUsersRequest,
): SearchUsersResponse {
  const q = (req.searchQuery ?? "").trim();
  const filtered = q ? USERS.filter((u) => matchesQuery(u, q)) : USERS;
  const offset = req.pagination?.offset ?? 0;
  const limit = req.pagination?.limit ?? 20;
  const page = filtered.slice(offset, offset + limit);
  return {
    users: page,
    total: filtered.length,
    limit,
    offset,
    hasMore: offset + page.length < filtered.length,
  };
}
