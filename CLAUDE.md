# cs-tools

Repo-wide conventions. Component-specific guidance lives in that component's own
`CLAUDE.md` (`entity-service/`, `apps/csm-portal/backend/`, `apps/csm-portal/webapp/`,
each `integrations/*`, …) — this file is only for things that hold everywhere.

## Read a component's specs before changing it, and keep them true to the code

Each component's `docs/` folder holds specs (`entity-service/docs/`,
`integrations/csm-notification-service/docs/`, …) that explain behavior and the
reasons behind it, which the code alone does not. Read the ones relevant to the
area you are touching before editing it, not just the code.

The code is the source of truth. If a spec and the code disagree, the spec is
stale: fix the spec to match the code in the same change, never the reverse, and
never leave a known mismatch in place. Keep spec edits short.

## Run the component's review.md against the diff before every commit

Before committing a change to any component that has a `review.md`
(`entity-service`, `apps/csm-portal/backend`, `apps/customer-portal/backend-v2`, …),
read that component's `review.md`, check every item against the staged diff, and
state what matched — and why — before running `git commit`. Do not silently fix and
move on. Fixes go into the same commit.

Do this on every commit, not only the one that opens the PR: the checklist comes
from past reported bugs, and a later commit on the same branch can reintroduce one.
If a review turns up a failure class that is not on the list, add it to that
`review.md` in the same style (one line, grounded in what actually happened).

Nothing enforces this — there is no hook — so it is a manual gate.

## Always pull and check for conflicts before pushing

Never push a branch without first fetching the target and checking whether it
still merges. `upstream/dev-app-csm-portal` moves fast — it advanced 22 commits
during a single working session — so a branch that merged cleanly an hour ago
may not now, and finding that out at push time (or in review) is later than
necessary.

```bash
git fetch upstream dev-app-csm-portal
git merge-tree --write-tree HEAD upstream/dev-app-csm-portal   # exit 0 = clean, 1 = conflicts
```

`merge-tree` is read-only — it never touches the worktree or the index, so it is
safe to run with uncommitted changes present. Note that it compares *commits*:
uncommitted work is invisible to it, so commit first or check your modified paths
against the target's own changed paths by hand.

Two remotes exist and they are not interchangeable: `upstream` is
`wso2-open-operations/cs-tools` (the real target) and `origin` is a personal fork
whose `dev-app-csm-portal` may be thousands of commits stale. Check against
`upstream`.

## Migrations are keyed by filename — renaming one makes it new

`entity-service/migrations/` is `NNNN_<description>.sql`, forward-only, one file
per migration (see `entity-service/CLAUDE.md`, "Database migrations"). `make
migrate` records each applied file in `csm_migration_applied_migration` by its
**full filename**, and applies every `migrations/*.sql` it has not recorded, in
name order. Three consequences:

- **A number used twice is not a conflict.** Two files that share a number
  both apply, each under its own name (`0026_account_contact_table` and
  `0026_tag_tables` both exist). Git will not warn about it either, so if the
  order between two same-numbered files matters, check which one sorts first.
  Do not rely on the number.
- **A renamed file looks new.** Against a database that already applied the old
  name, it runs again from scratch. It must be safe to re-run, or the rename
  needs a stated reason and a plan for the servers that already have it.
- **Only `NNNN_*.sql` belongs in that folder.** The runner globs `*.sql`, so an
  old-style `000NNN_x.up.sql`/`.down.sql` pair would run both halves, and
  would run them ahead of `0001`. That is how the Team Schedule's original
  files broke `make migrate` until they were rewritten as 0152–0155.

`scripts/csm-compose/migrate-and-seed.sh` (the local compose stack) still
iterates the old `*.up.sql` names and keeps its own `schema_migrations` table.
It has not been moved to the new convention yet.
