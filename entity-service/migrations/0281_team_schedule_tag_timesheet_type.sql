-- Team Schedule tags say for themselves whether they take people off the
-- worklog timesheet, and may carry a description shown on hover.
--
-- Until now the timesheet left out the allocation kinds ENG, ALLO_EXT and
-- ALLO_INT by code, so a tag added from the portal could never do the same
-- (an SRE rotation tag, say), and nobody could tell from the tag which ones
-- did. excludes_from_timesheet makes that a property of the tag: the three
-- kinds that already did it are set here, so the timesheet leaves out exactly
-- the same people as before. Leave kinds never set it (leave shows as leave),
-- and EXCLUDED kinds are left out by their bucket, as before.
--
-- Additive and safe to re-run.
ALTER TABLE team_schedule_absence_kind
    ADD COLUMN IF NOT EXISTS excludes_from_timesheet BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS description VARCHAR(300);

UPDATE team_schedule_absence_kind
   SET excludes_from_timesheet = TRUE
 WHERE code IN ('ENG', 'ALLO_EXT', 'ALLO_INT')
   AND bucket = 'ALLOCATION'
   AND NOT excludes_from_timesheet;
