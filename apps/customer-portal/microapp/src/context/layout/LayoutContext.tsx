import { createContext, type ReactNode } from "react";

export type AppBarVariant = "minimal" | "default" | "extended" | "notifications";

export type LayoutContextType = {
  /* AppBar Properties */
  title: string;
  showAppBar: boolean;
  hasBackAction: boolean;
  appBarVariant: AppBarVariant;
  overlineSlot?: ReactNode | string;
  subtitleSlot?: ReactNode | string;
  startSlot?: ReactNode;
  endSlot?: ReactNode;
  appBarSlots?: ReactNode;
  setTitleOverride: (title: string | undefined) => void;
  setOverlineSlotOverride: (slot: ReactNode | string) => void;
  setSubtitleSlotOverride: (slot: ReactNode | string) => void;
  setStartSlotOverride: (slot: ReactNode) => void;
  setEndSlotOverride: (slot: ReactNode) => void;
  setAppBarSlotsOverride: (slot: ReactNode) => void;

  /* TabBar Properties */
  activeTabIndex: number;
};

export const LayoutContext = createContext<LayoutContextType | null>(null);
