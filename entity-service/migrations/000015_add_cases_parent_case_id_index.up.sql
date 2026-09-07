BEGIN;

-- Ports ServiceNow's "Prevent Closure Of Parent" business rule (a `before`
-- update rule with setAbortAction) into the native Postgres path. That guard
-- rejects closing a case while any of its child cases is still open; enforcing
-- it means, on every close, counting the case's non-closed children via
--   SELECT COUNT(*) FROM cases WHERE parent_case_id = $1 AND state <> 'closed'
-- (see CaseRepository.CountNonClosedChildCases). parent_case_id has no index
-- today -- the existing indexes on cases cover the parent direction of the
-- self-join (pc.id = c.parent_case_id) but not the child direction (looking up
-- rows BY parent_case_id) -- so without this the guard is a sequential scan on
-- every close.
--
-- A partial index keyed exactly to the guard's predicate: only non-closed
-- children of a parent can ever block a close, so closed rows and rows with no
-- parent are excluded. This keeps the index small (it holds only the open
-- children that matter) and makes the count an index-only lookup.
CREATE INDEX IF NOT EXISTS idx_cases_open_children
  ON cases (parent_case_id)
  WHERE parent_case_id IS NOT NULL AND state <> 'closed';

COMMIT;
