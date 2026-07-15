// Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License. You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied. See the License for the
// specific language governing permissions and limitations
// under the License.

import { useCallback, useRef, useState } from "react";
import { detectPii } from "@features/support/utils/piiDetection";
import type { PiiWarningDialogProps } from "@features/support/components/dialogs/PiiWarningDialog";

export interface UsePiiGuardResult {
  /**
   * Scans `text` for PII before running `onProceed`.
   *
   * - No PII found: `onProceed` runs immediately.
   * - PII found: the warning dialog opens and `onProceed` is deferred until the
   *   user chooses "Post anyway".
   */
  checkBeforeSubmit: (text: string, onProceed: () => void) => void;
  /** Spread onto {@link PiiWarningDialog}. */
  dialogProps: PiiWarningDialogProps;
}

/**
 * Reusable PII interception for submit handlers (chat, case creation, comments).
 * Keeps the warning dialog state and the deferred submission action, so a call
 * site only needs to wrap its send logic and render the dialog.
 */
export const usePiiGuard = (): UsePiiGuardResult => {
  const [open, setOpen] = useState(false);
  const [detectedLabels, setDetectedLabels] = useState<string[]>([]);
  // The submission to run if the user proceeds despite the warning.
  const pendingActionRef = useRef<(() => void) | null>(null);

  const checkBeforeSubmit = useCallback(
    (text: string, onProceed: () => void) => {
      const matches = detectPii(text);
      if (matches.length === 0) {
        onProceed();
        return;
      }
      // Distinct labels, preserving detection order.
      setDetectedLabels([...new Set(matches.map((match) => match.label))]);
      pendingActionRef.current = onProceed;
      setOpen(true);
    },
    []
  );

  const onEdit = useCallback(() => {
    pendingActionRef.current = null;
    setOpen(false);
  }, []);

  const onProceed = useCallback(() => {
    const action = pendingActionRef.current;
    pendingActionRef.current = null;
    setOpen(false);
    action?.();
  }, []);

  return {
    checkBeforeSubmit,
    dialogProps: { open, detectedLabels, onEdit, onProceed },
  };
};
