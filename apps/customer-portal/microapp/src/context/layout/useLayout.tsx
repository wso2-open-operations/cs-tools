import { useContext } from "react";
import { LayoutContext } from "@src/context/layout";

export function useLayout() {
  const context = useContext(LayoutContext);

  if (!context) {
    throw new Error("useLayout must be used within an LayoutProvider");
  }

  return context;
}
