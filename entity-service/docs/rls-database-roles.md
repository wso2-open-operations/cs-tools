# Row-level security and database roles

Why people connecting straight to the database see "0 rows", the two ways to
fix it, and the runbook for the one that gives the service its own login role.

---

## 1. What is going on

Row-level security (RLS) is enabled on 23 tables (`work_item`, `"case"`,
`comment`, `announcement`, `deployment`, ... — the list is
`rlsProtectedTables` in `internal/repository/rls_schema_integration_test.go`)
with `FORCE ROW LEVEL SECURITY`. The policies read three transaction-local
settings that `repository.Scoped` sets for every query:

| setting | meaning |
|---|---|
| `app.is_internal` | `'true'` for staff / system work: sees everything |
| `app.viewer_email` | the caller, for customer scoping |
| `app.viewer_project_ids` | the caller's projects |

With none of them set the policies match nothing. That is deliberate: a code
path that forgets to identify the caller returns nothing instead of leaking.

Postgres exempts superusers and `BYPASSRLS` roles from RLS always, and exempts
a **table owner** unless the table is also `FORCE`d. The deployed setup uses
**one login, which owns the tables, for everything**: entity-service, DBeaver /
pgAdmin, the sync services, migrations. `FORCE` therefore binds all of them,
and a plain DBeaver session (no settings) reads **0 rows** from every protected
table. Catalog information (table list, sizes, columns) is not affected by RLS,
which is why the tree in a DB tool still looks normal.

Nothing is broken; the data is there. Two fixes:

### Option A: identify yourself in the tool (no change to the system)

In DBeaver set the connection's *Bootstrap queries* to

```sql
SET app.is_internal = 'true'
```

Every session that connection opens then reads as internal staff. Use it only
on connections for people who are allowed to see everything. Drawbacks: it is a
per-connection setting each person must add, and it does nothing for a tool
that cannot run a bootstrap query.

### Option B: give the service its own role (this document)

Split the one login in two:

| role | who uses it | RLS |
|---|---|---|
| **owner** (`csmpd_stg_user` today) | people, DBeaver, sync services, migrations | exempt (tables are `NO FORCE`) |
| **application role** (`csm_entity_app`) | entity-service only | **bound** by every policy, fail-closed |

RLS stays `ENABLE`d with all 85 policies. The application role neither owns
the tables nor has `BYPASSRLS`, so it is bound without `FORCE`. Anyone using
the owner login now sees all rows with no settings, which is what a DB tool
needs.

**The trade-off to accept:** the owner login (and any other service that uses
it) is no longer subject to RLS, exactly as before RLS existed. RLS protects
against bugs in entity-service's own code paths, not against anyone holding the
owner credential. Keep that credential limited to the people and jobs that
already have it.

Rejected alternatives:

* `ALTER ROLE <owner> SET app.is_internal = 'true'`: every connection by that
  role, **including entity-service**, would run as internal staff, which turns
  the customer isolation off. Never do this.
* A bypass keyed on `application_name`: the client chooses its own
  `application_name`, so it is a forgeable key.

---

## 2. What the PR adds

| file | purpose |
|---|---|
| `scripts/rls/create_app_role.sql` | creates the application login role and its grants (admin, once) |
| `scripts/rls/owner_exempt_no_force.sql` | `NO FORCE` on the 23 tables; **refuses to run** unless the application role is already connected |
| `scripts/rls/force_all_tables.sql` | rollback: `ENABLE` + `FORCE` on the 23 tables |
| `scripts/rls/emergency_disable.sql` | switches RLS off entirely, the break-glass switch |
| `internal/db/rls_protection.go` | startup check: is RLS in use but not applied to the role the service connects as? |
| `RLS_PROTECTION_REQUIRED` | with `true` the service refuses to start in that state; default `false` logs an `ERROR` |
| `RLS_SCHEMA_TEST_ALLOW_NO_FORCE` | test-only: lets the schema test accept `NO FORCE` when the test role is genuinely bound |

The PR changes no migration, table, policy or runtime query. Merging it changes
nothing in any environment until the steps below are run by hand.

---

## 3. Rollout (per database; strict order)

> **Order matters.** `FORCE` is the only thing that binds the table owner. If
> the service is still connecting as the owner when `NO FORCE` runs, RLS
> silently stops protecting it, and everything keeps working, so nobody
> notices. Steps 3 and 4 must not be swapped. Step 4's script refuses to run
> if the application role has no live connection, and step 3's startup check
> makes the mistake visible in the log.

Variables used below (set them in your own shell; do not paste passwords into
tickets or chat):

```bash
export PGHOST=...  PGPORT=5432  PGDATABASE=csm_platform_dev  PGSSLMODE=require
SCHEMA=csmpd_stg_user          # the schema that holds the tables
OWNER_ROLE=csmpd_stg_user      # the role that owns them today
APP_ROLE=csm_entity_app
```

1. **Prerequisite.** All raw-pool repositories must already be on
   `repository.Scoped`, so that nothing in entity-service reads a protected
   table without an identity (`TestRLSBypassLint_NoRawPoolAgainstAProtectedTable`
   is the guard). If not, those paths return 0 rows as soon as the service
   uses the application role.

2. **Create the application role** (a role that can create roles, once per
   database; safe to re-run). Pick a strong password and keep it in your
   password manager. `-v` puts the value in the process list for the duration of
   the command, so run it from a machine you control:

   ```bash
   read -rs -p "new app role password: " APP_DB_PASSWORD; echo; export APP_DB_PASSWORD
   psql -X -v app_password="$APP_DB_PASSWORD" -v schema=$SCHEMA \
        -v owner_role=$OWNER_ROLE -v app_role=$APP_ROLE \
        -f scripts/rls/create_app_role.sql
   ```

   The login that runs this needs `CREATEROLE`, and must be able to grant on
   the owner's tables and set default privileges *for* the owner: a superuser,
   the owner itself (if it has `CREATEROLE`), or a role that is a member of the
   owner role (`GRANT <owner_role> TO <admin_role>` first). Tested locally on
   PostgreSQL 17 with a non-superuser `CREATEROLE` admin: **without** that
   membership every grant fails with `permission denied for schema` (the script
   stops there and the role exists without privileges; re-run it after fixing
   the membership); **with** it everything succeeds. A
   `WARNING: no privileges were granted for "<db>"` on the `CONNECT` line is
   harmless (`CONNECT` is already granted to `PUBLIC`). A managed Postgres
   server admin is usually not a superuser, so expect to need the membership.
   *Not run on the real Azure server; confirm there before the window.*

   It creates the role (`NOSUPERUSER NOBYPASSRLS`), grants `SELECT/INSERT/
   UPDATE/DELETE`, sequence usage and function execute on the schema, sets
   default privileges so tables created by later migrations are granted
   automatically, and sets the role's `search_path` to the schema (the default
   `"$user"` would look for a schema named after the role). It changes no table
   and no existing role.

3. **Switch entity-service to the application role.** In Choreo, on the
   *Customer Entity Service* component only, set `DB_USER` and `DB_PASSWORD`
   to the new role and redeploy. Every other component keeps the owner login.
   Then check:
   * the service starts and its startup log says
     `RLS protection: role "csm_entity_app" is bound by row-level security on all 23 RLS-enabled tables`;
   * the portals load and a staff user and a customer user still see what they
     should;
   * no `permission denied for table ...` in the service log (a grant gap shows
     up here). It is a missing `GRANT` on an object the owner created outside
     the default-privilege rule, fixed by re-running step 2.

   Because the tables are still `FORCE`d at this point, this step changes
   nothing about who sees what; it is safe to stay here as long as needed.

4. **Make the owner exempt.** As the owner role:

   ```bash
   psql -X -v schema=$SCHEMA -v app_role=$APP_ROLE -f scripts/rls/owner_exempt_no_force.sql
   ```

   The script prints `REFUSING` and changes nothing if the application role has
   no live connection (service not switched, or idle with an empty pool: make
   one request and re-run). Otherwise it runs `NO FORCE` on the 23 tables in one
   transaction and prints `enabled=23, forced=0, policies=85`. Takes effect
   immediately, including on open connections; no restart.

5. **Turn on the guard** (optional but recommended once step 4 is verified):
   set `RLS_PROTECTION_REQUIRED=true` on entity-service. If anyone later points
   it at an owner / superuser / `BYPASSRLS` login, it refuses to start instead
   of silently running unprotected.

6. **Verify.**
   * As the owner in DBeaver with **no** bootstrap query:
     `SELECT count(*) FROM work_item` returns all rows.
   * As the application role without settings: `0` (fail-closed). With
     `SELECT set_config('app.is_internal','true',false)` the same query returns
     all rows.
   * `ALTER TABLE work_item DISABLE ROW LEVEL SECURITY` as the application
     role fails with `must be owner of table work_item`.
   * Portal checks as in step 3, including a customer login that must see only
     its own projects.

### Rollback

| to | run (as the owner) | effect |
|---|---|---|
| put `FORCE` back (owner is bound again, DBeaver shows 0 rows again) | `psql -X -v schema=$SCHEMA -f scripts/rls/force_all_tables.sql` | immediate, one transaction |
| everything off, break-glass | `psql -X -v schema=$SCHEMA -f scripts/rls/emergency_disable.sql` | immediate; re-enable with `force_all_tables.sql` |
| undo step 3 | set `DB_USER` / `DB_PASSWORD` back, redeploy | only needed if the role itself is the problem |

`force_all_tables.sql` is safe whichever login the service uses, which makes it
the first thing to run if anything looks wrong after step 4.

---

## 4. Things to know

* **New RLS tables.** A future migration that adds a protected table needs
  `ENABLE ROW LEVEL SECURITY`; `FORCE` is harmless but no longer what protects
  the application role. Add the table to the `NO FORCE` / `FORCE` scripts and
  to `rlsProtectedTables`.
* **Who creates objects.** The default privileges cover objects created by
  `OWNER_ROLE`. If migrations are ever run by a different login, repeat the
  `ALTER DEFAULT PRIVILEGES` for that role or the application role gets
  `permission denied` on the new tables.
* **Other services on the owner login** (sync services, migration job)
  become exempt from RLS at step 4. That is the intent, it is how they behaved
  before RLS. If one of them should be bound, give it its own non-owner role
  the same way.
* **Test setup.** `RLS_SCHEMA_TEST_ALLOW_NO_FORCE=true` makes
  `TestRLSSchemaIntegration_EveryProtectedTableHasForceRowLevelSecurity` accept
  un-forced tables, but only if the test's own role is bound; against an owner
  or superuser login it still fails. It is off by default so CI keeps asserting
  `FORCE`.
* **The startup check never takes the service down by accident.** If the
  catalog query itself cannot run it is logged and ignored; only a confirmed
  "RLS is enabled but does not apply to this role" with
  `RLS_PROTECTION_REQUIRED=true` stops startup.
