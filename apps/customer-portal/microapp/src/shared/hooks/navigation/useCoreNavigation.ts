import { useNavigate } from "react-router-dom";

import type { User } from "@features/users/types";

import type { CaseType } from "@shared/types";

export const useCoreNavigation = () => {
  const navigate = useNavigate();

  return {
    back: () => navigate(-1),

    toHome: (options: { replace: boolean } = { replace: false }) => navigate("/", options),
    toSupport: () => navigate("/support"),
    toUsers: () => navigate("/users"),
    toProfile: () => navigate("/profile"),

    toAll: (type: CaseType) =>
      navigate({
        pathname: "/support/all",
        search: new URLSearchParams({ type }).toString(),
      }),

    toInviteUser: () => navigate("/users/invite"),
    toEditUser: (user: User) => navigate("/users/edit", { state: { user } }),
  };
};
