import { useCallback, useMemo, useState } from "react";

import type { Attachment } from "@features/case-types/cases/types";
import { PreviewModal } from "@features/preview/components";

import { PreviewContext } from "./PreviewContext";

export function PreviewProvider({ children }: { children: React.ReactNode }) {
  const [attachment, setAttachment] = useState<Attachment | null>(null);
  const [open, setOpen] = useState(false);

  const openPreview = useCallback((a: Attachment) => {
    setAttachment(a);
    setOpen(true);
  }, []);

  const handleClose = useCallback(() => {
    setOpen(false);
    // Delay clearing so the dialog can animate out without a flash
    setTimeout(() => setAttachment(null), 300);
  }, []);

  const value = useMemo(() => ({ open: openPreview }), [openPreview]);

  return (
    <PreviewContext.Provider value={value}>
      {children}
      <PreviewModal open={open} attachment={attachment} onClose={handleClose} />
    </PreviewContext.Provider>
  );
}
