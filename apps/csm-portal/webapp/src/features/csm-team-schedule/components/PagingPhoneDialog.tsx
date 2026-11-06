/**
 * Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import { useEffect, useId, useRef, useState, type JSX, type KeyboardEvent } from "react";
import RequiredMark from "@components/RequiredMark";
import type { PagingChainMember } from "../types";
import { validateE164 } from "../utils/pagingPhone";

export const PAGING_PHONE_HELPER =
  "Used only for Case Alerts calls when the person has no number on their profile. Never changes their profile.";

interface Props {
  mode: "edit" | "remove";
  member: PagingChainMember;
  busy: boolean;
  /** Why the server refused the last save or removal. */
  error?: string;
  onSave: (phone: string) => void;
  onRemove: () => void;
  onClose: () => void;
}

const FOCUSABLE = 'button:not([disabled]), input:not([disabled]), [href], [tabindex]:not([tabindex="-1"])';

/**
 * Add, change or remove a person's paging-only number. A modal: focus moves
 * in on open, Tab stays inside, Esc closes, and focus goes back to the
 * control that opened it.
 */
export default function PagingPhoneDialog({ mode, member, busy, error, onSave, onRemove, onClose }: Props): JSX.Element {
  const ids = useId();
  const titleId = `${ids}-title`;
  const helperId = `${ids}-helper`;
  const errorId = `${ids}-error`;
  const inputId = `${ids}-phone`;
  const boxRef = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLInputElement>(null);
  const confirmRef = useRef<HTMLButtonElement>(null);
  const [value, setValue] = useState(member.pagingPhone?.phone ?? "");
  const [touched, setTouched] = useState(false);
  const name = member.name || member.email;
  const check = validateE164(value);
  const fieldError = touched ? check.error : undefined;

  // Focus in on open, and back to the opener on close.
  useEffect(() => {
    const opener = document.activeElement as HTMLElement | null;
    (inputRef.current ?? confirmRef.current)?.focus();
    return () => opener?.focus?.();
  }, []);

  const onKeyDown = (e: KeyboardEvent<HTMLDivElement>) => {
    if (e.key === "Escape") {
      e.stopPropagation();
      onClose();
      return;
    }
    if (e.key !== "Tab" || !boxRef.current) return;
    const items = [...boxRef.current.querySelectorAll<HTMLElement>(FOCUSABLE)];
    if (items.length === 0) return;
    const first = items[0];
    const last = items[items.length - 1];
    if (e.shiftKey && document.activeElement === first) {
      e.preventDefault();
      last.focus();
    } else if (!e.shiftKey && document.activeElement === last) {
      e.preventDefault();
      first.focus();
    }
  };

  const submit = () => {
    setTouched(true);
    if (check.value && !busy) onSave(check.value);
  };

  return (
    <div className="cp-modal-back" onMouseDown={(e) => e.target === e.currentTarget && onClose()}>
      <div
        ref={boxRef}
        className="cp-modal"
        role="dialog"
        aria-modal="true"
        aria-labelledby={titleId}
        aria-describedby={mode === "edit" ? helperId : undefined}
        onKeyDown={onKeyDown}
      >
        {mode === "edit" ? (
          <form
            onSubmit={(e) => {
              e.preventDefault();
              submit();
            }}
            noValidate
          >
            <h3 id={titleId}>
              {member.pagingPhone ? "Change" : "Add"} alert number for {name}
            </h3>
            <label htmlFor={inputId} className="cp-lbl">
              Mobile number (with country code)
              <RequiredMark />
            </label>
            <input
              ref={inputRef}
              id={inputId}
              className="cp-input"
              type="tel"
              required
              inputMode="tel"
              autoComplete="off"
              placeholder="+94771234567"
              value={value}
              aria-invalid={fieldError ? true : undefined}
              aria-describedby={`${helperId}${fieldError ? ` ${errorId}` : ""}`}
              onChange={(e) => setValue(e.target.value)}
              onBlur={() => setTouched(true)}
            />
            <p id={helperId} className="cp-help">
              {PAGING_PHONE_HELPER}
            </p>
            {fieldError ? (
              <p id={errorId} className="cp-err" role="alert">
                {fieldError}
              </p>
            ) : null}
            {error ? (
              <p className="cp-err" role="alert">
                {error}
              </p>
            ) : null}
            <div className="cp-modal-actions">
              <button type="button" className="btn sm" onClick={onClose}>
                Cancel
              </button>
              <button type="submit" className="btn sm primary" aria-disabled={busy || undefined}>
                {busy ? "Saving…" : "Save"}
              </button>
            </div>
          </form>
        ) : (
          <>
            <h3 id={titleId}>Remove alert number for {name}?</h3>
            <p className="cp-help">
              Case Alerts will no longer call {name} on {member.pagingPhone?.masked ?? "this number"}. Their profile
              is not changed.
            </p>
            {error ? (
              <p className="cp-err" role="alert">
                {error}
              </p>
            ) : null}
            <div className="cp-modal-actions">
              <button type="button" className="btn sm" onClick={onClose}>
                Cancel
              </button>
              <button
                ref={confirmRef}
                type="button"
                className="btn sm primary"
                aria-disabled={busy || undefined}
                onClick={() => {
                  if (!busy) onRemove();
                }}
              >
                {busy ? "Removing…" : "Remove"}
              </button>
            </div>
          </>
        )}
      </div>
    </div>
  );
}
