import { useQuery } from "@tanstack/react-query";

import { users } from "@features/users/api/users.queries";

import { ADMIN_ROLE_ID } from "@shared/constants";

export function useMe() {
  const { data, ...query } = useQuery(users.me());

  return {
    ...query,
    data: data
      ? {
          ...data,
          name: `${data.firstName} ${data.lastName}`,
          isAdmin: data.roles.includes(ADMIN_ROLE_ID),
        }
      : undefined,
  };
}
