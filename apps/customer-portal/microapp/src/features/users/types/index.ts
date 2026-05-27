import type { USER_EDIT_MODES } from "@shared/constants";

export type UserEditModeType = (typeof USER_EDIT_MODES)[keyof typeof USER_EDIT_MODES];

export * from "./user.dto";
export * from "./user.model";
