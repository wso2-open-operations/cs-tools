import { useQuery } from "@tanstack/react-query";

import { users } from "@features/users/api/users.queries";

import { ADMIN_ROLE_ID } from "@shared/constants";

export function useMe() {
  const me = useQuery(users.me());

  const name = me.data ? `${me.data.firstName} ${me.data.lastName}` : undefined;
  const isAdmin = me.data ? me.data.roles.includes(ADMIN_ROLE_ID) : false;

  return { ...me, data: { ...me.data, name, isAdmin } };
}
