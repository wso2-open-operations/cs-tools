import { useNavigate } from "react-router-dom";

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
  };
};
