import type { CreateContactRequestDTO, EditMeDTO, Me, MeDTO, Role, User, UserDTO, UsersDTO } from "@src/types";
import { mutationOptions, queryOptions } from "@tanstack/react-query";
import apiClient from "@src/services/apiClient";

import { PROJECT_USERS_ENDPOINT, USER_ACTIONS_ENDPOINT, USERS_ME_ENDPOINT } from "@config/endpoints";

const getMe = async (): Promise<Me> => {
  const response = (await apiClient.get<MeDTO>(USERS_ME_ENDPOINT)).data;
  return toMe(response);
};

const getUsers = async (id: string): Promise<User[]> => {
  const response = (await apiClient.get<UsersDTO>(PROJECT_USERS_ENDPOINT(id))).data;
  return response.map(toUser);
};

const createContact = async (id: string, body: CreateContactRequestDTO): Promise<void> => {
  await apiClient.post(PROJECT_USERS_ENDPOINT(id), body);
};

const deleteContact = async (id: string, email: string): Promise<void> => {
  await apiClient.delete(USER_ACTIONS_ENDPOINT(id, email));
};

const editContact = async (
  id: string,
  email: string,
  body: Partial<Omit<CreateContactRequestDTO, "contactEmail" | "contactFirstName" | "contactLastName">>,
): Promise<void> => {
  await apiClient.patch(USER_ACTIONS_ENDPOINT(id, email), body);
};

const editMe = async (body: Partial<EditMeDTO>): Promise<void> => {
  await apiClient.patch(USERS_ME_ENDPOINT, body);
};

/* Mappers */
function toMe(dto: MeDTO): Me {
  return {
    id: dto.id,
    email: dto.email,
    firstName: dto.firstName,
    lastName: dto.lastName,
    timezone: dto.timeZone,
  };
}

function toUser(dto: UserDTO): User {
  const roles: Role[] = [];
  if (dto.isCsAdmin) roles.push("Admin");
  if (dto.isSecurityContact) roles.push("System User");
  else roles.push("Portal User");

  return {
    id: dto.id,
    email: dto.email,
    firstName: dto.firstName,
    lastName: dto.lastName,
    status: dto.membershipStatus === "REGISTERED" ? "registered" : "invited",
    roles: roles,
    lastActive: new Date(), // TODO: Replace this placeholder with actual last active date from backend when available
  };
}

/* Query Options */
export const users = {
  me: () => queryOptions({ queryKey: ["me"], queryFn: getMe }),

  editMe: () =>
    mutationOptions({
      mutationFn: (body: Partial<EditMeDTO>) => editMe(body),
    }),

  all: (projectId: string) => queryOptions({ queryKey: ["users", projectId], queryFn: () => getUsers(projectId) }),

  create: (projectId: string) =>
    mutationOptions({
      mutationFn: (body: CreateContactRequestDTO) => createContact(projectId, body),
    }),

  edit: (projectId: string, email: string) =>
    mutationOptions({
      mutationFn: (
        body: Partial<Omit<CreateContactRequestDTO, "contactEmail" | "contactFirstName" | "contactLastName">>,
      ) => editContact(projectId, email, body),
    }),

  delete: (projectId: string, email: string) =>
    mutationOptions({
      mutationFn: () => deleteContact(projectId, email),
    }),
};
