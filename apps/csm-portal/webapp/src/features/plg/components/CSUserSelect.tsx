import { Autocomplete, TextField } from "@wso2/oxygen-ui";
import { useMemo, useState, type JSX } from "react";

import { useDebouncedValue } from "@hooks/useDebouncedValue";
import { useCSUsers } from "@features/plg/api/hooks";
import type { UserRef } from "@features/plg/api/types";

/**
 * The PLG CS engineer picker.
 *
 * WHY THIS IS NOT A PLAIN `<TextField select>`, WHICH IS WHAT IT USED TO BE.
 * There are ~1,900 active internal users and the API page limit is 100, so the
 * dropdown only ever listed the first 100 by first name — every engineer from
 * roughly the A's onward was unreachable, with nothing on screen saying so. A
 * picker that silently omits 95% of the people it offers is worse than one that
 * is obviously incomplete.
 *
 * Typing now searches the whole directory server-side. Opening it without
 * typing still shows the same first 100, so the common case — assigning to
 * someone you can see — is unchanged.
 *
 * Debounced at 300ms with no minimum length, matching AsyncAccountMultiSelect,
 * the same pattern csm-cases already uses for account search.
 */
export function CSUserSelect({
  value,
  onChange,
  label,
  extraOptions,
  selected,
  disabled,
  required = false,
  size = "small",
  sx,
}: {
  /** The chosen engineer's id, or "" for none. */
  value: string;
  /** The chosen id, and the record it came from so a caller can keep it. */
  onChange: (id: string, user: UserRef | null) => void;
  label: string;
  /**
   * Fixed entries shown above the engineers — "Anyone" and "Unassigned" on the
   * list filter. They are UserRefs with a sentinel id so one option type serves
   * the whole list; the picker on the detail page passes none.
   */
  extraOptions?: UserRef[];
  /**
   * The engineer `value` names, when the caller already knows them.
   *
   * Needed because the options are one page of a much longer list: an owner
   * outside the current 100 is not in `options`, and Autocomplete renders a
   * value it cannot find as blank — so an organisation with an owner would look
   * unassigned. Passing them here keeps the name on screen.
   */
  selected?: UserRef | null;
  disabled?: boolean;
  /** Marks the field required (asterisk and aria-required). Display only: the
   *  caller still holds back a save without a value. */
  required?: boolean;
  size?: "small" | "medium";
  sx?: object;
}): JSX.Element {
  const [input, setInput] = useState("");
  const search = useDebouncedValue(input, 300);
  const { data: users, isFetching } = useCSUsers(search);

  const options = useMemo(() => {
    const fixed = extraOptions ?? [];
    const list = users ?? [];
    // The selected engineer next, and only if the page does not already carry
    // them — otherwise they appear twice.
    const withSelected =
      selected && !list.some((u) => u.id === selected.id) ? [selected, ...list] : list;
    return [...fixed, ...withSelected];
  }, [users, selected, extraOptions]);

  const current = options.find((u) => u.id === value) ?? null;

  return (
    <Autocomplete
      options={options}
      value={current}
      disabled={disabled}
      size={size}
      sx={sx}
      loading={isFetching}
      // The server has already matched; filtering again here would hide rows it
      // deliberately returned (an email match whose name does not contain the
      // term, say).
      filterOptions={(o) => o}
      getOptionLabel={(u) => u.name || u.email}
      isOptionEqualToValue={(a, b) => a.id === b.id}
      onChange={(_, picked) => onChange(picked?.id ?? "", picked)}
      inputValue={undefined}
      onInputChange={(_, text, reason) => {
        // "reset" fires when a value is chosen; clearing the term then would
        // throw away the results the user is still looking at.
        if (reason === "input") setInput(text);
        if (reason === "clear") setInput("");
      }}
      noOptionsText={search ? "No engineer matches that" : "No engineers found"}
      renderInput={(params) => (
        <TextField {...params} label={label} required={required} placeholder="Search by name/email" />
      )}
    />
  );
}
