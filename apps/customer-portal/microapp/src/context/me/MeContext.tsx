import { createContext } from "react";

export type MeContextType = {
  id: string;
  roles: string[];
  isAdmin: boolean;
  /** WSO2 staff, as opposed to a customer or partner. */
  isInternalUser: boolean;
  timezone?: string;
};

export const MeContext = createContext<MeContextType | null>(null);
