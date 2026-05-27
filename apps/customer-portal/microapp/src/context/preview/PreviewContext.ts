import { createContext } from "react";

import type { Attachment } from "@features/cases/types";

interface PreviewContextValue {
  open: (attachment: Attachment) => void;
}

export const PreviewContext = createContext<PreviewContextValue | null>(null);
