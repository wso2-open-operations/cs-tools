import { useQuery } from "@tanstack/react-query";

import { useProject } from "@context/project";

import { users } from "@features/users/api/users.queries";

export function useUserStats() {
  const { projectId } = useProject();
  const { data } = useQuery(users.all(projectId!));

  return {
    total: data?.length,
    registered: data?.filter((u) => u.status === "registered").length,
    invited: data?.filter((u) => u.status === "invited").length,
    admins: data?.filter((u) => u.roles.includes("Admin")).length,
  };
}
