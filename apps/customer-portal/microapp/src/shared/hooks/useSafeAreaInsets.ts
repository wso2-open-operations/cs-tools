import { useLayoutEffect } from "react";

import { requestDeviceSafeAreaInsets } from "@src/bridge";

export function useSafeAreaInsets() {
  useLayoutEffect(() => {
    requestDeviceSafeAreaInsets((data) => {
      if (data?.insets) {
        const { top, right, bottom, left } = data.insets;
        const root = document.documentElement;
        root.style.setProperty("--safe-top", `${top}px`);
        root.style.setProperty("--safe-right", `${right}px`);
        root.style.setProperty("--safe-bottom", `${bottom}px`);
        root.style.setProperty("--safe-left", `${left}px`);
      }
    });
  }, []);
}
