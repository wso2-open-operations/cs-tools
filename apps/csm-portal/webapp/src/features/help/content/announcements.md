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

Use **New announcement** on the list page to publish one. You provide a
subject, a description (rich text), and one or more target projects. Picking
several projects doesn't create one shared announcement — it creates one
independent case per project, each with its own comment thread; if you later
reply to "the announcement," you're replying to one specific project's copy
of it, not all of them at once.

There's no draft state: an announcement is live for its target project(s) as
soon as you submit. If some of the selected projects fail to create (and
others succeed), the ones that succeeded are already published; you're told
which project(s) failed so you can retry just those.

## What you can't do yet

- **Targeting is project-level only.** There's no way to target a tier or an
  "Asgardeo customers only" audience yet — pick specific projects one by one,
  or all of them.
- **No unpublish or edit.** Once created, an announcement can't be
  retargeted, edited, or taken down from the CSM portal.
