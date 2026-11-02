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

import {
  createContext,
  useContext,
  useEffect,
  useMemo,
  useRef,
  type JSX,
  type ReactNode,
} from "react";
import type { EncodedAttachment } from "@components/attachments/encodeAttachment";

/** The user-entered values of the New service request form. */
export interface NewServiceRequestDraft {
  /**
   * Identifies how the form was entered (locked project / related case). A
   * draft is only restored into a form opened with the same key, so a form
   * opened for a different project never inherits a stale draft.
   */
  entryKey: string;
  projectId: string;
  deploymentId: string;
  deployedProductId: string;
  catalogId: string;
  catalogItemId: string;
  /** The catalog item's variable answers, keyed by variable id. They belong to `catalogItemId`. */
  answers: Record<string, string>;
  attachments: EncodedAttachment[];
}

export interface NewServiceRequestDraftStore {
  get: () => NewServiceRequestDraft | undefined;
  set: (draft: NewServiceRequestDraft) => void;
  clear: () => void;
}

/** Used outside a provider: nothing is retained. */
const NO_RETENTION: NewServiceRequestDraftStore = {
  get: () => undefined,
  set: () => undefined,
  clear: () => undefined,
};

const NewServiceRequestDraftContext = createContext<NewServiceRequestDraftStore>(NO_RETENTION);

/**
 * Keeps the New service request form's draft in memory for as long as this
 * provider stays mounted, so leaving `/operations/service-requests/new` for
 * another route (e.g. an open case tab) and coming back does not lose what was
 * entered. Deliberately in-memory only: a reload starts with an empty form, and
 * the draft is dropped when a sign-out begins (the `app:signing-out` event every
 * manual and idle sign-out dispatches) so it never outlives the user it belongs
 * to. Held in a ref, not state, so saving a keystroke never re-renders the tree
 * above the routes.
 */
export function NewServiceRequestDraftProvider({
  children,
}: {
  children: ReactNode;
}): JSX.Element {
  const draftRef = useRef<NewServiceRequestDraft | undefined>(undefined);
  const store = useMemo<NewServiceRequestDraftStore>(
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
  useEffect(() => {
    const discard = (): void => {
      draftRef.current = undefined;
    };
    window.addEventListener("app:signing-out", discard);
    return () => window.removeEventListener("app:signing-out", discard);
  }, []);
  return (
    <NewServiceRequestDraftContext.Provider value={store}>
      {children}
    </NewServiceRequestDraftContext.Provider>
  );
}

/** The New service request draft store (a no-op store outside a {@link NewServiceRequestDraftProvider}). */
export function useNewServiceRequestDraft(): NewServiceRequestDraftStore {
  return useContext(NewServiceRequestDraftContext);
}
