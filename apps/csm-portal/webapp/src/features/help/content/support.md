# Support

Support (`/cases`) is the cases list and case detail view, where most engineers spend most
of their day. This topic covers finding cases, saving filter combinations you use often, and
working a case once you're on its detail page.

## Finding cases

The search box at the top of the list matches on case number, subject, or internal ID only —
use the **Assignee**, **Project**, or **CRE Team** filters below it to narrow by assignee,
project, or CRE team instead. Next to the search box, **Filters** expands a grid of additional
controls, with a **Simple** / **Advanced** toggle above it:

- **Simple** (the default) is a fixed grid: **Severity**, **State**, **CRE Team**, **Case
  type**, **Assignee** (search the engineer directory, or pick "Me"), **Product**, **Onboarding
  status**, and **Project**.
  - **Severity**: S0 (Catastrophic) through S4 (Low / Query). S0 is reserved for Managed Cloud
    projects.
  - **State**: Open, Work in progress, Solution proposed, Awaiting info, Waiting on WSO2,
    Closed. Click a state once to require it, again to exclude it instead, a third time to
    clear it — the chip's color tells you whose move it is: blue for active states on us
    (Open, Work in progress), amber for an elevated on-us state (Waiting on WSO2), grey for
    waiting on the customer (Solution proposed, Awaiting info), green for Closed.
- **Advanced** replaces the grid with a list of field/operator/value rows — every field Simple
  mode covers, plus more (**Tags** — include or exclude a tag by picking the "includes" or
  "excludes" operator on its row — SRE team, project type, escalation, SLA business-elapsed
  percent, created/updated/closed date ranges, created-by, and a few opaque IDs). Click **Add
  filter** for a new row, pick its field, operator, and value, and the trash icon to remove it.
  Below the rows, **OR groups** lets you combine several conditions with OR instead of the
  default AND (e.g. "Severity is S1 OR Type is Engagement").
- The bar switches to Advanced automatically if the filters it's showing (from a saved view, a
  shared link, or a dashboard click-through) include anything Simple mode can't express — Tags
  is the most common reason. **Simple** is greyed out (with a tooltip explaining why) whenever
  the current filters can't be shown there; clear the Advanced-only ones to switch back.

When a case list arrives already filtered, for example after clicking into a dashboard
widget, an Advanced-only filter (an SLA-percent bound, an escalation filter, and so on)
switches the bar into Advanced mode automatically, where the filter row or OR group it landed
in is itself the way you see and remove it — Advanced mode doesn't show a separate chip for
it. The one exception is the work-state sub-filter (Ongoing/Paused, a narrow slice of State
that stays Simple-representable and has no dedicated control of its own): if one arrives, say
from a dashboard click-through, while you're in Simple mode, it shows up as a removable chip
above the grid instead.

Once any filter is active, the **Filters** button turns into **Clear filters (N)**, showing
how many are active and clearing all of them in one click.

## Saved filter views

The **Saved views** button next to search lets you save the filters you currently have set as
a named view, and reapply it later without rebuilding it by hand, useful for something you
check every day, like "my open S1/S2."

A few things worth knowing:

- **Saved views live in your browser only.** They're stored in this browser's local storage,
  not on the server; they don't sync across devices, and a teammate can't see or share your
  saved views. Switching browsers or clearing site data loses them.
- You can save up to 50 views. Saving with a name that matches an existing view (case-
  insensitively) overwrites it rather than creating a duplicate.
- Whichever view's filters exactly match what's currently applied is checked in the menu.
- Delete a saved view with the trash icon next to it in the menu.
- Reorder your saved views with the up/down arrows next to each one in the menu; a freshly
  saved view still jumps to the top of the list.

## Case detail

Opening a case shows its full detail: overview fields, the comment/activity timeline,
attachments, and any linked service requests.

If a customer submits Case Feedback (a CSAT survey, typically sent once a case closes), it
appears in the activity timeline alongside comments and status changes, in its correct
chronological position — showing the submitter, star rating, and any free-text comment they
left.

**Linked service requests** appear in their own widget on the case, listing each linked
request's number, name, state, and current assignee. Clicking a row navigates to that
request's own case page (with a "back" link that returns you here). You can start a new
linked request either from that widget or from the case's **More** menu ("Create service
request…") — both open the same pre-filled form. If a case is closed, the **Create service
request** action is disabled in both places: closed cases are read-only.

Service requests aren't available on every project. If the selected project isn't eligible,
the create form shows a warning and blocks submission.

## Watchers

The **Watchers** tab lists everyone notified on updates to the case, and lets you add or
remove people (a case must always keep at least one watcher, so the last one can't be
removed).

If you're not on the list, a **Follow case updates** button adds you with one click. If
you're already watching, it becomes **Unfollow case updates** to take yourself off the list —
unless you were added automatically as the case's assigned engineer, in which case Unfollow is
disabled with a tooltip explaining why.

## Comments

The comment composer at the bottom of the timeline sends either a public reply visible to the
customer, or an **internal note**:

- Toggle **Internal note** (the lock icon) to write a note only WSO2 engineers can see. The
  composer's background tints and gets a left accent bar while this is on, and existing
  internal notes are shown the same way in the timeline with an "Internal note" chip, so
  customer-visible and internal content are never visually ambiguous.
- If public replies are currently locked for the case (for example, work is paused), the
  composer forces you into internal-note mode and shows why, with a one-click "Resume work"
  action when that's the reason, so you don't have to leave the composer to unblock a public
  reply.
- The **HTML source** toggle switches the editor to raw HTML for fixing paste formatting or
  inserting a table; whatever you type there is sent as-is.
- Attach files from the editor's toolbar before sending; drag-and-drop onto the composer works
  too.

Comments in the timeline are color/role-tagged (Customer, WSO2, System, AI Agent) so you can
scan who said what at a glance, and each has a permalink (click the timestamp) for referencing
a specific comment.

**Request update…**, in the case's **More** menu, posts a customer-visible comment nudging the
customer for a response — pick a first, second, or final reminder (each shown as a read-only
preview of the exact wording that will be posted) or write a custom message instead. It's only
offered while the case is **Awaiting info** or has a **Solution proposed**, since those are the
states where a reply from the customer is actually expected.

**Hold auto-closure…**, also in the case's **More** menu, exempts a case from the automated
auto-closure sequence until the date you pick. Setting or extending the hold automatically
posts an internal note in the timeline recording the hold and its date, so anyone on the case
can see when a hold was set or moved — resending the same date (e.g. an accidental re-submit)
doesn't post a duplicate note.

## Attachments

Click an image or PDF attachment to preview it: images open inline in a dialog; PDFs open in a
new browser tab (this is a browser limitation, not a portal choice: Chrome won't render a PDF
inside the sandboxed preview). Other file types don't have an inline preview; download them
instead.
