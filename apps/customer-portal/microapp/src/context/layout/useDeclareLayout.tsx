import { useContext, useLayoutEffect } from "react";

import { LayoutContext, type LayoutDeclaration } from "./LayoutContext";

export const useDeclareLayout = (config: Partial<LayoutDeclaration>) => {
  const { declareLayout } = useContext(LayoutContext);

  useLayoutEffect(() => {
    declareLayout(config);
  }, [config]);
};
