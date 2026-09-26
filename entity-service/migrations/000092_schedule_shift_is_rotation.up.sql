-- Whether a window is a turn on the rota, or simply when a team works.
--
-- The frontend was deciding this by reading the shift's code: anything whose
-- code contained "REGULAR" was standing work, and CRE_AMERICAS was a second
-- hardcoded exception. Both are guesses about a naming convention rather than
-- facts about the window, and neither the compiler nor the database would
-- catch a new standing shift that happened not to say REGULAR -- it would
-- simply be miscounted as a rotation, silently, in every view.
--
-- None of the existing columns answers it. tier, day_scope, is_on_call and
-- is_escalation all describe what the window *is*; none says whether being on
-- it is a turn somebody takes.
--
-- Defaults TRUE because a rotation is the common case: every window added
-- since this feature began has been one, and a standing shift has to say so.
ALTER TABLE schedule_shift
    ADD COLUMN IF NOT EXISTS is_rotation BOOLEAN NOT NULL DEFAULT TRUE;

-- Regular hours are what everybody not holding a rotation works that day.
--
-- Matched by pattern deliberately, and note the distinction from what this
-- column replaces: reading a naming convention ONCE, here, to classify the
-- rows that exist today is safe and reviewable. Reading it at runtime, on
-- every render, was not -- a new standing window that did not happen to say
-- REGULAR was silently miscounted. From here the column is the answer and the
-- convention is never consulted again.
--
-- The pattern matters: CRE_REGULAR_IND is a third regular-hours window, and an
-- exact-code list written from memory missed it.
UPDATE schedule_shift SET is_rotation = FALSE
 WHERE code LIKE '%REGULAR%';

-- Americas night cover is that team's standing shift, not a turn: the whole
-- team is on it every weekday. It is excluded from "who else is on with me"
-- for exactly this reason, which was the hardcoded exception this replaces.
UPDATE schedule_shift SET is_rotation = FALSE
 WHERE code = 'CRE_AMERICAS';

COMMENT ON COLUMN schedule_shift.is_rotation IS
  'TRUE when being on this window is a turn somebody takes; FALSE when it is simply when a team works.';
