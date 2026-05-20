import { useLocation } from "react-router-dom";

import type { User } from "@features/users/types";

import { USER_EDIT_MODES } from "@shared/constants";

export function useMode() {
  const { pathname, state } = useLocation();
  const mode = pathname.includes(USER_EDIT_MODES.INVITE) ? USER_EDIT_MODES.INVITE : USER_EDIT_MODES.EDIT;

  return { mode, initial: mode === USER_EDIT_MODES.EDIT ? ((state?.user as User) ?? null) : null };
}
