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

import { useCallback, useState } from "react";

// One-wso2's plain-MUI port of this domain used a global NotificationsContext
// (a one-wso2-specific toast provider) for showSuccess/showWarning/showError.
// This app has no equivalent global context, and adding one is out of scope
// for a single domain's port — this is a small local stand-in: a single
// {severity, message} slot a page renders as a Snackbar+Alert at its own call
// site (see SplCaseDetailPage.tsx). Good enough for the one worknote-submit
// flow that needs it; not meant to become this app's real notification system.
export interface CaseNotice {
  severity: "success" | "warning" | "error";
  message: string;
}

export function useCaseNotice() {
  const [notice, setNotice] = useState<CaseNotice | null>(null);

  const showSuccess = useCallback((message: string) => setNotice({ severity: "success", message }), []);
  const showWarning = useCallback((message: string) => setNotice({ severity: "warning", message }), []);
  const showError = useCallback((message: string) => setNotice({ severity: "error", message }), []);
  const clear = useCallback(() => setNotice(null), []);

  return { notice, showSuccess, showWarning, showError, clear };
}
