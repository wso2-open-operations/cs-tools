import { createContext, type ReactNode } from "react";

export type AppBarVariant = "minimal" | "default" | "extended";

export type LayoutContextType = {
  /* AppBar Properties */
  title: string;
  showAppBar: boolean;
  hasBackAction: boolean;
  appBarVariant: AppBarVariant;
  appBarSlots?: ReactNode;
  setTitleOverride: (title: string) => void;

  /* TabBar Properties */
  activeTabIndex: number;
};

export const LayoutContext = createContext<LayoutContextType | null>(null);
