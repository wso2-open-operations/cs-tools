BEGIN;

-- case_watchers is the native watch list: the people notified about a case
-- beyond its creator and assignee.
--
-- ServiceNow modelled this as four separate glide_lists that Ballerina
-- collapsed into one on read, and those upstream lists could name groups as
-- well as users -- which is why domain.WatchListUser carries a nullable User
-- reference and never promises the entry's id is a user's. This table keeps
-- that looseness deliberately: user_id is the canonical reference when the
-- watcher is a known platform user, and NULL when the entry came from a group
-- or an address with no user record behind it. email is therefore the field
-- that is always present, and the one the notification path actually consumes.
CREATE TABLE IF NOT EXISTS case_watchers (
  case_id  UUID        NOT NULL REFERENCES cases(id) ON DELETE CASCADE,
  user_id  TEXT        REFERENCES users(id) ON DELETE SET NULL,
  email    TEXT        NOT NULL,
  added_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

  -- Keyed on email, not user_id: email is the one column always populated, and
  -- it is what makes "watch this case" idempotent for an address that has no
  -- user record. Callers normalise to lower case before writing (see
  -- CaseRepository.ReplaceCaseWatchers) so the key does not admit case-variant
  -- duplicates of one address.
  PRIMARY KEY (case_id, email)
);

-- ON DELETE CASCADE covers removing a case's watchers with the case; this index
-- serves the other direction -- finding or clearing one user's watched cases --
-- which no other index on the table covers. Partial because a NULL user_id
-- entry can never be found by user.
CREATE INDEX IF NOT EXISTS idx_case_watchers_user_id
  ON case_watchers (user_id)
  WHERE user_id IS NOT NULL;

COMMIT;
