# Navigation & personalization

These controls live in the top header bar and work from anywhere in the
portal: you don't need to be on a case, project, or account page to use
them.

## Quick navigation

Click the "Search or jump to…" box in the header, or press **⌘K** (Mac) or
**Ctrl+K** (Windows/Linux), to open the quick-nav palette.

Type to search across:

- Cases, incidents, change requests, problems, and conversations: matched
  by number/id first, falling back to free-text search of subject and
  description
- Your pinned pages and recently viewed items
- Portal pages themselves (both top-level sections and second-level tabs,
  such as jumping straight to the Incidents tab instead of just Operations)

If what you type matches a known number pattern (a case number, a WSO2 case
id, an incident/problem/change-request number, or a conversation number),
quick nav shows only that exact match with a banner confirming what it
matched on. Use the "Search in subject and description too" button in that
banner to widen the search to free text across all of those record types.

If nothing matches a known number pattern, quick nav still searches subject
and description text across all types and shows a note that it didn't
recognize a specific number format.

Use the arrow keys to move between results and **Enter** to open the
selected one, or click any result directly. Press **Esc** or click outside
the palette to close it without navigating anywhere.

## Pinning a page

Pinning keeps a page one click away in the header, regardless of what else
you navigate to.

1. Open the page you want to keep handy: a case, project, account, a
   filtered list view, a dashboard, or any other page in the portal.
2. Click the pin icon in the header's action area (it reads "Pin this page
   to top nav bar" when unpinned).

The page appears as a chip in the header's pinned tabs strip, next to the
quick-nav box. Click a chip to jump back to that page; click its close (×)
control to unpin it. The chip for whichever pinned page you're currently on
is visually highlighted so you can tell where you are.

To unpin a page you're currently viewing, click the same header icon again:
it toggles to "Unpin this page from the top nav bar" once the page is
pinned.

Pinned pages are not evicted automatically. Unlike your recently viewed
history, there's no cap on how long a pin sticks around; it stays until
you unpin it yourself.

## Renaming a pinned tab

Pinned tab chips get an automatically generated title, which is not always
the label you'd want at a glance (for example, a filtered view's title
summarizes its filters).

1. Right-click the pinned tab's chip in the header.
2. Choose **Rename** from the menu that appears.
3. Edit the name in the "Rename pinned tab" dialog and click **Save**.

An empty or whitespace-only name is rejected: the Save button stays
disabled until you enter something.

## Recent views

The clock/history icon in the header's action area opens a "Recently
viewed" panel, separate from your pinned tabs. It lists pages you've
visited recently, grouped by type (cases, projects, accounts, incidents,
change requests, problems, searches, pages), most recent first, each with a
relative timestamp.

Unlike pinning, recent views build up automatically just by visiting pages;
you don't have to do anything to add an entry. Only your last 12 unpinned
visits are kept; older ones drop off as you visit new pages. Pinned entries
are exempt from that cap and always stay in the list, marked so you can
unpin them from the same panel (via the pin icon on each row) without
opening the page itself.

Click **Clear history** in the panel to remove your unpinned recent items.
This never removes pinned entries: clearing history only resets browsing
history, not your deliberately pinned working set.

## What gets remembered

Both pinned tabs and recent-view entries store a title, not just a URL. For
case, project, incident, and similar detail pages, that title comes from
the record itself (for example, a case number and subject). For a page
without its own record (a dashboard, a filtered list, or any other route),
the title is derived from the page's name in the navigation menu, or, for
a filtered view, from a short summary of the applied filters (e.g. a quoted
search term, or a count of active filters). If the route isn't recognized
by the portal at all, the title falls back to a human-readable guess based
on the URL itself. This is why a pinned or recent entry can occasionally
show an unexpected name; you can always fix it via the rename flow above
for pinned tabs.

Your pinned and recent items are tied to your signed-in account and stored
in your browser; they don't carry over to a different browser or device,
and signing out clears them.
