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

import { useState } from "react";
import { Alert } from "@mui/material";

/**
 * Dismissible banner for a refetch that failed while `keepPreviousData` is
 * still showing an earlier filter's data — the page's cold-start
 * `isError && !data` guard doesn't fire in that case, so without this the
 * stale numbers would sit on screen with no sign anything went wrong.
 *
 * Callers must remount this on every new failure — e.g.
 * `<StaleDataAlert key={errorUpdatedAt} .../>` — so a second failure after a
 * dismissal isn't silently swallowed by the first dismissal's state.
 */
export function StaleDataAlert({ message }: { message: string }) {
  const [dismissed, setDismissed] = useState(false);

  if (dismissed) return null;
  return (
    <Alert severity="error" onClose={() => setDismissed(true)} sx={{ mb: 2 }}>
      {message}
    </Alert>
  );
}
