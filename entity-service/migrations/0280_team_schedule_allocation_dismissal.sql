-- Days a rota admin or team lead took a customer engagement allocation (from
-- the allocation app, customer_engagement_allocation_resource) off a person's
-- Team Schedule roster.
--
-- The allocation itself is synced in and never edited here: a removal is a
-- row per weekday, so it survives the next sync, can be undone day by day
-- (Restore deletes the rows), and the worklog report no longer treats those
-- days as allocated. Removing the synced allocation row removes its days too.
--
-- Additive and safe to re-run.
CREATE TABLE IF NOT EXISTS team_schedule_allocation_dismissal (
    id                     UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    allocation_resource_id UUID         NOT NULL
        REFERENCES customer_engagement_allocation_resource (id) ON DELETE CASCADE,
    user_id                UUID         NOT NULL,
    team_key               VARCHAR(100) NOT NULL,
    rota_date              DATE         NOT NULL,
    created_on             TIMESTAMPTZ  NOT NULL DEFAULT now(),
    created_by             VARCHAR(255) NOT NULL,
    CONSTRAINT team_schedule_allocation_dismissal_day_unique
        UNIQUE (allocation_resource_id, rota_date)
);

CREATE INDEX IF NOT EXISTS idx_team_schedule_allocation_dismissal_user_day
    ON team_schedule_allocation_dismissal (user_id, rota_date);
