import type { ReactNode } from "react";
import type { AppBarVariant } from "@src/context/layout";
import { AllCasesAppBarSlot } from "@pages/AllCasesPage";
import { DetailedPageAppBarSlot } from "@pages/DetailedPage";

type AppBarConfig = {
  showNotifications: boolean;
  showProjectSelector: boolean;
  showChips: boolean;
};

interface MainLayoutConfigType {
  path: string;
  title: string;
  tabIndex: number;
  showAppBar?: boolean;
  hasBackAction?: boolean;
  appBarVariant?: AppBarVariant;
  appBarSlots?: ReactNode;
}

export const APP_BAR_CONFIG: Record<AppBarVariant, AppBarConfig> = {
  default: {
    showNotifications: true,
    showProjectSelector: true,
    showChips: false,
  },
  minimal: {
    showNotifications: false,
    showProjectSelector: false,
    showChips: false,
  },
  extended: {
    showNotifications: true,
    showProjectSelector: true,
    showChips: true,
  },
};

export const MAIN_LAYOUT_CONFIG: MainLayoutConfigType[] = [
  { path: "/", title: "Dashboard", appBarVariant: "extended", tabIndex: 0 },
  { path: "/support", title: "Support Center", tabIndex: 1 },
  { path: "/users", title: "Project Users", tabIndex: 2 },
  { path: "/profile", title: "My Profile", appBarVariant: "minimal", tabIndex: 3 },
  {
    path: "/notifications",
    title: "Notifications",
    appBarVariant: "minimal",
    hasBackAction: true,
    tabIndex: -1,
  },
  { path: "/chat", title: "Chat with Novera", appBarVariant: "minimal", hasBackAction: true, tabIndex: -1 },
  { path: "/create-case", title: "Create Support Case", appBarVariant: "minimal", hasBackAction: true, tabIndex: -1 },
  {
    path: "/cases/all",
    title: "All Cases",
    appBarVariant: "minimal",
    hasBackAction: true,
    tabIndex: -1,
    appBarSlots: <AllCasesAppBarSlot />,
  },
  {
    path: "/cases/:id",
    title: "Authentication Service Issue",
    appBarVariant: "minimal",
    hasBackAction: true,
    tabIndex: -1,
    appBarSlots: <DetailedPageAppBarSlot />,
  },
];
