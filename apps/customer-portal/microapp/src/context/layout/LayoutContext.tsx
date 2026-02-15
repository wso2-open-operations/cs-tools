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
  setOverlineSlotOverride: (slot: ReactNode | string | undefined) => void;
  setSubtitleSlotOverride: (slot: ReactNode | string | undefined) => void;
  setStartSlotOverride: (slot: ReactNode | undefined) => void;
  setEndSlotOverride: (slot: ReactNode | undefined) => void;
  setAppBarSlotsOverride: (slot: ReactNode | undefined) => void;

  /* TabBar Properties */
  activeTabIndex: number;
};

export const LayoutContext = createContext<LayoutContextType | null>(null);
