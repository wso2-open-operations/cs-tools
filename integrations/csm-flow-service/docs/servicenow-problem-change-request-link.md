# ServiceNow: let the CSM API set a problem's change request

**For:** whoever maintains the `x_wso2_customer_0.ProblemUtils` script include.

## Why

The CSM portal can now link a problem to a change request: `PATCH /problems/{id}` with `changeRequestId`. In dual-write, entity-service writes Postgres, then mirrors the same request to ServiceNow's CSM API.

`ProblemUtils.updateProblem` doesn't know the field yet. It writes `assigned_to`, `assignment_group`, `cause_notes`, `fix_notes`, `workaround` and `due_date`, but never `rfc`. So the link is saved in Postgres but not in ServiceNow:

- A request carrying only `changeRequestId` fails with **400 "No supported field update key"**.
- Each failure is recorded in `sn_writeback_failures`, so the gap is visible, not silent.

## The change

Three edits in `ProblemUtils`, all mirroring how `assignmentGroupId` is already handled. `rfc` is the problem's own "Change request" field: it's editable on the form, and ServiceNow's "Create Normal Change" action sets it.

1. **Reference check.** In `_applyProblemFieldUpdate`, add to `refFields`:

   ```js
   { key: 'changeRequestId', table: 'change_request', label: 'changeRequestId' }
   ```

2. **Apply the field.** Next to the other `_applyProblemField` calls, in both `_applyProblemFieldUpdate` and `_applyProblemTransition`, add:

   ```js
   this._applyProblemField(gr, payload, 'changeRequestId', 'rfc');
   ```

   `_applyProblemField` already writes `''` for an empty value. That's how the portal unlinks.

3. **Count it as an update.** Add `'changeRequestId'` to `_PROBLEM_UPDATE_FIELD_KEYS`.

## After applying

Re-drive the recorded failures so ServiceNow catches up with the links already saved in Postgres. They are the `sn_writeback_failures` rows for entity `problem` whose payload carries `changeRequestId`.
