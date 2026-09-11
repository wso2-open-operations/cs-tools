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

import { Box } from "@wso2/oxygen-ui";
import { useRef, type JSX } from "react";
import FloatingSlidePanel from "@components/FloatingSlidePanel";
import { useCloseOnOutsideClick } from "@hooks/useCloseOnOutsideClick";
import KBArticlePreviewContent from "@features/csm-kb-articles/components/KBArticlePreviewContent";
import { KB_QUICK_PREVIEW_EYE_SELECTOR } from "@features/csm-kb-articles/utils/kbQuickPreviewEye";
import type { KBArticle } from "@features/csm-kb-articles/types/csmKbArticles";

interface KBArticlePreviewDrawerProps {
  /** The article being previewed. `null` keeps the drawer mounted-but-closed,
   * so its close transition can play instead of unmounting mid-animation. */
  article: KBArticle | null;
  knowledgeBaseName: string;
  authorName: string;
  updatedByName: string;
  onClose: () => void;
}

/**
 * KB article quick-preview drawer, mirroring csm-cases' CasePreviewDrawer:
 * `FloatingSlidePanel` (not `Drawer`) so the rest of the page stays fully
 * interactive while open, plus `useCloseOnOutsideClick` for click-outside
 * dismissal that doesn't race the eye button's own toggle logic.
 */
export default function KBArticlePreviewDrawer({
  article,
  knowledgeBaseName,
  authorName,
  updatedByName,
  onClose,
}: KBArticlePreviewDrawerProps): JSX.Element {
  const contentRef = useRef<HTMLDivElement | null>(null);
  useCloseOnOutsideClick(!!article, contentRef, KB_QUICK_PREVIEW_EYE_SELECTOR, onClose);

  return (
    <FloatingSlidePanel open={!!article} ariaLabel="KB article preview">
      {article && (
        <Box ref={contentRef} sx={{ height: "100%" }}>
          <KBArticlePreviewContent
            article={article}
            knowledgeBaseName={knowledgeBaseName}
            authorName={authorName}
            updatedByName={updatedByName}
            onClose={onClose}
          />
        </Box>
      )}
    </FloatingSlidePanel>
  );
}
