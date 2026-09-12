# Updates

The Updates section looks up WSO2 product update level release notes, the same
information published for WUM/update levels, so you can check what changed
between two update levels of a product version without leaving the portal.

## Searching between two update levels

Pick a product, then a base version of that product, then a start level and an
end level, and select **Search**:

- **Product** and **Version** are populated from the product catalog. Selecting
  a product resets the version and level fields below it; selecting a version
  resets the level fields.
- **Start level** always offers `0` (meaning "no updates installed yet") in
  addition to the real update levels available for that version.
- **End level** only lists levels higher than the chosen start level, so the
  range is always valid.
- **Search** is disabled until all four fields are set and the end level is
  greater than the start level.
- **Clear** resets all four fields and drops the current results.

The results table lists every update level in the range, its type (**Security**,
**Regular**, or **Mixed**), and a **View** action.

## Viewing update details

Select **View** on a row to open the full release notes for that update level:
one block per update number, each with its description, install instructions
(when there's more than "N/A"), bug fixes, added/modified/removed files, and any
security advisories with their severity and overview. Descriptions and advisory
overviews may contain formatted (HTML) release-note text; the portal renders
that formatting and sanitizes it, or falls back to plain text if the content
isn't HTML.

## Report preview and PDF export

Once a search returns results, two extra actions become available:

- **Preview report** opens a single scrollable view of every update in the
  current result set, the same detail shown per level, laid out for reading top
  to bottom instead of level by level.
- **Download PDF** generates a PDF of that same report for offline reference or
  sharing.

Both actions are tied to the exact filter values you searched with. If you
change any of the Product / Version / Start level / End level dropdowns after
running a search, both buttons are disabled again until you select **Search**
with the new values; this avoids downloading or previewing a report that
doesn't match what's on screen.

## Refreshing results

While a result set is showing, use **Refresh updates** to re-run the search
against the current filter values and see when it was last updated.
