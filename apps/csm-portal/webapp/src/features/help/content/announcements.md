# Announcements

The Announcements section lists customer-facing announcements published across
projects. Under the hood, an announcement is a case of type `announcement`,
so it shows up here using the same search and detail infrastructure as
regular cases, just trimmed down to what makes sense for a broadcast.

## Viewing the list

The list shows one row per announcement: number, subject, project, state,
who created it, and when it was last updated. Rows are real links, so you can
middle-click or cmd-click to open an announcement in a new tab, or copy its
URL.

You can narrow the list with:

- **Search**: matches subject or number.
- **State**: filter to one or more states (open, work in progress, solution
  proposed, awaiting info, waiting on WSO2, closed).
- **Project**: filter to one or more projects.

All filters default to "show all"; changing any filter resets the list back
to the first page. Use **Clear filters** to reset state and project in one
click.

## Opening an announcement

Selecting a row opens the announcement's detail page. It reuses the same
case detail view as regular cases, but with the parts that don't apply to a
broadcast hidden:

- No assignment, acknowledgement, or state-transition actions: an
  announcement isn't assigned to or worked by an engineer.
- No Related, Watchers, SLA, Time, or Call Requests tabs, since none of
  those concepts apply to an announcement.

The announcement's body is shown as its Description, rendered as rich text
(the same HTML editor content used elsewhere in the portal), so formatting,
links, and lists in the original announcement are preserved.

## Commenting on an announcement

Announcements aren't read-only anymore. An **Add comment** button reveals a
composer — the same one used on regular cases — offering a choice between:

- A **customer-visible comment**, which the targeted project's customers can
  see, for clarifying whether an announcement applies to them, follow-up
  details, etc.
- An **internal work note**, visible only to CS engineers.

A closed announcement no longer accepts new comments of either kind, same as
any other closed case.

## Creating an announcement

Use **New announcement** on the list page. You first choose the type:

- **Create announcement for customers**: you provide a subject, a rich-text
  description, and the audience: specific projects, or all customer projects
  (with options to exclude Cloud Support / Cloud Evaluation Support projects
  and Restricted / Suspended projects). An optional **security** label marks
  the announcement as a security announcement.
- **Product version / EOL announcement**: you pick a product and a version,
  and the announcement goes only to customers running that end-of-life
  version.

Nothing is sent when you create the request. It starts as a **draft**, and
you can **Save as draft** at any point. Picking several projects doesn't
create one shared announcement; once published it becomes one independent
case per project, each with its own comment thread, so replying to "the
announcement" later means replying to one project's copy.

## From draft to published

Every announcement goes through the **Requests** tab on the Announcements
page. Open a request to see its state and the action for that state:

1. **Draft.** The content is freely editable. **Submit for approval** runs a
   dry run first: it creates one real case in a fixed test project so your
   approver can see exactly what customers will get, then submits the
   request. Unsaved edits block Submit, so save first.
2. **Pending approval.** The request is read-only. An approver reviews it
   (the dry-run case is the thing to look at) and uses **Mark as approved**.
   **Edit** is available but asks for confirmation, because editing sends the
   request back to draft and clears its dry run.
3. **Approved.** The audience is now frozen to the list resolved at submit
   time and can't be changed; the subject and description can still be
   edited, and what is saved is what gets sent. Only the person who created
   the request can publish it, either:
   - **Publish** now. A confirmation shows the content and the recipient list
     first, then one case is created per project. If some projects fail, the
     ones that succeeded are kept and the dialog stays open so you can retry
     just the failed ones; the dialog can't be closed while a send is in
     progress. Each project's outcome is recorded as it completes, so a
     reopened request resumes where it left off and never sends a second case
     to a project that already has one.
   - **Schedule** a date and time (in your profile timezone) for it to be
     published automatically, and **Cancel schedule** to go back to manual
     publishing.
4. **Published.** The request becomes a read-only summary.

For a published request, its creator can **Post an update**: the update is
recorded and added as a comment on every case the announcement created, so
each project's thread shows it. Past updates are listed under the request.

## What you can't do yet

- **Targeting is by project or by product version only.** There's no way to
  target a tier or a customer group yet.
- **No unpublish or retarget.** Once published, an announcement can't be
  retargeted or taken down from the CSM portal; use updates to correct it.
