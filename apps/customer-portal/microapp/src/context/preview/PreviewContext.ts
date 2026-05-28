import { createContext } from "react";

import type { Attachment } from "@features/case-types/cases/types";

interface PreviewContextValue {
  open: (attachment: Attachment) => void;
}

export const PreviewContext = createContext<PreviewContextValue | null>(null);
