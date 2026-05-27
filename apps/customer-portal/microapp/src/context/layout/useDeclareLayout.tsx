import { type DependencyList, useContext, useLayoutEffect } from "react";

import { LayoutContext, type LayoutDeclaration } from "./LayoutContext";

export const useDeclareLayout = (
  config: Partial<LayoutDeclaration>,
  options: { enabled?: boolean } = { enabled: true },
  deps?: DependencyList,
) => {
  const { declareLayout } = useContext(LayoutContext);

  useLayoutEffect(() => {
    if (!options.enabled) return;
    declareLayout(config);
  }, deps ?? []);
};
