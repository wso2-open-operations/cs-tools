import { useMemo, useState, type ReactNode } from "react";
import { LayoutContext, type LayoutContextType } from "@src/context/layout";
import { matchPath, useLocation } from "react-router-dom";
import { MAIN_LAYOUT_CONFIG } from "@root/src/components/layout/config";

export default function LayoutProvider({ children }: { children: React.ReactNode }) {
  const location = useLocation();
  const [titleOverride, setTitleOverride] = useState<string | undefined>(undefined);
  const [overlineSlotOverride, setOverlineSlotOverride] = useState<ReactNode | string | undefined>(undefined);
  const [subtitleSlotOverride, setSubtitleSlotOverride] = useState<ReactNode | string | undefined>(undefined);
  const [startSlotOverride, setStartSlotOverride] = useState<ReactNode | undefined>(undefined);
  const [endSlotOverride, setEndSlotOverride] = useState<ReactNode | undefined>(undefined);
  const [appBarSlotsOverride, setAppBarSlotsOverride] = useState<ReactNode | undefined>(undefined);

  const meta = useMemo(() => {
    const config = MAIN_LAYOUT_CONFIG.find((route) => matchPath({ path: route.path, end: true }, location.pathname));
    if (config === undefined) throw Error("Route Configuration Not Found");
    return config;
  }, [location.pathname]);

  const value: LayoutContextType = useMemo(
    () => ({
      title: titleOverride ?? meta.title,
      showAppBar: meta.showAppBar ?? true,
      hasBackAction: meta.hasBackAction ?? false,
      appBarVariant: meta.appBarVariant ?? "default",
      overlineSlot: overlineSlotOverride ?? meta.overlineSlot ?? null,
      subtitleSlot: subtitleSlotOverride ?? meta.subtitleSlot ?? null,
      startSlot: startSlotOverride ?? meta.startSlot ?? null,
      endSlot: endSlotOverride ?? meta.endSlot ?? null,
      appBarSlots: appBarSlotsOverride ?? meta.appBarSlots ?? null,
      setTitleOverride,
      setOverlineSlotOverride,
      setSubtitleSlotOverride,
      setStartSlotOverride,
      setEndSlotOverride,
      setAppBarSlotsOverride,

      activeTabIndex: meta.tabIndex,
    }),

    [
      meta,
      titleOverride,
      overlineSlotOverride,
      subtitleSlotOverride,
      startSlotOverride,
      endSlotOverride,
      appBarSlotsOverride,
    ],
  );
  return <LayoutContext.Provider value={value}>{children}</LayoutContext.Provider>;
}
