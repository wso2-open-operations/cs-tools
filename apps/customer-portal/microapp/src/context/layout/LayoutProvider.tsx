import { useMemo, useState } from "react";
import { LayoutContext, type LayoutContextType } from "@src/context/layout";
import { matchPath, useLocation } from "react-router-dom";
import { MAIN_LAYOUT_CONFIG } from "@root/src/layout/config";

export default function LayoutProvider({ children }: { children: React.ReactNode }) {
  const location = useLocation();
  const [titleOverride, setTitleOverride] = useState<string | undefined>(undefined);

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
      appBarSlots: meta.appBarSlots ?? null,
      setTitleOverride,

      activeTabIndex: meta.tabIndex,
    }),
    [titleOverride, meta],
  );
  return <LayoutContext.Provider value={value}>{children}</LayoutContext.Provider>;
}
