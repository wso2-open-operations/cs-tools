# Overview

The CSM portal is the internal tool CS engineers use to handle customer support work:
cases, service and change requests, incidents, professional-services engagements, security
findings, product/version updates, time tracking, and the customer/account data those all
reference. It is separate from the customer-facing portal: customers never see this app.

This Help section lives under **Help** in the left sidebar, right after **Settings**. Each
topic below covers one part of the portal in more detail than this page does.

## The portal at a glance

The sections below appear in the left sidebar in this order:

- **Dashboard**: your landing page: configurable widgets summarizing cases, requests, and
  other work.
- **Support**: cases: the core case list, case detail, and comment trail.
- **Team Schedule**: who is on duty right now, the team rota, and who is away. Team leads can
  edit their own team's rota.
- **Operations**: five tabs: Service requests, Change requests, Incidents, Problem
  management, and Outages.
- **Engagements**: professional-services work such as migrations, implementations,
  onboarding, and training.
- **Announcements**: portal-wide announcements.
- **Security Center**: two tabs: Security reports and Vulnerabilities.
- **Customers**: two tabs: Accounts and Projects.
- **PLG**: the PLG Customer Success Portal pages.
- **Updates**: product/version update tracking.
- **Time cards**: engineer time logged against a case, with an approval flow.
- **Team Schedule**: the team rota.
- **Settings**: user management: users, roles, groups, teams, and permissions.
- **Help**: this section.
- **PLG**: the PLG Customer Success Portal, with six tabs: Leadership Dashboard, Dashboard,
  My Work, Organisations, New Registrations, and Manage Playbooks.

Accounts set up for the Sales and Solutions Architecture view see a different sidebar in place
of the one above: **Sales / Solutions Architecture**, with six tabs: Accounts, Projects, Team
schedule, User scan, Usage metrics, and Customer health.

Most of these sections have their own topic further down this page with the specifics.

## Team Schedule

Team Schedule shows the on-duty rota. It has up to four tabs, and you land on
the first one that applies to you:

- **My week**: your own shifts for the week, with what's on each day. Engineers
  open on this tab; managers who hold no rota of their own don't see it and
  open on **Who is working today** instead.
- **Who is working today**: everyone on duty for the selected day, arranged as
  a ladder of time blocks.
- **Who is working this week**: a Monday-to-Sunday table of who covers what.
- **Month roster**: a grid of people against days, with leave shown alongside
  rotations. Choose how many months it spans (one, three or six); it is
  centred on the selected day, so it opens and closes on the days either side
  of it rather than on month boundaries.

**Previous/Next** step by a day, a week or a month depending on the tab, the
date field jumps to any day, and **Today** returns to the current day. Group
(CRE or SRE) and team controls sit on each view; a team picked on one view
doesn't hide your own people on **My week**.

**Time zone.** Every time on this page is shown in the time zone set on your
profile, not your computer's. "Today", the highlighted day, the current-week
band and the weeks and months fetched all follow that same clock, and they
roll over at your profile's midnight. If you travel, change the zone on your
profile and every view follows. There is deliberately no zone picker on this
page, so two people comparing a rota are always reading the same clock.

**Editing the rota.** Team leads see **Edit rota** on the Month roster for
their own team. In edit mode each editable cell is a button: click it, or tab
to it and press Enter or Space, to open the picker for that day and change the
shift or leave. Changes are marked on the roster until you select **Done
editing**, and **Recent changes** lists what has changed on your teams' rota
and leave.

## Signing in and staying signed in

If your profile can't be loaded after you sign in, the portal shows an error
with a **Try again** button rather than leaving you on a loading screen; select
it to retry the load.

For your security the portal signs you out automatically after a period of
inactivity. A warning appears first; if you don't respond to it, you are
signed out and need to sign in again. Anything you hadn't saved is lost, so
save work before stepping away.

## Jumping to a person's profile

Wherever a person's name appears (a case's creator or assignee, a comment's author, and
similar references throughout the portal), clicking it opens their profile page. If the name
can't yet be resolved to a profile (no id is available), it's shown as plain, unclickable
text instead of a broken link.

Shortcuts for navigating between pages and personalizing your workspace (pinning, quick-nav,
and similar) are covered in the **Navigation & personalization** topic.
