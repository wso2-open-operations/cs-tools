/**
 * Props that keep a Select-backed `TextField`'s label where the fields beside
 * it put theirs.
 *
 * oxygen-ui's own theme targets
 * `.MuiFormControl-root:has(.MuiSelect-select) &:not(.MuiInputLabel-shrink)`
 * and lifts an UNSHRUNK label by `top: -7px`. Almost every select in PLG opens
 * on an "All …" option whose value is the empty string, so almost every one of
 * them renders in exactly that state — which is why the labels read as sitting
 * too high next to the text fields and buttons on the same row.
 *
 * That compound `:has()`/`:not()` selector out-specifies anything `sx` emits,
 * so `!important` is the only thing that reliably wins. csm-portal reached the
 * same conclusion twice — see `MultiSelectField.tsx` and
 * `ProductVulnerabilitiesTab.tsx` — and this is those two fixes with the value
 * test factored out, because PLG has fifteen of these rather than two.
 *
 * `shrink` and `notched` follow the field's own value instead of MUI's
 * focus-driven default, so the label and the outline's notch agree whether or
 * not the field happens to have focus.
 */
export function selectLabelProps(value: unknown) {
  const hasValue = value !== "" && value !== null && value !== undefined;
  return {
    slotProps: {
      inputLabel: { shrink: hasValue, sx: { top: "0px !important" } },
      select: { notched: hasValue },
    },
  };
}
