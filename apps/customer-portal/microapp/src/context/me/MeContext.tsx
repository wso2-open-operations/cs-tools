import { createContext } from "react";

export type MeContextType = {
  id: string;
  roles: string[];
  isAdmin: boolean;
  timezone?: string;
};

export const MeContext = createContext<MeContextType | null>(null);
