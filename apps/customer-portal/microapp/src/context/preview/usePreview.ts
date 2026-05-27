import { useContext } from "react";

import { PreviewContext } from "./PreviewContext";

export function usePreview() {
  const ctx = useContext(PreviewContext);
  if (!ctx) {
    throw new Error("usePreview must be used withinPreviewProvider");
  }
  return ctx;
}
