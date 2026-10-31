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

/* eslint-disable react-refresh/only-export-components -- Provider component and its useXxx hook are colocated per the repo's context idiom (fast-refresh DX only) */

import { createContext, useContext, useMemo, useRef, type JSX, type ReactNode } from "react";
import type { Severity } from "@features/csm-dashboard/types/abtDashboard";
import type { BeCaseIssueType } from "@api/backend/types";
import type { EncodedAttachment } from "@components/attachments/encodeAttachment";
import type { WatchListEdits } from "@features/csm-cases/utils/createCaseWatchList";

/** The user-entered values of the New case form. */
export interface NewCaseDraft {
  /**
   * Identifies how the form was entered (locked project / related case). A
   * draft is only restored into a form opened with the same key, so a form
   * opened for a different project never inherits a stale draft.
   */
  entryKey: string;
  projectId: string;
  deploymentId: string;
  deployedProductId: string;
  severity: Severity | "";
  issueType: BeCaseIssueType | "";
  subject: string;
  description: string;
  attachments: EncodedAttachment[];
  watchEdits: WatchListEdits;
}

export interface NewCaseDraftStore {
  get: () => NewCaseDraft | undefined;
  set: (draft: NewCaseDraft) => void;
  clear: () => void;
}

/** Used outside a provider: nothing is retained. */
const NO_RETENTION: NewCaseDraftStore = {
  get: () => undefined,
  set: () => undefined,
  clear: () => undefined,
};

const NewCaseDraftContext = createContext<NewCaseDraftStore>(NO_RETENTION);

/**
 * Keeps the New case form's draft in memory for as long as this provider stays
 * mounted, so leaving `/cases/new` for another route (e.g. an open case tab)
 * and coming back does not lose what was typed. Deliberately in-memory only:
 * a reload or sign-out starts with an empty form. Held in a ref, not state, so
 * saving a keystroke never re-renders the tree above the routes.
 */
export function NewCaseDraftProvider({ children }: { children: ReactNode }): JSX.Element {
  const draftRef = useRef<NewCaseDraft | undefined>(undefined);
  const store = useMemo<NewCaseDraftStore>(
    () => ({
      get: () => draftRef.current,
      set: (draft) => {
        draftRef.current = draft;
      },
      clear: () => {
        draftRef.current = undefined;
      },
    }),
    [],
  );
  return <NewCaseDraftContext.Provider value={store}>{children}</NewCaseDraftContext.Provider>;
}

/** The New case draft store (a no-op store outside a {@link NewCaseDraftProvider}). */
export function useNewCaseDraft(): NewCaseDraftStore {
  return useContext(NewCaseDraftContext);
}
