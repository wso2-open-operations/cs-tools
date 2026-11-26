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
import { useState, type JSX, type KeyboardEvent } from "react";
import type { CcPerson } from "../api/useFamilyCcPeople";
import type { RotaFamily } from "../types";

/** The most suggestions shown at once. */
const MAX_OPTIONS = 8;

/** The domains an address outside the family may be typed in from. */
const CC_ADDRESS_DOMAINS =["wso2.com", "choreolabs.com"];

// No comma either: the list is comma- or newline-separated (listOf), so a typed
// "first,last@wso2.com" would be stored as two chips and fail to save.
const EMAIL = /^[^\s@,]+@[^\s@,]+\.[^\s@,]+$/;

/** A whole address in one of CC_ADDRESS_DOMAINS, lower-cased, or null. */
function typedAddress(text: string): string | null {
  const a = text.trim().toLowerCase();
  if (!EMAIL.test(a)) return null;
  return CC_ADDRESS_DOMAINS.includes(a.slice(a.lastIndexOf("@") + 1)) ? a : null;
}

/** The addresses in a newline-separated list, as the form keeps them. */
function listOf(value: string): string[] {
  return value
    .split(/[,\n\r]+/)
    .map((a) => a.trim())
    .filter(Boolean);
}

interface Props {
  /** The input's id: the field's label points at it. */
  id: string;
  /** The chosen addresses, newline-separated (the form's own shape). */
  value: string;
  onChange: (value: string) => void;
  /** Who may be picked. */
  people: CcPerson[];
  /** The family the list is limited to, for the "not a member" mark. */
  family: RotaFamily;
  describedBy?: string;
}

/**
 * Picks the people copied on a rota reminder: a chip per person, and a search
 * box that suggests the family's members by name or email. Somebody outside
 * the family can be added by typing their whole address, but only in one of
 * CC_ADDRESS_DOMAINS: it is offered as "Add <address>" and picked like a
 * member. An address that is not a member (typed in, saved before the picker
 * existed, or of somebody who has since left) keeps its chip, marked, so it
 * can be removed but is never lost without anybody seeing it.
 */
export default function RotaCcPicker({ id, value, onChange, people, family, describedBy }: Props): JSX.Element {
  const [query, setQuery] = useState("");
  const [open, setOpen] = useState(false);
  const [active, setActive] = useState(0);
  const chosen = listOf(value);
  const chosenSet = new Set(chosen.map((a) => a.toLowerCase()));
  const byEmail = new Map(people.map((p) => [p.email.toLowerCase(), p]));

  const q = query.trim().toLowerCase();
  const members = people
    .filter((p) => !chosenSet.has(p.email.toLowerCase()))
    .filter((p) => !q || p.name.toLowerCase().includes(q) || p.email.toLowerCase().includes(q))
    .slice(0, MAX_OPTIONS);
  // A whole address in an allowed domain that is not a member, nor already
  // chosen, is offered after the members as "Add <address>".
  const typed = typedAddress(query);
  const addable = typed && !chosenSet.has(typed) && !byEmail.has(typed) ? typed : null;
  const options: CcPerson[] = addable ? [...members, { name: addable, email: addable }] : members;
  const listId = `${id}-list`;
  const showList = open && options.length > 0;

  const pick = (p: CcPerson): void => {
    onChange([...chosen, p.email].join("\n"));
    setQuery("");
    setActive(0);
    // Closed after a pick, so the list never covers what follows (the Save
    // button). Typing or the Down arrow opens it again.
    setOpen(false);
  };
  const remove = (email: string): void => {
    onChange(chosen.filter((a) => a.toLowerCase() !== email.toLowerCase()).join("\n"));
  };
  const onKeyDown = (e: KeyboardEvent<HTMLInputElement>): void => {
    if (e.key === "ArrowDown") {
      e.preventDefault();
      setOpen(true);
      setActive((i) => Math.min(i + 1, Math.max(options.length - 1, 0)));
    } else if (e.key === "ArrowUp") {
      e.preventDefault();
      setActive((i) => Math.max(i - 1, 0));
    } else if (e.key === "Enter") {
      // Never submits the form from here: Enter picks the highlighted person.
      e.preventDefault();
      const p = showList ? options[Math.min(active, options.length - 1)] : undefined;
      if (p) pick(p);
    } else if (e.key === "Escape") {
      setOpen(false);
    } else if (e.key === "Backspace" && !query && chosen.length > 0) {
      remove(chosen[chosen.length - 1]);
    }
  };

  return (
    <div className="rcc">
      {chosen.length > 0 ? (
        <ul className="rcc-chips" aria-label="Copied">
          {chosen.map((email) => {
            const p = byEmail.get(email.toLowerCase());
            return (
              <li key={email.toLowerCase()} className={p ? "rcc-chip" : "rcc-chip rcc-other"} title={email}>
                <span className="rcc-name">{p ? p.name : email}</span>
                {p ? null : <span className="rcc-note">Not a {family} member</span>}
                <button
                  type="button"
                  className="rcc-x"
                  aria-label={`Remove ${p ? p.name : email}`}
                  onClick={() => remove(email)}
                >
                  ×
                </button>
              </li>
            );
          })}
        </ul>
      ) : null}
      <div className="rcc-box">
        <input
          id={id}
          type="text"
          role="combobox"
          autoComplete="off"
          placeholder={`Search ${family} members, or type a ${CC_ADDRESS_DOMAINS.map((d) => `@${d}`).join(" or ")} address`}
          aria-expanded={showList}
          aria-controls={listId}
          aria-autocomplete="list"
          aria-activedescendant={showList ? `${listId}-${Math.min(active, options.length - 1)}` : undefined}
          aria-describedby={describedBy}
          value={query}
          onChange={(e) => {
            setQuery(e.target.value);
            setOpen(true);
            setActive(0);
          }}
          onFocus={() => setOpen(true)}
          onBlur={() => setOpen(false)}
          onKeyDown={onKeyDown}
        />
        {showList ? (
          <ul className="rcc-list" id={listId} role="listbox">
            {options.map((p, i) => (
              <li
                key={p.email.toLowerCase()}
                id={`${listId}-${i}`}
                role="option"
                aria-selected={i === active}
                className={i === active ? "rcc-opt on" : "rcc-opt"}
                // mousedown, not click: a click lands after the input's blur
                // closed the list.
                onMouseDown={(e) => {
                  e.preventDefault();
                  pick(p);
                }}
              >
                {p.email === addable ? (
                  <span className="rcc-name">Add {p.email}</span>
                ) : (
                  <>
                    <span className="rcc-name">{p.name}</span>
                    <span className="rcc-mail">{p.email}</span>
                  </>
                )}
              </li>
            ))}
          </ul>
        ) : null}
        {open && q && options.length === 0 ? (
          <p className="rcc-none" role="status">
            No {family} member matches “{query.trim()}”. Type a whole{" "}
            {CC_ADDRESS_DOMAINS.map((d) => `@${d}`).join(" or ")} address to add someone else.
          </p>
        ) : null}
      </div>
    </div>
  );
}
