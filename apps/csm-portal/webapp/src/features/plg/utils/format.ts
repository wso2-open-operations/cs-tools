/** Display helpers shared by every feature. */

const dateFormatter = new Intl.DateTimeFormat("en-GB", { day: "2-digit", month: "short", year: "numeric" });
const dateTimeFormatter = new Intl.DateTimeFormat("en-GB", {
  day: "2-digit",
  month: "short",
  year: "numeric",
  hour: "2-digit",
  minute: "2-digit",
});

export function formatDate(value: string | null | undefined): string {
  if (!value) return "—";
  const d = new Date(value);
  return Number.isNaN(d.getTime()) ? "—" : dateFormatter.format(d);
}

export function formatDateTime(value: string | null | undefined): string {
  if (!value) return "—";
  const d = new Date(value);
  return Number.isNaN(d.getTime()) ? "—" : dateTimeFormatter.format(d);
}

/** "3 days ago" / "in 5 days" — used for due dates and last-activity columns. */
export function formatRelative(value: string | null | undefined): string {
  if (!value) return "—";
  const d = new Date(value);
  if (Number.isNaN(d.getTime())) return "—";
  const diffDays = Math.round((d.getTime() - Date.now()) / 86_400_000);
  if (diffDays === 0) return "today";
  if (diffDays === 1) return "tomorrow";
  if (diffDays === -1) return "yesterday";
  return diffDays > 0 ? `in ${diffDays} days` : `${Math.abs(diffDays)} days ago`;
}

/**
 * Words that are acronyms, not prose, and must keep their case.
 *
 * Sentence-casing is right for MANUAL_REVIEW_REQUIRED and wrong for the handful
 * of enum values built from initialisms: PLG_CS_ELIGIBLE became "Plg cs
 * eligible" and PAYG became "Payg", in dropdowns and chips alike. Fixing it in
 * the one function every label passes through keeps the filter list, the chip
 * and the detail panel saying the same thing.
 *
 * Keyed on the lower-cased word so the lookup happens after the split, and
 * matching is per word — PLG_CS_ELIGIBLE needs two substitutions and keeps
 * "eligible" as ordinary prose.
 */
const ACRONYMS: Record<string, string> = {
  plg: "PLG",
  cs: "CS",
  payg: "PAYG",
};

/** MANUAL_REVIEW_REQUIRED → "Manual review required"; PAYG → "PAYG". */
export function humanizeEnum(value: string | null | undefined): string {
  if (!value) return "—";
  const words = value.toLowerCase().split("_");
  return words
    .map((word, i) => {
      const acronym = ACRONYMS[word];
      if (acronym) return acronym;
      // Only the first word is capitalised: the result is a sentence, not a
      // title, so "First value achieved" rather than "First Value Achieved".
      return i === 0 ? word.charAt(0).toUpperCase() + word.slice(1) : word;
    })
    .join(" ");
}

