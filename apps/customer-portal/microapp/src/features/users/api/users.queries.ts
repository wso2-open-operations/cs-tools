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

import { mutationOptions, queryOptions } from "@tanstack/react-query";
import type { CreateContactRequestDto, EditMeDto } from "@features/users/types/user.dto";
import { createContact, deleteContact, editContact, editMe, getMe, getUsers } from "@features/users/api/users.api";

export const users = {
  me: () => queryOptions({ queryKey: ["me"], queryFn: getMe }),

  editMe: () => mutationOptions({ mutationFn: (body: Partial<EditMeDto>) => editMe(body) }),

  all: (projectId: string) => queryOptions({ queryKey: ["users", projectId], queryFn: () => getUsers(projectId) }),

  create: (projectId: string) =>
    mutationOptions({ mutationFn: (body: CreateContactRequestDto) => createContact(projectId, body) }),

  edit: (projectId: string, email: string) =>
    mutationOptions({
      mutationFn: (
        body: Partial<Omit<CreateContactRequestDto, "contactEmail" | "contactFirstName" | "contactLastName">>,
      ) => editContact(projectId, email, body),
    }),

  delete: (projectId: string, email: string) => mutationOptions({ mutationFn: () => deleteContact(projectId, email) }),
};
