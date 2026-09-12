# Settings

Settings is where you look up the platform's identity data: who exists, what roles they
carry, which assignment groups they belong to, and which team registry entries they're
part of. It's a set of read-only directories today, not a configuration console; there's
no create, edit, or delete here yet for any of Users, Roles, Groups, or Teams.

Open Settings from the sidebar and you land on a tile grid, one tile per directory, rather
than a tab strip. A tile marked **WIP** isn't wired up yet; the tiles that aren't marked WIP
open their real directory.

## Users

The Users directory lists every account on the platform. Search by username or email
(case-insensitive), and narrow the list with the Roles, Groups, Teams, and Status filters;
they combine, so picking a role and a team returns only users who match both.

Click any row to open that person's profile, which shows their full role list and other
details the table truncates. There's no way to change a user's roles, deactivate an account,
or edit any field from here: this view is for finding people and checking what they're
assigned, not managing them.

## Roles

Roles shows the platform's curated catalogue of assignable role keys, not every role concept
the backing identity source knows about, just the ones this platform actually uses. Search by
name, then click a role to see the list of users who hold it.

Like Users, this is a lookup, not an editor: you can't create a role or add/remove someone's
role assignment from this screen.

## Groups

Groups mirrors assignment groups from the backing data source with a live search: this list
can be larger than the curated Roles or Teams catalogues, since it reflects whatever groups
exist upstream. Click a group to see its member list.

Read-only, same as the others: group membership is managed upstream, not from this page.

## Teams

Teams is the platform's own team registry, a curated list like Roles, rather than a live
query against an external source. Each team has a name and a family/grouping label; click one
to see who's on it.

Also read-only for now: no way to create a team or edit its membership from this screen.

## Permissions

Permissions is not built yet. Opening it shows a "coming soon" notice that names what it's
waiting on: the backend permissions endpoints. When it ships, it's meant to be a fine-grained
permission catalog and assignment view, but there's nothing to configure there today.

## Who can see this

Settings itself (Users, Roles, Groups, Teams, and Permissions) is visible to every signed-in
CS engineer, the same as most of the portal; there's no role check hiding the section or its
directories.
