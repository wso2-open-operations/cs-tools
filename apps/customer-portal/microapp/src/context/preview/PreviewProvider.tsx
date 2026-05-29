// Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.
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
