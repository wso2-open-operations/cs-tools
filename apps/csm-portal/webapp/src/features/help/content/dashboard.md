# Dashboard

Dashboard is your landing page. It shows a grid of widgets (cases, requests, and other work)
summarized as counts, lists, or charts, so you can see the state of your work at a glance
without opening the underlying tabs.

## Choosing a dashboard

The dropdown at the top of the page switches between the dashboards available to you. Which
dashboard you land on by default depends on your own team membership: if you belong to a
team, the portal picks that team's dashboard; if you don't have a team (or your profile
hasn't resolved one), you get the general default dashboard instead. Some roles (for example
onboarding or migration specialists) always land on their own dedicated dashboard regardless
of team.

Whichever dashboard you're on is reflected in the URL, so a link to a specific dashboard (and,
for team-based ones, a specific team) is shareable and survives a refresh.

## The team selector

Some dashboards are team-based and show a second dropdown next to the dashboard selector for
picking a team. It defaults to your own team if you have one, or to **All ABTs** if you
don't: "All ABTs" combines every team in that dashboard's own family (for example every CRE
team, or every SRE team), not every team in the portal. Switching the team re-scopes every
widget on the page to that team's data; switching between two team-based dashboards keeps
your team selection, while switching to a dashboard that isn't team-based hides the selector.

## Reading a widget

Each widget renders as one of a few shapes:

- **Count**: a single big number (for example, open cases assigned to you). The whole tile
  is a link: clicking anywhere on it takes you to the filtered list behind that number.
- **List**: a small table of the most recent matching records, using the same list component
  as the corresponding tab (for example, cases render through the same table as the Support
  section). It shows a capped number of rows.
- **Pie / bar chart**: a breakdown of a widget's records by category. Clicking a slice or bar
  navigates to the list filtered to just that category; clicking elsewhere on the tile (the
  title area, the empty space) goes to the widget's unfiltered base list.

A widget with a description shows it as an info icon (count tiles) or as a subtitle (pie/bar
tiles); hover or focus the icon to read it.

## Seeing more of a list widget

A list-shape widget only shows a handful of rows at a time. When it has more matching records
than that, a **View more** link appears under the table. It opens a dedicated preview page for
that widget with the same filters already applied, but with real pagination and (for
case-based widgets) the full, editable case filter bar, so you can search, filter further, or
page through everything the widget is summarizing, rather than just its first few rows.

This is different from clicking a count tile or a pie slice, which takes you straight to the
matching records in the resource's own tab (Support, Operations, and so on) with those same
filters carried over.
