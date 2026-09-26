# cs-tools

Repo-wide conventions. Component-specific guidance lives in that component's own
`CLAUDE.md` (`entity-service/`, `apps/csm-portal/backend/`, `apps/csm-portal/webapp/`,
each `integrations/*`, …) — this file is only for things that hold everywhere.

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

## Duplicate migration numbers are normal here — don't "fix" them

`entity-service/migrations/` has ~20 numbers used by two unrelated migrations
each (`000014` is both `create_alert_incident_mapping` and `deployed_product_table`;
so are `000067`, `000085`, and all of `000068`–`000081`). This is not corruption
and it does not need renumbering.

It works because the applier keys `schema_migrations` on each file's **full
basename**, not its number — see `scripts/csm-compose/migrate-and-seed.sh`, which
iterates `ls *.up.sql | sort` and records `000088_sn_id_mapping_columns` and
`000088_team_schedule_tables` as two separate, independently-applied rows. Two
branches adding the same number therefore merge and apply without conflict.

Two consequences worth knowing before touching anything here:

- **Git will not warn you about a number collision**, because the filenames differ.
  If ordering between two same-numbered migrations actually matters, basename sort
  decides it — so verify the dependency rather than trusting the number.
- **Renaming a migration makes it look new.** Since the recorded version is the
  basename, a renumbered file is re-applied from scratch on the next run against
  an existing database, which fails on `CREATE TABLE` and needs a volume reset.
  Renumber only with a specific reason, never for tidiness.
