import type { Role } from "@features/users/types";

import { ROLES } from "@shared/constants";

export const USER_EDIT_MODES = {
  INVITE: "invite",
  EDIT: "edit",
};

export const DEFAULT_USER_ROLE: Role = ROLES.PORTAL_USER;
